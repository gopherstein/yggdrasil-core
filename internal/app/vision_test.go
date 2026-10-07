package app

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/yeixio/toskar-core/internal/artifacts"
	"github.com/yeixio/toskar-core/internal/models"
	"github.com/yeixio/toskar-core/internal/pyenv"
	"github.com/yeixio/toskar-core/internal/speech"
	"github.com/yeixio/toskar-core/pkg/contracts"
	"github.com/yeixio/toskar-core/pkg/pluginapi"
)

// pictureApp is a stub-inference app with a text model and, when see is
// true, a vision model with its projector installed.
func pictureApp(t *testing.T, see bool) (*App, *[]string, *[][]pluginapi.ChatMessage) {
	t.Helper()
	t.Setenv("TOSKAR_STUB_INFERENCE", "1")
	t.Setenv("TOSKAR_DISCOVERY_ENABLED", "false")
	a, err := New(Options{DataDir: t.TempDir(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.DB.Close() })
	ctx := context.Background()
	install := func(e models.CatalogEntry) {
		a.Models.Catalog().Upsert(e)
		st := a.Models.Storage()
		if err := st.UpsertCatalogEntry(ctx, e); err != nil {
			t.Fatal(err)
		}
		_ = st.EnsureDirs()
		if err := os.WriteFile(st.ModelPath(e.ID), []byte("gguf"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := st.MarkInstalled(ctx, e.ID, st.ModelPath(e.ID), "", 4); err != nil {
			t.Fatal(err)
		}
	}
	install(models.CatalogEntry{ID: "text-model", DisplayName: "Text Model", MemoryNeededBytes: 1,
		Capabilities: contracts.ModelCapabilities{ToolCalling: true}, Purpose: []string{"general"}, Tags: []string{"general"}})
	vision := models.CatalogEntry{ID: "see-model", DisplayName: "See Model", MemoryNeededBytes: 1,
		Capabilities: contracts.ModelCapabilities{ToolCalling: true, Vision: true}, Tags: []string{"vision"},
		Projector: &models.ModelFile{URL: "http://unused/mmproj.gguf"}}
	install(vision)
	if see {
		if err := os.WriteFile(a.Models.Storage().ProjectorPath(vision.ID), []byte("mmproj"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var mu sync.Mutex
	var used []string
	var sent [][]pluginapi.ChatMessage
	a.StubReply = func(modelID string, messages []pluginapi.ChatMessage) string {
		mu.Lock()
		defer mu.Unlock()
		used = append(used, modelID)
		sent = append(sent, messages)
		return "A red bicycle against a brick wall."
	}
	return a, &used, &sent
}

func lastUser(messages []pluginapi.ChatMessage) pluginapi.ChatMessage {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i]
		}
	}
	return pluginapi.ChatMessage{}
}

func runPictureChat(t *testing.T, a *App, convID, message string, attach ...string) string {
	t.Helper()
	ctx := artifacts.WithAttachments(context.Background(), attach)
	stream, err := a.RunChat(ctx, "general-assistant", convID, message, false, "text-model", "")
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for c := range stream {
		b.WriteString(c.Content)
	}
	return b.String()
}

// A picture attached to a message is shown to a model that can see it, even
// when the chat's model reads text only, and a follow-up still sees it (#191).
func TestPicturesReachAModelThatSees(t *testing.T) {
	a, used, sent := pictureApp(t, true)
	ctx := context.Background()
	conv, err := a.Conversations.Create(ctx, "pictures", "general-assistant", "text-model")
	if err != nil {
		t.Fatal(err)
	}
	pic, err := a.Artifacts.Save(ctx, artifacts.Input{Name: "bike.png", Data: []byte("\x89PNG pixels"), ConversationID: conv.ID})
	if err != nil {
		t.Fatal(err)
	}

	runPictureChat(t, a, conv.ID, "What is in this picture?", pic.ID)
	if len(*used) == 0 || (*used)[len(*used)-1] != "see-model" {
		t.Fatalf("models = %v", *used)
	}
	msg := lastUser((*sent)[len(*sent)-1])
	if len(msg.Images) != 1 || !strings.HasPrefix(msg.Images[0], "data:image/png;base64,") {
		t.Fatalf("images = %v", msg.Images)
	}
	if !strings.Contains(msg.Content, "bike.png. It is shown to you") {
		t.Fatalf("content = %q", msg.Content)
	}

	// The next message has no picture of its own: the one before is still
	// shown.
	runPictureChat(t, a, conv.ID, "What color is it?")
	if (*used)[len(*used)-1] != "see-model" || len(lastUser((*sent)[len(*sent)-1]).Images) != 1 {
		t.Fatalf("follow-up: models = %v", *used)
	}

	// Then the chat goes back to its own model.
	runPictureChat(t, a, conv.ID, "Thanks! Tell me a joke.")
	if (*used)[len(*used)-1] != "text-model" || len(lastUser((*sent)[len(*sent)-1]).Images) != 0 {
		t.Fatalf("after: models = %v", *used)
	}
}

// Without a model that can see, the picture isn't sent, and the answer says
// how to get one.
func TestPicturesWithoutAModelThatSees(t *testing.T) {
	a, used, sent := pictureApp(t, false)
	ctx := context.Background()
	conv, _ := a.Conversations.Create(ctx, "pictures", "general-assistant", "text-model")
	pic, _ := a.Artifacts.Save(ctx, artifacts.Input{Name: "bike.png", Data: []byte("\x89PNG pixels"), ConversationID: conv.ID})

	if _, ok := a.seeingModel(ctx); ok {
		t.Fatal("a vision model without its projector counts as seeing")
	}
	runPictureChat(t, a, conv.ID, "Tell me about this photo.", pic.ID)
	if (*used)[len(*used)-1] != "text-model" || len(lastUser((*sent)[len(*sent)-1]).Images) != 0 {
		t.Fatalf("models = %v", *used)
	}
	if !strings.Contains(lastUser((*sent)[len(*sent)-1]).Content, "You cannot see it") {
		t.Fatal("the model wasn't told it can't see the picture")
	}
	msgs, _ := a.Conversations.ListMessages(ctx, conv.ID)
	last := msgs[len(msgs)-1]
	if last.Meta == nil || !strings.Contains(last.Meta.Notice, "No installed model can see pictures") {
		t.Fatalf("notice = %+v", last.Meta)
	}
	for _, m := range a.Capabilities(ctx).Models {
		if m.ID == "see-model" && m.Vision {
			t.Fatal("inventory says a model without its projector can see")
		}
	}
}

func TestWithoutImages(t *testing.T) {
	in := []pluginapi.ChatMessage{{Role: "system", Content: "s"}, {Role: "user", Content: "u", Images: []string{"data:x"}}}
	out := withoutImages(in)
	if len(out[1].Images) != 0 || len(in[1].Images) != 1 {
		t.Fatalf("out = %+v in = %+v", out, in)
	}
	plain := in[:1]
	if got := withoutImages(plain); &got[0] != &plain[0] {
		t.Fatal("messages without pictures were copied")
	}
}

// fakeFrames is a stand-in for PyAV: it writes the frames asked for.
const fakeFrames = `#!/usr/bin/env python3
import json, sys
cfg = json.load(open(sys.argv[3]))
frames = []
for i in range(cfg["count"]):
    open(cfg["out_dir"] + "/frame-%d.jpg" % i, "wb").write(b"\xff\xd8JPEG")
    frames.append({"time": 5.0 + i * 10, "file": "frame-%d.jpg" % i})
json.dump({"duration": 62.0, "has_audio": True, "frames": frames}, sys.stdout)
`

type framesEnv struct{ path string }

func (f framesEnv) Ensure(context.Context, pyenv.Spec, pyenv.Progress) (string, error) {
	return f.path, nil
}
func (f framesEnv) Env() []string                 { return nil }
func (f framesEnv) Unavailable(pyenv.Spec) string { return "" }

// A video is shown to a model that can see as frames sampled through it,
// with when each is (#191).
func TestVideoFramesReachAModelThatSees(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not installed")
	}
	a, used, sent := pictureApp(t, true)
	py := filepath.Join(t.TempDir(), "python")
	if err := os.WriteFile(py, []byte(fakeFrames), 0o755); err != nil {
		t.Fatal(err)
	}
	a.Speech = &speech.Engine{Python: framesEnv{py}, Dir: t.TempDir()}
	ctx := context.Background()
	conv, _ := a.Conversations.Create(ctx, "video", "general-assistant", "text-model")
	clip, err := a.Artifacts.Save(ctx, artifacts.Input{Name: "walk.mp4", Data: []byte("MP4"), ConversationID: conv.ID})
	if err != nil {
		t.Fatal(err)
	}
	runPictureChat(t, a, conv.ID, "What happens in this video?", clip.ID)
	if (*used)[len(*used)-1] != "see-model" {
		t.Fatalf("models = %v", *used)
	}
	msg := lastUser((*sent)[len(*sent)-1])
	if len(msg.Images) != framesPerVideo || !strings.HasPrefix(msg.Images[0], "data:image/jpeg;base64,") {
		t.Fatalf("images = %d", len(msg.Images))
	}
	for _, want := range []string{"Video attached to this message by the user: walk.mp4. It is 1:02 long.", "shown 6 frames", "at 0:05, 0:15, 0:25", "call speech.transcribe"} {
		if !strings.Contains(msg.Content, want) {
			t.Fatalf("content lacks %q:\n%s", want, msg.Content)
		}
	}
	msgs, _ := a.Conversations.ListMessages(ctx, conv.ID)
	last := msgs[len(msgs)-1]
	if last.Meta == nil || !slices.ContainsFunc(last.Meta.Steps, func(s contracts.ActivityStep) bool { return s.Text == "Watched walk.mp4" }) {
		t.Fatalf("steps = %+v", last.Meta)
	}

	// A follow-up uses the frames read before.
	runPictureChat(t, a, conv.ID, "What color is the dog?")
	if len(lastUser((*sent)[len(*sent)-1]).Images) != framesPerVideo {
		t.Fatal("the follow-up didn't see the video")
	}
}
