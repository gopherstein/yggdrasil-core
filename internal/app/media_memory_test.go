package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yeixio/toskar-core/internal/imagegen"
)

func ids(ms []loadedModel) string {
	var out []string
	for _, m := range ms {
		out = append(out, m.ModelID)
	}
	return strings.Join(out, ",")
}

// The models in a picture's way are unloaded: on a graphics card with its
// own memory, every model on it; in shared memory, the least recently
// used, until the job fits (#541).
func TestMediaUnloads(t *testing.T) {
	now := time.Now()
	const gb = int64(1 << 30)
	loaded := []loadedModel{
		{ModelID: "chat", Memory: 10 * gb, OnGPU: true, LastUsed: now},
		{ModelID: "judge", Memory: 4 * gb, OnGPU: true, LastUsed: now.Add(-time.Hour)},
		{ModelID: "embed", Memory: 1 * gb, LastUsed: now.Add(-2 * time.Hour)},
	}
	if got := ids(mediaUnloads(loaded, 12*gb, 64*gb, true)); got != "chat,judge" {
		t.Fatalf("dedicated GPU unloads %q", got)
	}
	// 24 GB shared: 15 loaded + 12 + 2 spare doesn't fit; dropping the
	// oldest (embed, then judge) leaves 10 + 12 + 2 = 24.
	if got := ids(mediaUnloads(loaded, 12*gb, 24*gb, false)); got != "embed,judge" {
		t.Fatalf("shared memory unloads %q", got)
	}
	if got := ids(mediaUnloads(loaded, 12*gb, 64*gb, false)); got != "" {
		t.Fatalf("unloaded with room to spare: %q", got)
	}
	// Memory unknown: only what's on a dedicated GPU goes.
	if got := ids(mediaUnloads(loaded, 12*gb, 0, false)); got != "" {
		t.Fatalf("unknown memory unloads %q", got)
	}
}

// One picture or clip at a time, and a model start waits for it; a model
// already running doesn't wait.
func TestMediaHoldsStarts(t *testing.T) {
	a := &App{}
	ctx := context.Background()
	release, err := a.media.enter(ctx, "clip")
	if err != nil {
		t.Fatal(err)
	}
	second := make(chan struct{})
	go func() {
		r, err := a.media.enter(ctx, "picture")
		if err == nil {
			r()
		}
		close(second)
	}()
	started := make(chan error, 1)
	go func() { started <- a.waitForMedia(ctx, "qwen") }()
	select {
	case <-second:
		t.Fatal("a second job ran beside the first")
	case <-started:
		t.Fatal("a model started during the clip")
	case <-time.After(100 * time.Millisecond):
	}
	short, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	if err := a.waitForMedia(short, "other"); err == nil {
		t.Fatal("a wait ended before the clip")
	}
	release()
	release() // twice is harmless
	if err := <-started; err != nil {
		t.Fatal(err)
	}
	<-second
	if _, _, running := a.media.running(); running {
		t.Fatal("still held")
	}
	if err := a.waitForMedia(ctx, "qwen"); err != nil {
		t.Fatal(err)
	}
}

// A model works well on a paired computer by its verdict, or, from a
// computer that reports none, by its memory (#550).
func TestWorksWell(t *testing.T) {
	for _, c := range []struct {
		m    imagegen.ModelStatus
		want bool
	}{
		{imagegen.ModelStatus{Verdict: imagegen.VerdictWorks}, true},
		{imagegen.ModelStatus{Verdict: imagegen.VerdictTight}, false},
		{imagegen.ModelStatus{Verdict: imagegen.VerdictUnsupported}, false},
		{imagegen.ModelStatus{}, true},
		{imagegen.ModelStatus{TightMemory: true}, false},
		{imagegen.ModelStatus{TooLittleMemory: true}, false},
	} {
		if got := worksWell(c.m); got != c.want {
			t.Errorf("%+v = %v", c.m, got)
		}
	}
}
