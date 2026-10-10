package quality

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/yeixio/toskar-core/internal/config"
)

// The reasoning set (#459, docs/deliberate.md): questions with one right
// answer, by kind, to measure how often a model gets them right and what it
// costs, alone and, later, deliberating. A real model's numbers are
// measured, not held to a pass rate; the stub must score every case, which
// checks the set and its scoring.

// reasoningCase is one question in reasoning.json.
type reasoningCase struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Message string `json:"message"`
	// Answer must match the final answer; Wrong is the tempting mistake.
	Answer string `json:"answer"`
	Wrong  string `json:"wrong"`
	// Solution is what the stub answers.
	Solution string `json:"solution"`
}

// reasoningAsk is added to every question, the same for every mode, so the
// final answer can be found.
const reasoningAsk = "\n\nGive your final answer on the last line, as: Final answer: …"

var finalLine = regexp.MustCompile(`(?i)final answer\s*[:：]?`)

// finalAnswer is the part of an answer that counts: after its last "Final
// answer", or its last line when it has none.
func finalAnswer(answer string) string {
	if locs := finalLine.FindAllStringIndex(answer, -1); len(locs) > 0 {
		return strings.TrimSpace(strings.Trim(answer[locs[len(locs)-1][1]:], " *_:"))
	}
	lines := strings.Split(strings.TrimSpace(answer), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// scoreReasoning reports whether a final answer is right: it matches the
// case's answer, and when it also matches the tempting mistake, the right
// one comes first ("Cara (Ben has the fish)" is right; "$0.10, I mean
// $0.05" is not).
func scoreReasoning(c reasoningCase, final string) (bool, error) {
	right, err := regexp.Compile("(?i)" + c.Answer)
	if err != nil {
		return false, err
	}
	at := right.FindStringIndex(final)
	if at == nil {
		return false, nil
	}
	if c.Wrong == "" {
		return true, nil
	}
	wrong, err := regexp.Compile("(?i)" + c.Wrong)
	if err != nil {
		return false, err
	}
	if w := wrong.FindStringIndex(final); w != nil && w[0] < at[0] {
		return false, nil
	}
	return true, nil
}

// reasoningResult is one case's outcome, for the reports.
type reasoningResult struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	Correct   bool    `json:"correct"`
	Final     string  `json:"final"`
	LatencyMs float64 `json:"latency_ms"`
	Tokens    int     `json:"tokens"`
	Calls     int     `json:"calls"`
	Error     string  `json:"error,omitempty"`
}

func loadReasoning(t *testing.T) []reasoningCase {
	t.Helper()
	raw, err := os.ReadFile("reasoning.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Cases []reasoningCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Cases) == 0 {
		t.Fatal("reasoning.json has no cases")
	}
	return file.Cases
}

// TestReasoningCases checks the set itself: unique IDs, a known kind, and
// a solution its own answer scores right.
func TestReasoningCases(t *testing.T) {
	kinds := []string{"math", "logic", "multihop", "trick"}
	seen := map[string]bool{}
	for _, c := range loadReasoning(t) {
		if seen[c.ID] {
			t.Errorf("%s: duplicate id", c.ID)
		}
		seen[c.ID] = true
		if !slices.Contains(kinds, c.Kind) {
			t.Errorf("%s: kind %q isn't one of %v", c.ID, c.Kind, kinds)
		}
		if ok, err := scoreReasoning(c, c.Solution); err != nil || !ok {
			t.Errorf("%s: its solution %q doesn't score right: %v", c.ID, c.Solution, err)
		}
	}
	// The scoring itself.
	pets := reasoningCase{Answer: `\bCara\b`, Wrong: `\b(Ben|Alice)\b`}
	ball := reasoningCase{Answer: `\$?0?\.05\b`, Wrong: `\$?0?\.10\b`}
	for final, want := range map[string]bool{
		"Cara (Ben has the fish)": true,
		"Ben":                     false,
		"Alice, then Cara":        false,
		"":                        false,
		"$0.05, not $0.10":        true,
		"$0.10, I mean $0.05":     false,
		"The ball costs $0.05.":   true,
	} {
		c := pets
		if strings.Contains(final, "$") {
			c = ball
		}
		if got, _ := scoreReasoning(c, final); got != want {
			t.Errorf("score(%q) = %v", final, got)
		}
	}
	for answer, want := range map[string]string{
		"Some working.\nFinal answer: 24":         "24",
		"**Final answer:** Cara":                  "Cara",
		"final answer is 7.5. Final Answer: 7.5°": "7.5°",
		"It is 12.\n\nTwelve.":                    "Twelve.",
	} {
		if got := finalAnswer(answer); got != want {
			t.Errorf("finalAnswer(%q) = %q, want %q", answer, got, want)
		}
	}
}

// TestReasoningQuality runs the reasoning set and reports, by kind, how
// many a model got right, how long it took, and how many tokens it used.
// TOSKAR_QUALITY_DELIBERATE names the mode being measured (single for
// now); TOSKAR_QUALITY_REASONING_REPORT writes a Markdown report and
// TOSKAR_QUALITY_REASONING_JSON the results, for the matrix.
func TestReasoningQuality(t *testing.T) {
	cases := loadReasoning(t)
	d, build := qualityDriver(t)
	mode := config.Env("QUALITY_DELIBERATE")
	if mode == "" {
		mode = "single"
	}
	var results []reasoningResult
	stubbedKinds := map[string]bool{}
	for _, rc := range cases {
		// The stub checks the plumbing and the scoring with one case of each
		// kind; TestReasoningCases checks every case's solution, and the
		// whole set would add a minute to every CI run.
		if d.Name() == "stub" {
			if stubbedKinds[rc.Kind] {
				continue
			}
			stubbedKinds[rc.Kind] = true
		}
		c := Case{
			ID: rc.ID, What: rc.Kind, Message: rc.Message + reasoningAsk,
			// Reasoning, not looking it up.
			Setup: Setup{Tools: map[string]string{"internet.search": "deny", "internet.open": "deny"}},
		}
		if d.Name() == "stub" {
			// The same reply for any correction pass the checks ask for.
			reply := "Working it through step by step.\nFinal answer: " + rc.Solution
			c.Stub = []string{reply, reply, reply}
		}
		if stopped(t, d, c) {
			break
		}
		res := reasoningResult{ID: rc.ID, Kind: rc.Kind, Error: "the case stopped before it could be checked; see its log"}
		t.Run(rc.ID, func(t *testing.T) {
			r := d.Run(t, c)
			res.Error = ""
			res.Final = finalAnswer(r.Answer)
			ok, err := scoreReasoning(rc, res.Final)
			if err != nil {
				t.Fatal(err)
			}
			res.Correct = ok
			if r.Run != nil {
				res.LatencyMs = r.Run.LatencyMs
				for _, m := range r.Run.Models {
					res.Tokens += m.PromptTokens + m.CompletionTokens
					res.Calls += m.Calls
				}
			}
			if !ok {
				msg := fmt.Sprintf("final answer %q isn't right", res.Final)
				if d.Name() == "stub" {
					t.Error(msg)
				} else {
					t.Log("WRONG: " + msg)
				}
			}
		})
		results = append(results, res)
	}
	sum := summarize(results)
	t.Logf("reasoning with the %s model, %s: %s", d.Name(), mode, sum.line())
	if path := config.Env("QUALITY_REASONING_REPORT"); path != "" {
		writeReport(t, d, build, path, reasoningReport(d.Name(), mode, results, sum))
	}
	if path := config.Env("QUALITY_REASONING_JSON"); path != "" {
		out, _ := json.MarshalIndent(map[string]any{"driver": d.Name(), "build": build, "mode": mode, "summary": map[string]any{"all": sum.all, "by_kind": sum.byKind}, "cases": results}, "", "  ")
		if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
			t.Errorf("results: %v", err)
		}
	}
}

