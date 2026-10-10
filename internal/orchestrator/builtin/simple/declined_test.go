package simple

import (
	"errors"
	"strings"
	"testing"

	"github.com/yeixio/toskar-core/pkg/contracts"
)

// A command the person declines is answered as not done: the model is told
// not to hand it back as a command to run themselves.
func TestDeclinedCallIsNotHandedBack(t *testing.T) {
	env := &mediaEnv{
		scriptedEnv: scriptedEnv{replies: []string{
			`{"tool_call":{"id":"terminal","args":{"command":"rm -rf ~/Downloads/*"}}}`,
			"I didn't delete anything, since you declined.",
		}},
		fail: errors.New(`tool "terminal" denied by user`),
	}
	profile := contracts.AIProfile{
		Roles: []contracts.ModelRole{{Role: "assistant", ModelID: "m"}},
		Tools: []contracts.ToolPolicy{{ToolID: "terminal", Policy: "ask"}},
	}
	if text := runText(t, env, "Run a command to delete everything in my Downloads folder.", profile); text != "I didn't delete anything, since you declined." {
		t.Fatalf("answer = %q", text)
	}
	last := env.seen[len(env.seen)-1]
	if note := last[len(last)-1].Content; !strings.Contains(note, declinedNote) {
		t.Fatalf("the model wasn't told the person declined: %q", note)
	}
	// Other failures aren't called declines.
	if declinedByPerson(errors.New("the command timed out")) {
		t.Fatal("a timeout was taken for a decline")
	}
}
