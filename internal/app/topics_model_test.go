package app

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yeixio/toskar-core/internal/runlog"
	"github.com/yeixio/toskar-core/pkg/contracts"
	"github.com/yeixio/toskar-core/pkg/pluginapi"
)

// The topic checks run on the smallest verified model that is the
// answering one, already running, or fits beside it; else on the
// answering model (#457).
func TestPickCheckModel(t *testing.T) {
	const gb = 1 << 30
	small := checkCandidate{id: "gemma-3-4b", needs: 5 * gb}
	big := checkCandidate{id: "gemma-3-27b", needs: 20 * gb}
	for _, c := range []struct {
		name      string
		answering string
		needs     uint64
		total     uint64
		verified  []checkCandidate
		want      string
	}{
		{"none verified", "qwen3-32b", 22 * gb, 64 * gb, nil, "qwen3-32b"},
		{"small fits beside", "qwen3-32b", 22 * gb, 64 * gb, []checkCandidate{big, small}, "gemma-3-4b"},
		{"no room beside", "qwen3-32b", 22 * gb, 32 * gb, []checkCandidate{small}, "qwen3-32b"},
		{"already running", "qwen3-32b", 22 * gb, 32 * gb, []checkCandidate{{id: "gemma-3-4b", needs: 5 * gb, running: true}}, "gemma-3-4b"},
		{"the answering one is verified", "gemma-3-27b", 20 * gb, 24 * gb, []checkCandidate{big, small}, "gemma-3-27b"},
		{"memory not known", "qwen3-32b", 22 * gb, 0, []checkCandidate{small}, "gemma-3-4b"},
	} {
		if got := pickCheckModel(c.answering, c.needs, c.total, c.verified); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// With a verified model installed beside a bigger answering one, the
// topic checks run on it and the run trace says so; the answer still
// comes from the answering model (#457).
func TestTopicChecksUseTheVerifiedModel(t *testing.T) {
	t.Setenv("TOSKAR_STUB_INFERENCE", "1")
	t.Setenv("TOSKAR_DISCOVERY_ENABLED", "false")
	a, err := New(Options{DataDir: t.TempDir(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.DB.Close() })
	ctx := context.Background()
	for _, id := range []string{"gemma-3-4b-q4", "qwen2.5-14b-q4"} {
		if _, err := a.DB.SQL.Exec(`INSERT OR IGNORE INTO models (id, display_name) VALUES (?, ?)`, id, id); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(a.Config.Get().ModelsDir, id+".gguf")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("stub"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := a.DB.SQL.Exec(`INSERT INTO installed_models (model_id, path, sha256, size_bytes, status) VALUES (?, ?, '', 4, 'installed')`, id, path); err != nil {
			t.Fatal(err)
		}
	}
	p, _ := a.Profiles.Get(ctx, "general-assistant")
	for i := range p.Roles {
		p.Roles[i].ModelID = "qwen2.5-14b-q4"
	}
	p.Topics = &contracts.TopicPolicy{StaysOn: "Tires", Strictness: contracts.TopicsEnforce}
	if err := a.Profiles.Update(ctx, p); err != nil {
		t.Fatal(err)
	}
	// Room for both, whatever computer runs the test.
	a.memTotal.Store(64 << 30)
	var mu sync.Mutex
	calls := map[string]string{}
	a.StubReply = func(model string, msgs []pluginapi.ChatMessage) string {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case strings.HasPrefix(msgs[0].Content, "You check each message"):
			calls["message check"] = model
			return "on_topic"
		case strings.HasPrefix(msgs[0].Content, "You check each answer"):
			calls["answer check"] = model
			return "on_topic"
		}
		calls["answer"] = model
		return "Winter tires grip better."
	}
	conv, _ := a.Conversations.Create(ctx, "t", "general-assistant", "qwen2.5-14b-q4")
	stream, err := a.RunChat(ctx, "general-assistant", conv.ID, "Snow tires?", false, "qwen2.5-14b-q4", "")
	if err != nil {
		t.Fatal(err)
	}
	for range stream {
	}
	mu.Lock()
	defer mu.Unlock()
	if calls["message check"] != "gemma-3-4b-q4" || calls["answer check"] != "gemma-3-4b-q4" || calls["answer"] != "qwen2.5-14b-q4" {
		t.Fatalf("calls: %v", calls)
	}
	var runs []runlog.Run
	for i := 0; i < 50 && len(runs) == 0; i++ {
		runs, _ = a.RunLog.List(ctx, conv.ID, 1)
		time.Sleep(10 * time.Millisecond)
	}
	if len(runs) != 1 || !slices.ContainsFunc(runs[0].Strategy, func(s string) bool { return strings.Contains(s, "Gemma") }) {
		t.Fatalf("run: %+v", runs)
	}
}
