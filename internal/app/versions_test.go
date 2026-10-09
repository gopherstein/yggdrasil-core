package app

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/yeixio/toskar-core/internal/turnopts"
	"github.com/yeixio/toskar-core/pkg/pluginapi"
)

// Retrying an answer or editing a message in a past chat adds a version of
// that point; the model is sent only the chat up to it, and the earlier
// versions stay (#447).
func TestRetryAndEditInAPastChat(t *testing.T) {
	t.Setenv("TOSKAR_STUB_INFERENCE", "1")
	t.Setenv("TOSKAR_DISCOVERY_ENABLED", "false")
	a, err := New(Options{DataDir: t.TempDir(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.DB.Close() })
	ctx := context.Background()
	var mu sync.Mutex
	var sent []pluginapi.ChatMessage
	n := 0
	a.StubReply = func(_ string, msgs []pluginapi.ChatMessage) string {
		mu.Lock()
		defer mu.Unlock()
		sent = msgs
		n++
		return "answer " + string(rune('0'+n))
	}
	conv, _ := a.Conversations.Create(ctx, "t", "general-assistant", "auto")
	ask := func(ctx context.Context, message string) {
		t.Helper()
		stream, err := a.RunChat(ctx, "general-assistant", conv.ID, message, false, "auto", "")
		if err != nil {
			t.Fatal(err)
		}
		for c := range stream {
			if c.Error != "" {
				t.Fatal(c.Error)
			}
		}
	}
	shown := func() string {
		msgs, _ := a.Conversations.ListMessages(ctx, conv.ID)
		var parts []string
		for _, m := range msgs {
			parts = append(parts, m.Content)
		}
		return strings.Join(parts, " | ")
	}
	history := func() string {
		mu.Lock()
		defer mu.Unlock()
		var parts []string
		for _, m := range sent {
			if m.Role != "system" {
				parts = append(parts, m.Content)
			}
		}
		return strings.Join(parts, " | ")
	}
	ask(ctx, "first question")
	ask(ctx, "second question")
	ask(ctx, "third question")
	if got := shown(); got != "first question | answer 1 | second question | answer 2 | third question | answer 3" {
		t.Fatalf("chat: %q", got)
	}
	msgs, _ := a.Conversations.ListMessages(ctx, conv.ID)

	// Retry the first answer: the model sees only the first question.
	ask(turnopts.WithBranch(ctx, turnopts.Branch{RetryOf: msgs[1].ID}), "")
	if got := history(); !strings.HasSuffix(got, "first question") || strings.Contains(got, "second question") {
		t.Fatalf("a retry was sent: %q", got)
	}
	if got := shown(); got != "first question | answer 4" {
		t.Fatalf("after a retry: %q", got)
	}
	after, _ := a.Conversations.ListMessages(ctx, conv.ID)
	if v := after[1].Versions; v == nil || v.Count != 2 || v.Index != 2 {
		t.Fatalf("versions: %+v", v)
	}

	// The earlier version, and what followed it, are still there.
	if _, err := a.Conversations.ShowVersion(ctx, conv.ID, msgs[1].ID); err != nil {
		t.Fatal(err)
	}
	if got := shown(); got != "first question | answer 1 | second question | answer 2 | third question | answer 3" {
		t.Fatalf("first version again: %q", got)
	}

	// Edit the second question: the model sees the chat before it, and
	// the edited question.
	ask(turnopts.WithBranch(ctx, turnopts.Branch{EditOf: msgs[2].ID}), "second question, edited")
	if got := history(); !strings.HasSuffix(got, "first question | answer 1 | second question, edited") || strings.Contains(got, "third") {
		t.Fatalf("an edit was sent: %q", got)
	}
	if got := shown(); got != "first question | answer 1 | second question, edited | answer 5" {
		t.Fatalf("after an edit: %q", got)
	}
	// The chat goes on from the version shown.
	ask(ctx, "fourth question")
	if got := history(); !strings.HasSuffix(got, "second question, edited | answer 5 | fourth question") {
		t.Fatalf("going on: %q", got)
	}

	// Retrying something that isn't an answer in this chat is refused.
	if _, err := a.RunChat(turnopts.WithBranch(ctx, turnopts.Branch{RetryOf: msgs[0].ID}), "general-assistant", conv.ID, "", false, "auto", ""); err == nil {
		t.Fatal("retried a question")
	}
}
