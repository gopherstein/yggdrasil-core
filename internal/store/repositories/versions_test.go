package repositories_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeixio/toskar-core/internal/store"
	"github.com/yeixio/toskar-core/internal/store/repositories"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

func contents(msgs []contracts.Message) string {
	var parts []string
	for _, m := range msgs {
		parts = append(parts, m.Content)
	}
	return strings.Join(parts, " | ")
}

// A chat from before versions reads as one version at every point; a
// retry or an edit adds a version there, shown with what follows it, and
// the earlier one stays (#447).
func TestMessageVersions(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := repositories.NewConversationRepo(db.SQL)
	ctx := context.Background()
	conv, err := repo.Create(ctx, "t", "general-assistant", "")
	if err != nil {
		t.Fatal(err)
	}
	// Messages saved before versions have no parent.
	for i, m := range [][2]string{{"user", "Q1"}, {"assistant", "A1"}, {"user", "Q2"}, {"assistant", "A2"}} {
		if _, err := db.SQL.Exec(`INSERT INTO messages (id, conversation_id, role, content) VALUES (?, ?, ?, ?)`,
			"old"+string(rune('0'+i)), conv.ID, m[0], m[1]); err != nil {
			t.Fatal(err)
		}
	}
	list, err := repo.ListMessages(ctx, conv.ID)
	if err != nil || contents(list) != "Q1 | A1 | Q2 | A2" || list[1].Versions != nil || list[1].ParentID != "old0" {
		t.Fatalf("old chat: %q %+v %v", contents(list), list, err)
	}

	// New messages go on at the end.
	q3, _ := repo.AddMessage(ctx, conv.ID, "user", "Q3")
	if q3.ParentID != "old3" {
		t.Fatalf("appended after %q", q3.ParentID)
	}
	a3, _ := repo.AddMessage(ctx, conv.ID, "assistant", "A3")

	// Retrying A1 adds a version after Q1, shown, and the chat ends there.
	a1b, err := repo.AddReply(ctx, conv.ID, "old0", "assistant", "A1 again", nil)
	if err != nil {
		t.Fatal(err)
	}
	list, _ = repo.ListMessages(ctx, conv.ID)
	if contents(list) != "Q1 | A1 again" || list[1].Versions == nil || list[1].Versions.Index != 2 || list[1].Versions.Count != 2 {
		t.Fatalf("after a retry: %q %+v", contents(list), list[1].Versions)
	}
	// The chat goes on from the version shown.
	if q, _ := repo.AddMessage(ctx, conv.ID, "user", "Q2b"); q.ParentID != a1b.ID {
		t.Fatalf("went on after %q", q.ParentID)
	}

	// Showing the first version again brings back what followed it.
	list, err = repo.ShowVersion(ctx, conv.ID, "old1")
	if err != nil || contents(list) != "Q1 | A1 | Q2 | A2 | Q3 | A3" || list[1].Versions.Index != 1 {
		t.Fatalf("first version: %q %v", contents(list), err)
	}

	// An edit of Q2 is a version beside it.
	if _, err := repo.AddReply(ctx, conv.ID, "old1", "user", "Q2 edited", nil); err != nil {
		t.Fatal(err)
	}
	if list, _ = repo.ListMessages(ctx, conv.ID); contents(list) != "Q1 | A1 | Q2 edited" {
		t.Fatalf("after an edit: %q", contents(list))
	}

	// A path to any message, whatever is shown.
	path, err := repo.PathTo(ctx, conv.ID, a3.ID)
	if err != nil || contents(path) != "Q1 | A1 | Q2 | A2 | Q3 | A3" {
		t.Fatalf("path: %q %v", contents(path), err)
	}
	if _, err := repo.PathTo(ctx, conv.ID, "nope"); !errors.Is(err, repositories.ErrNoMessage) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := repo.ShowVersion(ctx, conv.ID, "nope"); !errors.Is(err, repositories.ErrNoMessage) {
		t.Fatalf("show missing: %v", err)
	}
	if m, err := repo.Message(ctx, conv.ID, "old1"); err != nil || m.Versions == nil || m.Versions.Count != 2 {
		t.Fatalf("message: %+v %v", m, err)
	}
}
