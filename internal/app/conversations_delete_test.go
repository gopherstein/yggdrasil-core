package app

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"testing"

	"github.com/yeixio/toskar-core/internal/artifacts"
	"github.com/yeixio/toskar-core/internal/auth"
)

// Deleting chats deletes only the caller's (#452): Sam's bulk delete skips
// the Owner's chat as not found and leaves its messages and files, and a
// single delete of someone else's chat fails without touching its files.
func TestDeleteConversationsOnlyTheirOwn(t *testing.T) {
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

	chat := func(ctx context.Context, title string) (string, string) {
		t.Helper()
		c, err := a.Conversations.Create(ctx, title, "general-assistant", "")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := a.Conversations.AddMessage(ctx, c.ID, "user", "hi"); err != nil {
			t.Fatal(err)
		}
		f, err := a.Artifacts.Save(ctx, artifacts.Input{ConversationID: c.ID, Name: "notes.txt", Producer: "assistant", Data: []byte("notes")})
		if err != nil {
			t.Fatal(err)
		}
		return c.ID, f.ID
	}
	sam1, sam1File := chat(asSam, "one")
	sam2, _ := chat(asSam, "two")
	mine, mineFile := chat(owner, "the Owner's")

	got, err := a.deleteConversations(asSam, []string{sam1, mine, sam2, "nope"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Deleted, []string{sam1, sam2}) {
		t.Errorf("deleted = %v, want Sam's two", got.Deleted)
	}
	if len(got.Skipped) != 2 || got.Skipped[0].ID != mine || got.Skipped[0].Reason != "not_found" || got.Skipped[1].ID != "nope" {
		t.Errorf("skipped = %+v", got.Skipped)
	}
	if _, err := a.Conversations.Get(asSam, sam1); err == nil {
		t.Error("Sam's chat is still there")
	}
	if _, _, err := a.Artifacts.Read(asSam, sam1File); err == nil {
		t.Error("Sam's chat's file is still there")
	}
	if msgs, err := a.Conversations.ListMessages(owner, mine); err != nil || len(msgs) != 1 {
		t.Errorf("the Owner's messages = %d, %v", len(msgs), err)
	}

	// A single delete of someone else's chat fails, and their file stays.
	if err := a.deleteConversation(asSam, mine); err == nil {
		t.Error("Sam deleted the Owner's chat")
	}
	if _, data, err := a.Artifacts.Read(owner, mineFile); err != nil || string(data) != "notes" {
		t.Errorf("the Owner's file = %q, %v", data, err)
	}
	if err := a.deleteConversation(owner, mine); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.Artifacts.Read(owner, mineFile); err == nil {
		t.Error("the Owner's file is still there after their delete")
	}
}
