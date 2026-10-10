package simple

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/yeixio/toskar-core/internal/profiles"
	"github.com/yeixio/toskar-core/internal/structured"
	"github.com/yeixio/toskar-core/internal/tools"
	"github.com/yeixio/toskar-core/pkg/contracts"
	"github.com/yeixio/toskar-core/pkg/pluginapi"
)

// Deliberate (#459, docs/deliberate.md): independent drafts of the same
// answer, compared. The short-answer path comes first: when most drafts give
// the same short final answer (a number, a date, a name, yes or no), that
// answer is kept. Otherwise each draft is critiqued by another drafter, and
// a judge writes the answer from the drafts and critiques, saying where
// they still disagree. Anything that fails keeps the turn's own answer.

const (
	// EventDeliberateDraft is one draft, started or finished.
	EventDeliberateDraft = "deliberate.draft"
	// EventDeliberateCritique is one draft's critique by another drafter.
	EventDeliberateCritique = "deliberate.critique"
	// EventDeliberateDone is the outcome.
	EventDeliberateDone = "deliberate.done"
)

// Deliberate's values for orchestration.deliberate.
const (
	DeliberateNever  = "never"
	DeliberateAlways = "always"
	DeliberateAuto   = "auto"
)

// Outcomes of the vote.
const (
	// OutcomeAgreed: every draft gave the same short final answer.
	OutcomeAgreed = "agreed"
	// OutcomeMajority: most did.
	OutcomeMajority = "majority"
	// OutcomeDisagreed: no short final answer had a majority.
	OutcomeDisagreed = "disagreed"
	// OutcomeLong: the answers weren't short, so there was nothing to vote on.
	OutcomeLong = "long"
)

// deliberateDrafts is how many drafts a turn compares, its own answer
// included.
const deliberateDrafts = 3

// draftTokens caps each extra draft's reply.
const draftTokens = 1536

// draftTemperatures are the extra drafts' temperatures: the turn's own
// answer keeps the model's default, and the others sample more freely, so
// one model's drafts can still differ.
var draftTemperatures = []float64{0.6, 0.9}

// draftGuidance asks every draft, the turn's own included, for a final
// answer the drafts can be compared by.
const draftGuidance = "When the question has a short answer, such as a number, a date, a name, or yes or no, end your reply with a line: Final answer: <the answer>."

// draftTextRunes caps how much of a draft is kept for the person to read.
const draftTextRunes = 6000

// Caps for cross-examination and the judge.
const (
	critiqueTokens = 700
	judgeTokens    = 2048
	// promptDraftRunes caps each draft as quoted to a critic or the judge.
	promptDraftRunes = 4000
)

const critiqueInstructions = "You check another assistant's draft answer against independent drafts of the same question. " +
	"Reply with only a JSON object: {\"claims\": [the draft's key claims], \"disagreements\": [where it disagrees with the other drafts, and which is right if you can tell], \"likely_errors\": [mistakes in the draft: wrong steps, figures, or facts]}. " +
	"Keep each item to one sentence. Use empty lists when there's nothing to say."

// critiqueSchema constrains a critique where the runtime can.
var critiqueSchema = []byte(`{"type":"object","properties":{"claims":{"type":"array","items":{"type":"string"}},"disagreements":{"type":"array","items":{"type":"string"}},"likely_errors":{"type":"array","items":{"type":"string"}}},"required":["claims","disagreements","likely_errors"]}`)

const judgeGuidance = "You are the judge. Several independent drafts answered the person's question, and each was checked against the others. " +
	"Write the final answer for the person from them: keep what's right, fix what the checks show is wrong, and don't mention the drafts or the checks. " +
	"Where they disagree and you can't settle it, say so plainly and give both answers with the reason for each, instead of picking one silently."

// deliberating reports whether a profile deliberates. Auto behaves like
// never until the quality run shows where it helps (docs/deliberate.md).
func deliberating(profile contracts.AIProfile) bool {
	return profile.Orchestration.Deliberate == DeliberateAlways
}

