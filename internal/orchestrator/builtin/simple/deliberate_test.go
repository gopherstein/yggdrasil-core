package simple

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/yeixio/toskar-core/pkg/contracts"
	"github.com/yeixio/toskar-core/pkg/pluginapi"
)

// roleEnv answers by role, and records each role's options, computer, and
// prompt, and the events.
type roleEnv struct {
	mu       sync.Mutex
	replies  map[string]string
	opts     map[string]pluginapi.GenerateOptions
	systems  map[string]string
	calls    map[string]int
	events   []string
	payloads []map[string]any
}

func newRoleEnv(replies map[string]string) *roleEnv {
	return &roleEnv{replies: replies, opts: map[string]pluginapi.GenerateOptions{}, systems: map[string]string{}, calls: map[string]int{}}
}

func (e *roleEnv) Generate(ctx context.Context, role string, messages []pluginapi.ChatMessage) (<-chan pluginapi.ChatChunk, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.calls[role]++
	e.opts[role] = pluginapi.GenerateOptionsFrom(ctx)
	if len(messages) > 0 {
		e.systems[role] = messages[0].Content
	}
	content, ok := e.replies[role]
	if !ok {
		content = "done"
	}
	ch := make(chan pluginapi.ChatChunk, 1)
	ch <- pluginapi.ChatChunk{Content: content, Done: true}
	close(ch)
	return ch, nil
}

func (e *roleEnv) ExecuteTool(ctx context.Context, toolID string, args map[string]any) (map[string]any, error) {
	return map[string]any{"ok": true}, nil
}

func (e *roleEnv) Emit(eventType string, payload map[string]any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, eventType)
	e.payloads = append(e.payloads, payload)
}

// NodeForRole puts each drafter on a computer of its own.
func (e *roleEnv) NodeForRole(role string) (string, error) {
	if strings.HasPrefix(role, "drafter:") {
		return "pc-" + strings.TrimPrefix(role, "drafter:"), nil
	}
	return "local", nil
}

func (e *roleEnv) last(event string) map[string]any {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := len(e.events) - 1; i >= 0; i-- {
		if e.events[i] == event {
			return e.payloads[i]
		}
	}
	return nil
}

func (e *roleEnv) count(event string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, ev := range e.events {
		if ev == event {
			n++
		}
	}
	return n
}

const muffins = "A bakery sells muffins for $3 each or 4 for $10. Maya buys exactly 10 muffins as cheaply as possible. How much does she pay?"

func runDeliberate(t *testing.T, env *roleEnv, prompt string, orch contracts.OrchestrationPolicy) string {
	t.Helper()
	events, err := New().Run(context.Background(), contracts.Task{Prompt: prompt}, contracts.AIProfile{
		Roles:         []contracts.ModelRole{{Role: "assistant", ModelID: "m"}},
		Orchestration: orch,
	}, env)
	if err != nil {
		t.Fatal(err)
	}
	var text string
	for evt := range events {
		if evt.Type == "agent.message" {
			text += evt.Content
		}
	}
	return text
}

// always deliberates, with the answer checks off: they'd ask this fake
// environment's assistant again, which answers with its first draft.
var always = contracts.OrchestrationPolicy{Deliberate: DeliberateAlways, Verification: "off"}

func TestDeliberateVote(t *testing.T) {
	for name, c := range map[string]struct {
		own, second, third string
		outcome, final     string
		kept               string
		agreeing           int
	}{
		"agreed": {
			own: "Two sets of 4 and 2 singles.\nFinal answer: $26", second: "Final answer: 26 dollars", third: "**Final answer:** $26.00",
			outcome: OutcomeAgreed, final: "$26", kept: "Two sets of 4", agreeing: 3,
		},
		"majority, the turn's own kept": {
			own: "Final answer: $26", second: "Final answer: $30", third: "It comes to\nFinal answer: 26",
			outcome: OutcomeMajority, final: "$26", kept: "Final answer: $26", agreeing: 2,
		},
		"the turn's own outvoted": {
			own: "Final answer: $30", second: "Mixing sets and singles.\nFinal answer: $26", third: "Final answer: 26",
			outcome: OutcomeMajority, final: "$26", kept: "Mixing sets and singles", agreeing: 2,
		},
		"disagreed": {
			own: "Final answer: $26", second: "Final answer: $30", third: "Final answer: $28",
			outcome: OutcomeDisagreed, final: "$26", kept: "Final answer: $26",
		},
		"long answers": {
			own: "Here is a long explanation of muffin pricing with no single answer line.", second: "Another long take.", third: "A third.",
			outcome: OutcomeLong, kept: "long explanation of muffin pricing",
		},
	} {
		t.Run(name, func(t *testing.T) {
			env := newRoleEnv(map[string]string{"assistant": c.own, "drafter:2": c.second, "drafter:3": c.third})
			text := runDeliberate(t, env, muffins, always)
			if !strings.Contains(text, c.kept) {
				t.Fatalf("kept %q, want the draft with %q", text, c.kept)
			}
			done := env.last(EventDeliberateDone)
			if done == nil || done["outcome"] != c.outcome || done["agreeing"] != c.agreeing || done["drafts"] != 3 {
				t.Fatalf("done: %+v", done)
			}
			if done["final"] != c.final {
				t.Fatalf("final: %v want %q", done["final"], c.final)
			}
			// Running and finished, for each of the two extra drafts, and
			// finished for the turn's own.
			if n := env.count(EventDeliberateDraft); n != 5 {
				t.Fatalf("draft events: %d", n)
			}
		})
	}
}

