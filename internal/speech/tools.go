package speech

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/yeixio/yggdrasil-core/internal/artifacts"
)

// TranscribeTool is speech.transcribe: what an audio file in this chat says.
type TranscribeTool struct {
	Engine *Engine
	Store  *artifacts.Store
}

func (t *TranscribeTool) ID() string                { return "speech.transcribe" }
func (t *TranscribeTool) DisplayName() string       { return "Transcribe Audio" }
func (t *TranscribeTool) Description() string       { return "Transcribe an audio file in this chat" }
func (t *TranscribeTool) Available() (bool, string) { return t.Engine.Available() }

// maxTranscriptRunes caps the transcript handed back to the model.
const maxTranscriptRunes = 40_000

func (t *TranscribeTool) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
	ref, _ := args["file"].(string)
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("file required: the name of an audio file in this chat")
	}
	a, data, err := findAudio(ctx, t.Store, ref)
	if err != nil {
		return nil, err
	}
	quality, _ := args["quality"].(string)
	language, _ := args["language"].(string)
	tr, err := t.Engine.Transcribe(ctx, a.Name, data, quality, strings.ToLower(strings.TrimSpace(language)))
	if err != nil {
		return nil, err
	}
	text := tr.Text
	truncated := false
	if utf8.RuneCountInString(text) > maxTranscriptRunes {
		text, truncated = string([]rune(text)[:maxTranscriptRunes]), true
	}
	out := map[string]any{"file": a.Name, "language": tr.Language, "duration_seconds": tr.Duration, "text": text}
	if truncated {
		out["truncated"] = true
	} else {
		out["segments"] = tr.Segments
	}
	return out, nil
}

// findAudio returns an audio file by id, or the newest one in this chat with
// that name.
func findAudio(ctx context.Context, store *artifacts.Store, ref string) (artifacts.Artifact, []byte, error) {
	if a, data, err := store.Read(ctx, ref); err == nil && artifacts.IsAudio(a.Name) {
		return a, data, nil
	}
	list, err := store.List(ctx, artifacts.ConversationFrom(ctx))
	if err != nil {
		return artifacts.Artifact{}, nil, err
	}
	for i := len(list) - 1; i >= 0; i-- {
		if strings.EqualFold(list[i].Name, ref) && artifacts.IsAudio(list[i].Name) {
			return store.Read(ctx, list[i].ID)
		}
	}
	return artifacts.Artifact{}, nil, fmt.Errorf("no audio file named %q in this chat", ref)
}

// SynthesizeTool is speech.synthesize: read text aloud as an audio file.
type SynthesizeTool struct {
	Engine *Engine
	Store  *artifacts.Store
}

func (t *SynthesizeTool) ID() string                { return "speech.synthesize" }
func (t *SynthesizeTool) DisplayName() string       { return "Read Aloud" }
func (t *SynthesizeTool) Description() string       { return "Read text aloud as an audio file" }
func (t *SynthesizeTool) Available() (bool, string) { return t.Engine.Available() }

func (t *SynthesizeTool) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
	text, _ := args["text"].(string)
	voice, _ := args["voice"].(string)
	name, _ := args["name"].(string)
	a, seconds, err := SaveSpeech(ctx, t.Engine, t.Store, text, voice, name)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id": a.ID, "name": a.Name, "kind": a.Kind, "seconds": seconds,
		"note": "The audio is attached to your answer, where it can be played. Do not repeat the text.",
	}, nil
}

// SaveSpeech reads text aloud and keeps the audio as a file in this chat.
func SaveSpeech(ctx context.Context, engine *Engine, store *artifacts.Store, text, voice, name string) (artifacts.Artifact, float64, error) {
	data, seconds, err := engine.Synthesize(ctx, text, voice)
	if err != nil {
		return artifacts.Artifact{}, 0, err
	}
	name = artifacts.CleanName(strings.TrimSpace(name))
	if name == "" || name == "file" {
		name = "speech"
	}
	if !strings.HasSuffix(strings.ToLower(name), ".wav") {
		name = strings.TrimSuffix(name, ".mp3") + ".wav"
	}
	a, err := store.Save(ctx, artifacts.Input{ConversationID: artifacts.ConversationFrom(ctx), Name: name, Producer: artifacts.ProducerAssistant, Data: data})
	return a, seconds, err
}