var finalLineRe = regexp.MustCompile(`(?i)final answer\s*[:：]`)

// draftFinal is a draft's short final answer: what follows its last "Final
// answer:", when that's short. Empty when it has none.
func draftFinal(text string) string {
	locs := finalLineRe.FindAllStringIndex(text, -1)
	if len(locs) == 0 {
		return ""
	}
	final := text[locs[len(locs)-1][1]:]
	if i := strings.IndexByte(final, '\n'); i >= 0 {
		final = final[:i]
	}
	final = strings.TrimSpace(strings.Trim(final, " *_`"))
	if final == "" || len(strings.Fields(final)) > 8 {
		return ""
	}
	return final
}

var (
	numberRe    = regexp.MustCompile(`-?\d[\d,]*(\.\d+)?`)
	wordsOnlyRe = regexp.MustCompile(`[^\p{L}\p{N}\s]+`)
)

// normalizeFinal makes short final answers comparable: a number by its
// value ("$26.00" and "26" agree), anything else by its words, lowercased,
// without punctuation or a leading article.
func normalizeFinal(final string) string {
	if n := numberRe.FindString(final); n != "" {
		n = strings.ReplaceAll(n, ",", "")
		if strings.Contains(n, ".") {
			n = strings.TrimRight(strings.TrimRight(n, "0"), ".")
		}
		return "#" + n
	}
	words := strings.Fields(strings.ToLower(wordsOnlyRe.ReplaceAllString(final, " ")))
	if len(words) > 1 && (words[0] == "the" || words[0] == "a" || words[0] == "an") {
		words = words[1:]
	}
	return strings.Join(words, " ")
}

// draft is one independent answer.
type draft struct {
	role, node string
	text       string
	final      string
	failed     bool
}

// critique is one draft checked by another drafter.
type critique struct {
	draft         int
	critic        string
	claims        []string
	disagreements []string
	likelyErrors  []string
	failed        bool
}

func clip(s string, runes int) string {
	if utf8.RuneCountInString(s) > runes {
		return string([]rune(s)[:runes]) + "…"
	}
	return s
}

// draftLetter names a draft for the critics and the judge: A, B, C.
func draftLetter(i int) string { return string(rune('A' + i)) }

// parseCritique reads a critic's JSON, loosely: a model the runtime
// couldn't constrain may wrap it in text.
func parseCritique(content string) (claims, disagreements, likely []string, ok bool) {
	found, found2 := structured.Extract(tools.VisibleText(content))
	if !found2 {
		return nil, nil, nil, false
	}
	obj, isObj := found.Value.(map[string]any)
	if !isObj {
		return nil, nil, nil, false
	}
	list := func(key string) []string {
		var out []string
		items, _ := obj[key].([]any)
		for _, it := range items {
			if s, _ := it.(string); strings.TrimSpace(s) != "" {
				out = append(out, clip(strings.TrimSpace(s), 300))
			}
			if len(out) == 6 {
				break
			}
		}
		return out
	}
	return list("claims"), list("disagreements"), list("likely_errors"), true
}

// crossExamine has each draft that answered checked by the next drafter
// that did, at the same time.
func crossExamine(ctx context.Context, env pluginapi.ExecutionEnvironment, prompt string, drafts []draft) []critique {
	var answered []int
	for i, d := range drafts {
		if !d.failed {
			answered = append(answered, i)
		}
	}
	if len(answered) < 2 {
		return nil
	}
	critiques := make([]critique, len(answered))
	var wg sync.WaitGroup
	for k, i := range answered {
		critic := drafts[answered[(k+1)%len(answered)]].role
		critiques[k] = critique{draft: i, critic: critic}
		wg.Add(1)
		go func(k, i int, critic string) {
			defer wg.Done()
			var b strings.Builder
			fmt.Fprintf(&b, "The question:\n%s\n\nThe draft to check (draft %s):\n%s\n", prompt, draftLetter(i), clip(drafts[i].text, promptDraftRunes))
			for _, j := range answered {
				if j != i {
					fmt.Fprintf(&b, "\nIndependent draft %s:\n%s\n", draftLetter(j), clip(drafts[j].text, promptDraftRunes))
				}
			}
			ask := []pluginapi.ChatMessage{{Role: "system", Content: critiqueInstructions}, {Role: "user", Content: b.String()}}
			cctx := structured.WithSchema(pluginapi.WithGenerateOptions(ctx, pluginapi.GenerateOptions{MaxTokens: critiqueTokens}), critiqueSchema)
			text, _, err := generateText(cctx, env, critic, ask)
			c := &critiques[k]
			if err != nil {
				c.failed = true
				return
			}
			var ok bool
			c.claims, c.disagreements, c.likelyErrors, ok = parseCritique(text)
			c.failed = !ok
		}(k, i, critic)
	}
	wg.Wait()
	return critiques
}

