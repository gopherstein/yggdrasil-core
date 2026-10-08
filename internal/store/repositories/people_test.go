package repositories

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/automations"
	"github.com/yeixio/toskar-core/internal/store"
)

func as(id string, role auth.Role) context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{Person: auth.Person{ID: id, Role: role}, Via: auth.ViaAPIKey})
}

// Chats and automations are their person's (#206): nobody else lists,
// reads, changes, or deletes them, not even the Owner, while the
// scheduler, working for everyone, finds what's due across them all.
func TestChatsAndAutomationsArePrivate(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.SQL.Exec(`INSERT INTO people (id, name, role) VALUES ('sam', 'Sam', 'member')`); err != nil {
		t.Fatal(err)
	}
	owner, sam := as(auth.OwnerID, auth.RoleOwner), as("sam", auth.RoleMember)
	convs := NewConversationRepo(db.SQL)

	mine, _ := convs.Create(owner, "Owner's chat", "", "")
	theirs, _ := convs.Create(sam, "Sam's chat", "", "")
	_, _ = convs.AddMessage(sam, theirs.ID, "user", "secret plans")
	if list, _ := convs.List(owner); len(list) != 1 || list[0].ID != mine.ID {
		t.Fatalf("owner lists %+v", list)
	}
	if list, _ := convs.List(sam); len(list) != 1 || list[0].ID != theirs.ID {
		t.Fatalf("sam lists %+v", list)
	}
	if _, err := convs.Get(owner, theirs.ID); err == nil {
		t.Fatal("the owner read sam's chat")
	}
	if msgs, _ := convs.ListMessages(owner, theirs.ID); len(msgs) != 0 {
		t.Fatalf("the owner read sam's messages: %+v", msgs)
	}
	if msgs, _ := convs.ListMessages(sam, theirs.ID); len(msgs) != 1 {
		t.Fatalf("sam's own messages: %+v", msgs)
	}
	title := "renamed"
	if _, err := convs.Update(owner, theirs.ID, ConversationPatch{Title: &title}); err == nil {
		t.Fatal("the owner renamed sam's chat")
	}
	if err := convs.Delete(owner, theirs.ID); err == nil {
		t.Fatal("the owner deleted sam's chat")
	}
	if err := convs.DeleteAll(owner); err != nil {
		t.Fatal(err)
	}
	if list, _ := convs.List(sam); len(list) != 1 {
		t.Fatal("deleting the owner's chats deleted sam's")
	}
	if msgs, _ := convs.ListMessages(sam, theirs.ID); len(msgs) != 1 {
		t.Fatal("deleting the owner's chats deleted sam's messages")
	}

	autos := NewAutomationRepo(db.SQL)
	now := time.Date(2026, 10, 8, 7, 0, 0, 0, time.UTC)
	in := automations.CreateInput{ModelID: "m", Name: "Sam's digest", Prompt: "Summarize",
		Schedule: automations.Schedule{Kind: automations.KindDaily, TimeZone: "UTC", Hour: 8}}
	samAuto, err := autos.Create(sam, in, now)
	if err != nil || samAuto.PersonID != "sam" {
		t.Fatalf("create = %+v %v", samAuto, err)
	}
	if list, _ := autos.List(owner); len(list) != 0 {
		t.Fatalf("the owner lists sam's automations: %+v", list)
	}
	if _, err := autos.Get(owner, samAuto.ID); err == nil {
		t.Fatal("the owner read sam's automation")
	}
	if err := autos.Delete(owner, samAuto.ID); err == nil {
		t.Fatal("the owner deleted sam's automation")
	}
	if got, err := autos.Get(sam, samAuto.ID); err != nil || got.PersonID != "sam" {
		t.Fatalf("sam's own = %+v %v", got, err)
	}
	system := auth.WithSystem(context.Background())
	if list, _ := autos.List(system); len(list) != 1 {
		t.Fatalf("the scheduler sees %d automations", len(list))
	}
	if due, _ := autos.Due(system, now.Add(2*time.Hour)); len(due) != 1 || due[0].PersonID != "sam" {
		t.Fatalf("due = %+v", due)
	}
}
