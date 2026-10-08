package portals_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

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