// judgeRoleFor is the judge: the profile's judge model, else its reviewer,
// else the turn's own.
func judgeRoleFor(profile contracts.AIProfile, own string) string {
	if profiles.HasRole(profile, profiles.RoleJudge) {
		return profiles.RoleJudge
	}
	return reviewerRole(profile, own)
}

// judge writes the answer from the drafts and their critiques.
func judge(ctx context.Context, env pluginapi.ExecutionEnvironment, role, prompt, plainSys string, drafts []draft, critiques []critique) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "The person's question:\n%s\n", prompt)
	for i, d := range drafts {
		if !d.failed {
			fmt.Fprintf(&b, "\nDraft %s:\n%s\n", draftLetter(i), clip(d.text, promptDraftRunes))
		}
	}
	for _, c := range critiques {
		if c.failed || len(c.disagreements)+len(c.likelyErrors) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\nChecks of draft %s:\n", draftLetter(c.draft))
		for _, s := range c.disagreements {
			fmt.Fprintf(&b, "- Disagrees: %s\n", s)
		}
		for _, s := range c.likelyErrors {
			fmt.Fprintf(&b, "- Likely error: %s\n", s)
		}
	}
	ask := []pluginapi.ChatMessage{{Role: "system", Content: plainSys + "\n" + judgeGuidance}, {Role: "user", Content: b.String()}}
	text, _, err := generateText(pluginapi.WithGenerateOptions(ctx, pluginapi.GenerateOptions{MaxTokens: judgeTokens}), env, role, ask)
	text = strings.TrimSpace(tools.VisibleText(text))
	if err == nil && text == "" {
		err = fmt.Errorf("the judge wrote nothing")
	}
	return text, err
}

// vote finds the short final answer most drafts gave. It returns the
// outcome, the winning normalized answer, and the index of the draft kept:
// the turn's own when it agrees, else the first that does.
func vote(drafts []draft) (outcome, winner string, kept int) {
	counts := map[string]int{}
	finals, ok := 0, 0
	for _, d := range drafts {
		if d.failed {
			continue
		}
		ok++
		if d.final != "" {
			finals++
			counts[normalizeFinal(d.final)]++
		}
	}
	if finals < 2 {
		return OutcomeLong, "", 0
	}
	best := 0
	for k, n := range counts {
		if n > best || (n == best && k < winner) {
			winner, best = k, n
		}
	}
	if best < 2 || best*2 <= ok {
		return OutcomeDisagreed, "", 0
	}
	for i, d := range drafts {
		if !d.failed && d.final != "" && normalizeFinal(d.final) == winner {
			kept = i
			break
		}
	}
	if best == ok {
		return OutcomeAgreed, winner, kept
	}
	return OutcomeMajority, winner, kept
}