// kindSummary is how a kind of question went.
type kindSummary struct {
	Kind      string  `json:"kind"`
	Cases     int     `json:"cases"`
	Correct   int     `json:"correct"`
	MeanMs    float64 `json:"mean_ms"`
	MeanToken float64 `json:"mean_tokens"`
}

type reasoningSummary struct {
	all    kindSummary
	byKind []kindSummary
}

func summarize(results []reasoningResult) reasoningSummary {
	all := kindSummary{Kind: "all"}
	var byKind []kindSummary
	index := map[string]int{}
	add := func(s *kindSummary, r reasoningResult) {
		s.Cases++
		if r.Correct {
			s.Correct++
		}
		s.MeanMs += r.LatencyMs
		s.MeanToken += float64(r.Tokens)
	}
	for _, r := range results {
		i, ok := index[r.Kind]
		if !ok {
			i = len(byKind)
			index[r.Kind] = i
			byKind = append(byKind, kindSummary{Kind: r.Kind})
		}
		add(&byKind[i], r)
		add(&all, r)
	}
	mean := func(s *kindSummary) {
		if s.Cases > 0 {
			s.MeanMs /= float64(s.Cases)
			s.MeanToken /= float64(s.Cases)
		}
	}
	mean(&all)
	for i := range byKind {
		mean(&byKind[i])
	}
	return reasoningSummary{all: all, byKind: byKind}
}

func (s reasoningSummary) line() string {
	return fmt.Sprintf("%d of %d right (%.0f%%), %.1f s and %.0f tokens a question on average",
		s.all.Correct, s.all.Cases, share(s.all.Correct, s.all.Cases)*100, s.all.MeanMs/1000, s.all.MeanToken)
}

// reasoningReport is the Markdown report: by kind, then every case.
func reasoningReport(driver, mode string, results []reasoningResult, sum reasoningSummary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Reasoning: core, %s model, %s\n\n", driver, mode)
	fmt.Fprintf(&b, "%s.\n\n", sum.line())
	b.WriteString("| Kind | Right | Share | Seconds | Tokens |\n| --- | --- | --- | --- | --- |\n")
	for _, k := range append(sum.byKind, sum.all) {
		fmt.Fprintf(&b, "| %s | %d of %d | %.0f%% | %.1f | %.0f |\n", k.Kind, k.Correct, k.Cases, share(k.Correct, k.Cases)*100, k.MeanMs/1000, k.MeanToken)
	}
	b.WriteString("\n| Case | Kind | Result | Final answer | Seconds | Tokens |\n| --- | --- | --- | --- | --- | --- |\n")
	for _, r := range results {
		mark := "right"
		switch {
		case r.Error != "":
			mark = "stopped"
		case !r.Correct:
			mark = "**wrong**"
		}
		final := strings.ReplaceAll(r.Final, "|", "\\|")
		if len(final) > 80 {
			final = final[:80] + "…"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %.1f | %d |\n", r.ID, r.Kind, mark, final, r.LatencyMs/1000, r.Tokens)
	}
	return b.String()
}