// The extra drafts sample at their own temperatures, with a token cap, on
// computers of their own, from the same question without tools; the turn's
// own answer keeps the model's defaults, and every draft is asked for a
// final answer.
func TestDeliberateDrafts(t *testing.T) {
	env := newRoleEnv(map[string]string{"assistant": "Final answer: $26", "drafter:2": "Final answer: $26", "drafter:3": "Final answer: $26"})
	runDeliberate(t, env, muffins, always)
	if env.calls["drafter:2"] != 1 || env.calls["drafter:3"] != 1 {
		t.Fatalf("calls: %v", env.calls)
	}
	if o := env.opts["assistant"]; o != (pluginapi.GenerateOptions{}) {
		t.Fatalf("the turn's own answer changed options: %+v", o)
	}
	if o := env.opts["drafter:2"]; o.Temperature != 0.6 || o.MaxTokens != draftTokens {
		t.Fatalf("drafter:2: %+v", o)
	}
	if o := env.opts["drafter:3"]; o.Temperature != 0.9 || o.MaxTokens != draftTokens {
		t.Fatalf("drafter:3: %+v", o)
	}
	for role, sys := range env.systems {
		if !strings.Contains(sys, draftGuidance) {
			t.Errorf("%s wasn't asked for a final answer", role)
		}
		if strings.HasPrefix(role, "drafter:") && strings.Contains(sys, "tool_call") {
			t.Errorf("%s was offered tools", role)
		}
	}
	var nodes []string
	for _, p := range env.payloads {
		if node, _ := p["node_id"].(string); node != "" && p["status"] == "done" {
			nodes = append(nodes, node)
		}
	}
	if strings.Join(nodes, ",") != "local,pc-2,pc-3" {
		t.Fatalf("computers: %v", nodes)
	}
}

// Off, auto (until measured), small talk, and the Team strategy aren't
// deliberated, and the prompt doesn't ask for a final answer.
func TestDeliberateOnlyWhenAsked(t *testing.T) {
	for name, c := range map[string]struct {
		prompt string
		orch   contracts.OrchestrationPolicy
	}{
		"off":        {muffins, contracts.OrchestrationPolicy{}},
		"never":      {muffins, contracts.OrchestrationPolicy{Deliberate: DeliberateNever}},
		"auto":       {muffins, contracts.OrchestrationPolicy{Deliberate: DeliberateAuto}},
		"small talk": {"hi, how are you?", always},
		"team":       {muffins, contracts.OrchestrationPolicy{Deliberate: DeliberateAlways, Strategy: "team"}},
	} {
		t.Run(name, func(t *testing.T) {
			env := newRoleEnv(map[string]string{"assistant": "Final answer: $26"})
			runDeliberate(t, env, c.prompt, c.orch)
			if env.calls["drafter:2"] != 0 || env.count(EventDeliberateDone) != 0 {
				t.Fatalf("deliberated: %v", env.calls)
			}
			if strings.Contains(env.systems["assistant"], draftGuidance) {
				t.Fatal("asked for a final answer without deliberating")
			}
		})
	}
}

func TestDraftFinalAndNormalize(t *testing.T) {
	for text, want := range map[string]string{
		"work\nFinal answer: $26":                      "$26",
		"**Final answer:** Cara":                       "Cara",
		"Final answer: 7.5°\nsome trailing note":       "7.5°",
		"no final line":                                "",
		"Final answer: " + strings.Repeat("word ", 12): "",
		"final answer: one. Final Answer: two":         "two",
	} {
		if got := draftFinal(text); got != want {
			t.Errorf("draftFinal(%q) = %q, want %q", text, got, want)
		}
	}
	for a, b := range map[string]string{
		"$26":              "26 dollars",
		"$26.00":           "26",
		"1,200":            "1200",
		"The Indian Ocean": "indian ocean",
		"Cara.":            "cara",
		"Yes!":             "yes",
	} {
		if normalizeFinal(a) != normalizeFinal(b) {
			t.Errorf("%q and %q don't agree: %q vs %q", a, b, normalizeFinal(a), normalizeFinal(b))
		}
	}
	if normalizeFinal("26") == normalizeFinal("27") || normalizeFinal("yes") == normalizeFinal("no") {
		t.Fatal("different answers agree")
	}
}

// A failed draft doesn't count: two agreeing of the two that answered is
// agreement.
func TestVoteSkipsFailedDrafts(t *testing.T) {
	outcome, winner, kept := vote([]draft{{final: "26"}, {failed: true}, {final: "$26"}})
	if outcome != OutcomeAgreed || winner != "#26" || kept != 0 {
		t.Fatalf("%s %s %d", outcome, winner, kept)
	}
	if outcome, _, _ := vote([]draft{{final: "26"}, {failed: true}, {failed: true}}); outcome != OutcomeLong {
		t.Fatalf("one answer: %s", outcome)
	}
}
