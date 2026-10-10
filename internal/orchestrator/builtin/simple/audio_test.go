package simple

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yeixio/toskar-core/pkg/contracts"
)

// audioEnv has a recording attached, and transcribes it.
type audioEnv struct {
	mediaEnv
	reference string
}

func (e *audioEnv) ReferenceMaterial(context.Context, string) string { return e.reference }

func (e *audioEnv) ExecuteTool(ctx context.Context, toolID string, args map[string]any) (map[string]any, error) {
	if _, err := e.mediaEnv.ExecuteTool(ctx, toolID, args); err != nil {
		return nil, err
	}
	return map[string]any{"file": args["file"], "text": " Please book a tire rotation for Saturday. "}, nil
}

const recordingNote = `Audio file attached to this message by the user: memo.wav. To know what it says, call speech.transcribe with {"file": "memo.wav"}.`

var audioProfile = contracts.AIProfile{
	Roles: []contracts.ModelRole{{Role: "assistant", ModelID: "m"}},
	Tools: []contracts.ToolPolicy{{ToolID: "speech.transcribe", Policy: "allow"}},
}

// A recording attached to the message is transcribed before the model
// answers, and the model reads the transcript, not a note to call the tool.
func TestRecordingIsTranscribedFirst(t *testing.T) {
	env := &audioEnv{mediaEnv: mediaEnv{scriptedEnv: scriptedEnv{replies: []string{"It asks to book a tire rotation for Saturday."}}}, reference: recordingNote}
	events, err := New().Run(context.Background(), contracts.Task{Prompt: "What does this recording say?"}, audioProfile, env)
	if err != nil {
		t.Fatal(err)
	}
	var answer string
	for evt := range events {
		if evt.Type == "agent.message" {
			answer += evt.Content
		}
	}
	if env.toolID != "speech.transcribe" || env.args["file"] != "memo.wav" {
		t.Fatalf("called %s %+v", env.toolID, env.args)
	}
	if answer != "It asks to book a tire rotation for Saturday." {
		t.Fatalf("answer = %q", answer)
	}
	last := env.seen[len(env.seen)-1]
	user := last[len(last)-1].Content
	if !strings.Contains(user, "Transcript of memo.wav.") || !strings.Contains(user, "Please book a tire rotation for Saturday.") || strings.Contains(user, "call speech.transcribe") {
		t.Fatalf("the model was given %q", user)
	}
}

func TestTranscribeFirstOnlyWhenAllowed(t *testing.T) {
	env := &audioEnv{reference: recordingNote}
	if got := transcribeFirst(context.Background(), env, mediaProfile, recordingNote); got != recordingNote || env.tools != 0 {
		t.Fatalf("transcribed without the tool allowed: %q", got)
	}
	// A recording attached earlier in the chat isn't transcribed again.
	earlier := `Audio file attached earlier in this chat by the user: memo.wav. To know what it says, call speech.transcribe with {"file": "memo.wav"}.`
	if got := transcribeFirst(context.Background(), env, audioProfile, earlier); got != earlier || env.tools != 0 {
		t.Fatalf("transcribed an earlier recording: %q", got)
	}
	// A failure keeps the note, so the model can still try.
	env.fail = errors.New("no transcription model")
	if got := transcribeFirst(context.Background(), env, audioProfile, recordingNote); got != recordingNote {
		t.Fatalf("a failed transcription changed the reference: %q", got)
	}
}

// A reply that is only a call that can't run is asked for again in plain
// text, not sent empty.
func TestCallOnlyReplyIsNotSentEmpty(t *testing.T) {
	env := &mediaEnv{scriptedEnv: scriptedEnv{replies: []string{
		`{"tool_call":{"id":"speech.transcribe","args":{"file":"memo.wav"}}}`,
		`{"tool_call":{"id":"speech.transcribe","args":{"file":"memo.wav"}}}`,
		`{"tool_call":{"id":"speech.transcribe","args":{"file":"memo.wav"}}}`,
		"I can't listen to recordings here.",
	}}}
	if text := runText(t, env, "What does this recording say?", contracts.AIProfile{Roles: audioProfile.Roles}); strings.TrimSpace(text) == "" {
		t.Fatal("an empty answer was sent")
	}
	if env.tools != 0 {
		t.Fatalf("ran %d tools that weren't offered", env.tools)
	}
}
