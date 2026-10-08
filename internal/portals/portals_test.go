package portals_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/portals"
	"github.com/yeixio/toskar-core/internal/store"
)

func ptr[T any](v T) *T { return &v }

func TestPortals(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := portals.NewStore(db.SQL)
	ctx := context.Background()

	if _, err := s.Create(ctx, portals.Input{Slug: ptr("Support"), Name: ptr("Help desk")}); !errors.Is(err, portals.ErrNoPasscode) {
		t.Fatalf("a passcode portal without one: %v", err)
	}
	for _, bad := range []string{"a", "-x", "x-", "has space", "UPPER/slash"} {
		if _, err := s.Create(ctx, portals.Input{Slug: ptr(bad), Name: ptr("x"), Access: ptr("open")}); !errors.Is(err, portals.ErrBadSlug) {
			t.Errorf("slug %q: %v", bad, err)
		}
	}
	p, err := s.Create(ctx, portals.Input{Slug: ptr("Support"), Name: ptr("Help desk"), Passcode: ptr("tide-pool")})
	if err != nil {
		t.Fatal(err)
	}
	if p.Slug != "support" || p.Tools != portals.ToolsNone || p.Memory || !p.Enabled || !p.HasPasscode || string(p.Branding) != "{}" {
		t.Fatalf("new portal %+v", p)
	}
	if !p.CheckPasscode("tide-pool") || p.CheckPasscode("tide") {
		t.Fatal("passcode check")
	}
	if raw, _ := json.Marshal(p); containsHash(string(raw)) {
		t.Fatalf("the passcode's hash is sent: %s", raw)
	}
	if _, err := s.Create(ctx, portals.Input{Slug: ptr("support"), Name: ptr("Again"), Access: ptr("open")}); !errors.Is(err, portals.ErrSlugTaken) {
		t.Fatalf("a taken address: %v", err)
	}
	branding := json.RawMessage(`{"accent":"#0a7"}`)
	p, err = s.Update(ctx, p.ID, portals.Input{Tools: ptr("read_only"), Language: ptr("pt-BR"), Branding: &branding, Enabled: ptr(false)})
	if err != nil || p.Tools != portals.ToolsReadOnly || p.Language != "pt-BR" || p.Enabled || string(p.Branding) != `{"accent":"#0a7"}` {
		t.Fatalf("update %+v %v", p, err)
	}
	if _, err := s.Update(ctx, p.ID, portals.Input{Passcode: ptr("")}); !errors.Is(err, portals.ErrNoPasscode) {
		t.Fatalf("removing a passcode portal's passcode: %v", err)
	}
	notObject := json.RawMessage(`[1]`)
	if _, err := s.Update(ctx, p.ID, portals.Input{Branding: &notObject}); !errors.Is(err, portals.ErrBadBranding) {
		t.Fatalf("branding that isn't an object: %v", err)
	}
	if got, err := s.BySlug(ctx, "SUPPORT"); err != nil || got.ID != p.ID {
		t.Fatalf("by slug: %v", err)
	}

	people := auth.NewPeople(db.SQL)
	guest, err := people.CreateGuest(ctx, p.ID)
	if err != nil || guest.PortalID != p.ID || guest.Role != auth.RoleVisitor {
		t.Fatalf("guest %+v %v", guest, err)
	}
	if list, _ := people.List(ctx); len(list) != 1 {
		t.Fatalf("guests listed among people: %+v", list)
	}
	if err := s.Delete(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := people.Active(ctx, guest.ID); !errors.Is(err, auth.ErrNoPerson) {
		t.Fatalf("a removed portal's guest is still active: %v", err)
	}
}

func containsHash(s string) bool {
	return strings.Contains(s, "passcode_hash") || strings.Contains(s, "$argon2")
}

// The Owner sees a portal's visitors' conversations and use, and they go
// after the portal's retention (#205).
func TestOwnersView(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	s := portals.NewStore(db.SQL)
	ctx := context.Background()
	p, err := s.Create(ctx, portals.Input{Slug: ptr("shop"), Name: ptr("Shop"), Access: ptr("open"), RetentionDays: ptr(30)})
	if err != nil || p.RetentionDays != 30 {
		t.Fatalf("create %+v %v", p, err)
	}
	if _, err := s.Update(ctx, p.ID, portals.Input{RetentionDays: ptr(-1)}); !errors.Is(err, portals.ErrBadRetention) {
		t.Fatalf("negative retention: %v", err)
	}
	people := auth.NewPeople(db.SQL)
	robin, _ := people.CreateInvitedGuest(ctx, p.ID, "Robin")
	anon, _ := people.CreateGuest(ctx, p.ID)
	stale, _ := people.CreateGuest(ctx, p.ID)
	now := time.Now().UTC()
	at := func(d time.Duration) string { return now.Add(d).Format(time.RFC3339Nano) }
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.SQL.ExecContext(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO conversations (id, title, created_at, updated_at, person_id) VALUES ('c1', 'Winter tires', ?, ?, ?)`, at(-time.Hour), at(-time.Hour), robin.ID)
	exec(`INSERT INTO messages (id, conversation_id, role, content, created_at) VALUES ('m1', 'c1', 'user', 'Winter tires?', ?), ('m2', 'c1', 'assistant', 'Yes.', ?), ('m3', 'c1', 'system', 'hidden', ?)`, at(-time.Hour), at(-59*time.Minute), at(-58*time.Minute))
	exec(`INSERT INTO conversations (id, title, created_at, updated_at, person_id) VALUES ('c2', 'Old', ?, ?, ?)`, at(-40*24*time.Hour), at(-40*24*time.Hour), anon.ID)
	exec(`INSERT INTO messages (id, conversation_id, role, content, created_at) VALUES ('m4', 'c2', 'user', 'Hello?', ?)`, at(-40*24*time.Hour))
	exec(`INSERT INTO conversations (id, title, created_at, updated_at, person_id) VALUES ('c3', 'The owner''s own', ?, ?, 'owner')`, at(-time.Hour), at(-time.Hour))
	exec(`UPDATE people SET created_at = ? WHERE id IN (?, ?)`, at(-40*24*time.Hour), anon.ID, stale.ID)

	list, err := s.Conversations(ctx, p.ID, 0)
	if err != nil || len(list) != 2 || list[0].ID != "c1" || list[0].Visitor != "Robin" || list[0].Messages != 3 {
		t.Fatalf("conversations %+v %v", list, err)
	}
	msgs, err := s.Messages(ctx, p.ID, "c1")
	if err != nil || len(msgs) != 2 || msgs[0].Content != "Winter tires?" || msgs[1].Role != "assistant" {
		t.Fatalf("messages %+v %v", msgs, err)
	}
	if _, err := s.Messages(ctx, p.ID, "c3"); !errors.Is(err, portals.ErrNotFound) {
		t.Fatalf("the owner's own chat through a portal: %v", err)
	}
	u, err := s.Usage(ctx, p.ID)
	if err != nil || u.Messages7 != 1 || u.Visitors7 != 1 || u.Messages30 != 1 || u.Conversations30 != 1 {
		t.Fatalf("usage %+v %v", u, err)
	}

	gone, err := s.Prune(ctx)
	if err != nil || gone != 1 {
		t.Fatalf("prune %d %v", gone, err)
	}
	if list, _ := s.Conversations(ctx, p.ID, 0); len(list) != 1 {
		t.Fatalf("after prune %+v", list)
	}
	// An anonymous visitor with nothing left goes; Robin, invited, stays.
	if _, err := people.Get(ctx, stale.ID); !errors.Is(err, auth.ErrNoPerson) {
		t.Fatalf("a stale anonymous visitor stayed: %v", err)
	}
	if _, err := people.Get(ctx, robin.ID); err != nil {
		t.Fatalf("robin: %v", err)
	}
	var owners int
	_ = db.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM conversations WHERE id = 'c3'`).Scan(&owners)
	if owners != 1 {
		t.Fatal("the owner's own chat was pruned")
	}
}
