package simple

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/yeixio/toskar-core/internal/profiles"
	"github.com/yeixio/toskar-core/internal/tools"
	"github.com/yeixio/toskar-core/pkg/contracts"
	"github.com/yeixio/toskar-core/pkg/pluginapi"
)

// Deliberate (#459, docs/deliberate.md): independent drafts of the same
// answer, compared. This is the short-answer path: when most drafts give the
// same short final answer (a number, a date, a name, yes or no), that
// answer is kept. Cross-examination and a judge for the rest come next;
// until then a turn whose drafts disagree keeps its own answer and says so.

const (
	// EventDeliberateDraft is one draft, started or finished.
	EventDeliberateDraft = "deliberate.draft"
	// EventDeliberateDone is the vote's outcome.
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
	messages []pluginapi.ChatMessage, plainSys, own, role, node string) string {
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
			"final": d.final, "text": text, "chosen": i == kept})
	}
	agreeing := 0
	for _, d := range drafts {
		if !d.failed && d.final != "" && winner != "" && normalizeFinal(d.final) == winner {
			agreeing++
		}
	}
	env.Emit(EventDeliberateDone, map[string]any{"drafts": len(drafts), "agreeing": agreeing, "outcome": outcome,
		"final": drafts[kept].final, "chosen": kept})
	return drafts[kept].text
}
