package auth

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/yeixio/toskar-core/internal/store"
)

func TestRoles(t *testing.T) {
	for _, tc := range []struct {
		r, min Role
		want   bool
	}{
		{RoleOwner, RoleAdmin, true}, {RoleAdmin, RoleAdmin, true}, {RoleMember, RoleAdmin, false},
		{RoleVisitor, RoleMember, false}, {RoleMember, RoleVisitor, true}, {"stranger", RoleVisitor, false},
		{RoleOwner, "stranger", false},
	} {
		if got := tc.r.AtLeast(tc.min); got != tc.want {
			t.Errorf("%s at least %s: %v", tc.r, tc.min, got)
		}
	}
	if Role("king").Valid() || !RoleVisitor.Valid() {
		t.Error("Valid")
	}
}

// The install's implicit user is the Owner, and what was theirs stays
// theirs (#206).
func TestOwnerAndTheirKeys(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	people := NewPeople(db.SQL)
	owner, err := people.Active(ctx, OwnerID)
	if err != nil || owner.Role != RoleOwner || owner.Name != "Owner" {
		t.Fatalf("owner = %+v %v", owner, err)
	}
	keys := NewAPIKeyManager(db.SQL, NewSecretStore(dir))
	rec, secret, err := keys.Create(ctx, "laptop")
	if err != nil || rec.PersonID != OwnerID {
		t.Fatalf("create = %+v %v", rec, err)
	}
	if got, err := keys.Verify(ctx, secret); err != nil || got.PersonID != OwnerID {
		t.Fatalf("verify = %+v %v", got, err)
	}
	if list, _ := keys.List(ctx); len(list) != 1 || list[0].PersonID != OwnerID {
		t.Fatalf("list = %+v", list)
	}

	// Another person, then disabled: known, but not active.
	if _, err := db.SQL.Exec(`INSERT INTO people (id, name, role) VALUES ('sam', 'Sam', 'member')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.SQL.Exec(`INSERT INTO people (id, name, role) VALUES ('bad', 'Bad', 'emperor')`); err == nil {
		t.Fatal("an unknown role was stored")
	}
	if list, _ := people.List(ctx); len(list) != 2 || list[0].ID != OwnerID || list[1].Role != RoleMember {
		t.Fatalf("people = %+v", list)
	}
	if _, err := db.SQL.Exec(`UPDATE people SET disabled_at = '2026-10-08T00:00:00Z' WHERE id = 'sam'`); err != nil {
		t.Fatal(err)
	}
	if _, err := people.Active(ctx, "sam"); err != ErrNoPerson {
		t.Fatalf("disabled person active: %v", err)
	}
	if p, err := people.Get(ctx, "sam"); err != nil || p.Disabled == nil {
		t.Fatalf("get disabled = %+v %v", p, err)
	}

	// Work with no request behind it, such as the scheduler's, is the Owner's.
	if p := PrincipalFrom(ctx); p.Person.ID != OwnerID || p.Person.Role != RoleOwner {
		t.Fatalf("default principal = %+v", p)
	}
}
