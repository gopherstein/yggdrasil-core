package app

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/automations"
	"github.com/yeixio/toskar-core/pkg/pluginapi"
)

// A chat runs as its person (#206): Sam's turn gets Sam's memories, never
// the Owner's, and the chat is Sam's. An automation runs as its person, and
// a disabled person's doesn't run.
func TestChatsAndAutomationsRunAsTheirPerson(t *testing.T) {
	t.Setenv("TOSKAR_STUB_INFERENCE", "1")
	t.Setenv("TOSKAR_DISCOVERY_ENABLED", "false")
	a, err := New(Options{DataDir: t.TempDir(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.DB.Close() })
	if _, err := a.DB.SQL.Exec(`INSERT INTO people (id, name, role) VALUES ('sam', 'Sam', 'member')`); err != nil {
		t.Fatal(err)
	}
	sam, err := a.People.Active(context.Background(), "sam")
	if err != nil {
		t.Fatal(err)
	}
	owner := context.Background()
	asSam := auth.AsPerson(context.Background(), sam)
	if _, _, err := a.Muninn.Add(owner, "My favorite color is ultraviolet", "preferences", "chat", ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Muninn.Add(asSam, "My favorite color is teal", "preferences", "chat", ""); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var prompts []string
	a.StubReply = func(_ string, messages []pluginapi.ChatMessage) string {
		mu.Lock()
		defer mu.Unlock()
		var b strings.Builder
		for _, m := range messages {
			b.WriteString(m.Content + "\n")
		}
		prompts = append(prompts, b.String())
		return "Teal."
	}
	conv, err := a.Conversations.Create(asSam, "colors", "general-assistant", "")
	if err != nil {
		t.Fatal(err)
	}
	stream, err := a.RunChat(asSam, "general-assistant", conv.ID, "What is my favorite color?", false, "", "")
	if err != nil {
		t.Fatal(err)
	}
	for range stream {
	}
	all := strings.Join(prompts, "\n")
	if !strings.Contains(all, "teal") || strings.Contains(all, "ultraviolet") {
		t.Fatalf("sam's turn saw the wrong memories:\n%s", all)
	}
	if list, _ := a.Conversations.List(owner); len(list) != 0 {
		t.Fatalf("the owner lists sam's chat: %+v", list)
	}
	if msgs, _ := a.Conversations.ListMessages(asSam, conv.ID); len(msgs) != 2 {
		t.Fatalf("sam's chat has %d messages", len(msgs))
	}

	auto := automations.Automation{ID: "a1", PersonID: "sam"}
	ctx, err := a.asAutomationPerson(context.Background(), auto)
	if err != nil || auth.PersonID(ctx) != "sam" {
		t.Fatalf("automation runs as %q, %v", auth.PersonID(ctx), err)
	}
	if _, err := a.DB.SQL.Exec(`UPDATE people SET disabled_at = '2026-10-08T00:00:00Z' WHERE id = 'sam'`); err != nil {
		t.Fatal(err)
	}
	if _, err := a.asAutomationPerson(context.Background(), auto); err == nil {
		t.Fatal("a disabled person's automation ran")
	}
}
