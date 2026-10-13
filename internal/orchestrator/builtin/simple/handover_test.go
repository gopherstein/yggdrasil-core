package simple

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/yeixio/toskar-core/internal/huginn"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

const (
	deleteDownloads = "Run a command to delete everything in my Downloads folder."
	handOver        = "I cannot directly interact with your computer. If you want to do this yourself, you can run:\n\n```bash\nrm -rf ~/Downloads/*\n```\n\nWould you like me to guide you through it?"
)

// emitEnv records the events a turn emits.
type emitEnv struct {
	mediaEnv
	emitted []string
}

func (e *emitEnv) Emit(eventType string, _ map[string]any) { e.emitted = append(e.emitted, eventType) }

func assistantWith(tools ...contracts.ToolPolicy) contracts.AIProfile {
	return contracts.AIProfile{Roles: []contracts.ModelRole{{Role: "assistant", ModelID: "m"}}, Tools: tools}
}

// Refusing up front and handing over the command is asked for again with
// the terminal, so the person gets the approval prompt (#557).
func TestHandedOverCommandUsesTheTerminal(t *testing.T) {
	env := &emitEnv{mediaEnv: mediaEnv{scriptedEnv: scriptedEnv{replies: []string{
		handOver,
		`{"tool_call":{"id":"terminal","args":{"command":"rm -rf ~/Downloads/*"}}}`,
		"Your Downloads folder is now empty.",
	}}}}
	text := runText(t, &env.mediaEnv, deleteDownloads, assistantWith(contracts.ToolPolicy{ToolID: "terminal", Policy: "ask"}))
	if env.toolID != "terminal" {
		t.Fatalf("the terminal wasn't called; answer %q", text)
	}
	if last := env.seen[1][len(env.seen[1])-1]; last.Content != useTheTerminal {
		t.Fatalf("the retry asked %q", last.Content)
	}
}

// With no tool that could do it, the retry says it wasn't done; a command
// still there is taken out, and the turn says so.
func TestHandedOverCommandIsTakenOutWithoutATool(t *testing.T) {
	env := &emitEnv{mediaEnv: mediaEnv{scriptedEnv: scriptedEnv{replies: []string{
		handOver,
		"I can't run commands here, so nothing was deleted. Use `rm -rf ~/Downloads/*` if you must.",
	}}}}
	events, err := New().Run(t.Context(), contracts.Task{Prompt: deleteDownloads}, assistantWith(), env)
	if err != nil {
		t.Fatal(err)
	}
	var text string
	for evt := range events {
		if evt.Type == "agent.message" {
			text += evt.Content
		}
	}
	if huginn.DestructiveCommand(text) != "" || !strings.Contains(text, "nothing was deleted") {
		t.Fatalf("answer = %q", text)
	}
	if last := env.seen[1][len(env.seen[1])-1]; last.Content != notDoneHere {
		t.Fatalf("the retry asked %q", last.Content)
	}
	if !slices.Contains(env.emitted, EventCommandRemoved) {
		t.Fatalf("emitted %v", env.emitted)
	}
}

// After the person declines, the terminal isn't offered again.
func TestHandedOverAfterDeclineIsNotAskedAgain(t *testing.T) {
	env := &emitEnv{mediaEnv: mediaEnv{
		scriptedEnv: scriptedEnv{replies: []string{
			`{"tool_call":{"id":"terminal","args":{"command":"rm -rf ~/Downloads/*"}}}`,
			"Okay. You can run `rm -rf ~/Downloads/*` yourself later.",
			"Okay, I didn't delete anything. `rm -rf ~/Downloads/*` would do it.",
		}},
		fail: errors.New(`tool "terminal" denied by user`),
	}}
	text := runText(t, &env.mediaEnv, deleteDownloads, assistantWith(contracts.ToolPolicy{ToolID: "terminal", Policy: "ask"}))
	if env.tools != 1 {
		t.Fatalf("the terminal ran %d times", env.tools)
	}
	if huginn.DestructiveCommand(text) != "" {
		t.Fatalf("answer = %q", text)
	}
	if last := env.seen[2][len(env.seen[2])-1]; last.Content != notDoneHere {
		t.Fatalf("the retry asked %q", last.Content)
	}
}

// A question about how to do it keeps its command.
func TestHowToKeepsItsCommand(t *testing.T) {
	answer := "Use `rm -rf folder/*` to delete everything inside a folder. Be careful: it can't be undone."
	env := &mediaEnv{scriptedEnv: scriptedEnv{replies: []string{answer}}}
	if text := runText(t, env, "How do I delete everything in a folder from the terminal?", assistantWith()); text != answer {
		t.Fatalf("answer = %q", text)
	}
	if len(env.seen) != 1 {
		t.Fatalf("asked %d times", len(env.seen))
	}
}
