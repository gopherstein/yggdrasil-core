package auth

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"github.com/yeixio/toskar-core/internal/store"
)

// Members and Visitors can be pinned to profiles, by role or one by one;
// the Owner and Admins can't (#345).
func TestProfilePins(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	people := NewPeople(db.SQL)
	sam, _ := people.Create(ctx, "Sam", RoleMember)
	ada, _ := people.Create(ctx, "Ada", RoleAdmin)
	allowed := func(id string) []string {
		t.Helper()
		person, err := people.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		list, err := people.AllowedProfiles(ctx, person)
		if err != nil {
			t.Fatal(err)
		}
		return list
	}
	if allowed(sam.ID) != nil {
		t.Fatal("pinned by default")
	}
	if err := people.SetRoleProfiles(ctx, RoleMember, []string{"tires", " tires", "", "wheels"}); err != nil {
		t.Fatal(err)
	}
	if got := allowed(sam.ID); !slices.Equal(got, []string{"tires", "wheels"}) {
		t.Fatalf("role pin: %v", got)
	}
	if roles, _ := people.RoleProfiles(ctx); !slices.Equal(roles[RoleMember], []string{"tires", "wheels"}) {
		t.Fatalf("roles: %v", roles)
	}
	// Their own pin replaces the role's; empty is any profile.
	if _, err := people.SetProfiles(ctx, sam.ID, &[]string{"wheels"}); err != nil {
		t.Fatal(err)
	}
	if got := allowed(sam.ID); !slices.Equal(got, []string{"wheels"}) {
		t.Fatalf("own pin: %v", got)
	}
	if p, _ := people.SetProfiles(ctx, sam.ID, &[]string{}); p.Profiles == nil || allowed(sam.ID) != nil {
		t.Fatalf("any profile: %+v", p)
	}
	if _, err := people.SetProfiles(ctx, sam.ID, nil); err != nil || !slices.Equal(allowed(sam.ID), []string{"tires", "wheels"}) {
		t.Fatalf("back to the role's: %v", err)
	}
	if err := people.SetRoleProfiles(ctx, RoleMember, nil); err != nil || allowed(sam.ID) != nil {
		t.Fatal("role unpinned")
	}
	// The Owner and Admins are never pinned.
	if _, err := people.SetProfiles(ctx, ada.ID, &[]string{"tires"}); err != ErrPinRole {
		t.Fatalf("admin: %v", err)
	}
	if err := people.SetRoleProfiles(ctx, RoleAdmin, []string{"tires"}); err != ErrPinRole {
		t.Fatalf("admin role: %v", err)
	}
	if _, err := people.SetProfiles(ctx, OwnerID, &[]string{"tires"}); err != ErrPinRole {
		t.Fatalf("owner: %v", err)
	}
}
