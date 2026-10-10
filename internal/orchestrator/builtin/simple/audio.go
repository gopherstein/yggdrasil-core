package simple

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/yeixio/toskar-core/pkg/contracts"
	"github.com/yeixio/toskar-core/pkg/pluginapi"
)

// A recording attached to the message is transcribed before the model
// answers, as pictures are made first (makeMediaFirst): a small model
// asked "what does this say?" can write the call so it isn't taken, and
// answer with nothing (#510).

// audioNoteRe finds a recording attached to this message in the reference
// material, and the call its note asks for.
var audioNoteRe = regexp.MustCompile(`Audio file attached to this message by the user: [^\n]+?\. To know what it says, (call speech\.transcribe with \{"file": "([^"]+)"\})\.`)

// maxTranscribedFirst caps the recordings transcribed before answering;
// the model can still ask for more.
const maxTranscribedFirst = 3

// transcribeFirst transcribes the recordings attached to this message,
// when the profile allows it, and puts each transcript in the reference in
// place of the note to call the tool. A recording that fails keeps its
// note.
func transcribeFirst(ctx context.Context, env pluginapi.ExecutionEnvironment, profile contracts.AIProfile, reference string) string {
	if !toolEnabled(profile, "speech.transcribe") {
		return reference
	}
	done := map[string]bool{}
	for _, m := range audioNoteRe.FindAllStringSubmatch(reference, -1) {
		file := m[2]
		if done[file] || len(done) >= maxTranscribedFirst || ctx.Err() != nil {
			continue
		}
		done[file] = true
		result, err := env.ExecuteTool(ctx, "speech.transcribe", map[string]any{"file": file})
		if err != nil {
			continue
		}
		text, _ := result["text"].(string)
		if strings.TrimSpace(text) == "" {
			text = "(no speech was heard)"
		}
		reference = strings.Replace(reference, m[1], "read its transcript below", 1)
		reference = joinReference(reference, fmt.Sprintf("Transcript of %s. %s\n%s", file, untrustedNote, strings.TrimSpace(text)))
	}
	return reference
}