// deliberate writes the extra drafts of a turn's answer at the same time,
// on other computers when there are any, compares their final answers, and
// returns the answer to keep. own is the turn's answer, by role on node;
// messages are what it was written from, and plainSys the system prompt
// without tools: drafts answer, they don't call tools.
func deliberateAnswer(ctx context.Context, env pluginapi.ExecutionEnvironment, ch chan<- pluginapi.OrchestrationEvent,
	profile contracts.AIProfile, prompt string, messages []pluginapi.ChatMessage, plainSys, own, role, node string) string {
	drafts := make([]draft, deliberateDrafts)
	drafts[0] = draft{role: role, node: node, text: own, final: draftFinal(own)}
	// plainSys already carries draftGuidance.
	ask := append([]pluginapi.ChatMessage{{Role: "system", Content: plainSys}}, messages[1:]...)
	for i := 1; i < deliberateDrafts; i++ {
		drafts[i].role = fmt.Sprintf("%s:%d", profiles.RoleDrafter, i+1)
		// Placed one at a time, so each sees where the others went.
		drafts[i].node, _ = env.NodeForRole(drafts[i].role)
	}
	var wg sync.WaitGroup
	for i := 1; i < deliberateDrafts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			d := &drafts[i]
			announceRole(env, d.role)
			env.Emit(EventDeliberateDraft, map[string]any{"index": i, "role": d.role, "node_id": d.node, "status": "running"})
			opts := pluginapi.GenerateOptions{Temperature: draftTemperatures[(i-1)%len(draftTemperatures)], MaxTokens: draftTokens}
			text, metrics, err := generateText(pluginapi.WithGenerateOptions(ctx, opts), env, d.role, ask)
			d.text = strings.TrimSpace(tools.VisibleText(text))
			if err != nil || d.text == "" {
				d.failed = true
				return
			}
			d.final = draftFinal(d.text)
			reportRole(ch, env, d.role, metrics)
		}(i)
	}
	wg.Wait()
	if ctx.Err() != nil {
		// Stopped: the turn's own answer stands.
		return own
	}
	outcome, winner, kept := vote(drafts)
	// Drafts that disagree, or answers too long to compare, are checked
	// against each other and judged.
	var critiques []critique
	judged, judgeRole, judgeNode := "", "", ""
	if outcome == OutcomeDisagreed || outcome == OutcomeLong {
		critiques = crossExamine(ctx, env, prompt, drafts)
		if ctx.Err() == nil && len(critiques) > 0 {
			judgeRole = judgeRoleFor(profile, role)
			judgeNode, _ = env.NodeForRole(judgeRole)
			announceRole(env, judgeRole)
			if text, err := judge(ctx, env, judgeRole, prompt, plainSys, drafts, critiques); err == nil && ctx.Err() == nil {
				judged = text
			}
		}
		if ctx.Err() != nil {
			return own
		}
	}
	for _, c := range critiques {
		env.Emit(EventDeliberateCritique, map[string]any{"draft": c.draft, "critic": c.critic, "failed": c.failed,
			"claims": c.claims, "disagreements": c.disagreements, "likely_errors": c.likelyErrors})
	}
	for i, d := range drafts {
		status := "done"
		if d.failed {
			status = "failed"
		}
		text := d.text
		if utf8.RuneCountInString(text) > draftTextRunes {
			text = string([]rune(text)[:draftTextRunes]) + "…"
		}
		env.Emit(EventDeliberateDraft, map[string]any{"index": i, "role": d.role, "node_id": d.node, "status": status,
			"final": d.final, "text": text, "chosen": judged == "" && i == kept})
	}
	agreeing := 0
	for _, d := range drafts {
		if !d.failed && d.final != "" && winner != "" && normalizeFinal(d.final) == winner {
			agreeing++
		}
	}
	if judged != "" {
		env.Emit(EventDeliberateDone, map[string]any{"drafts": len(drafts), "agreeing": agreeing, "outcome": outcome,
			"final": draftFinal(judged), "judged": true, "judge_role": judgeRole, "judge_node": judgeNode})
		return judged
	}
	env.Emit(EventDeliberateDone, map[string]any{"drafts": len(drafts), "agreeing": agreeing, "outcome": outcome,
		"final": drafts[kept].final, "chosen": kept})
	return drafts[kept].text
}
