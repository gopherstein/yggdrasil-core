package auth

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yeixio/toskar-core/internal/store"
)

func TestPasswords(t *testing.T) {
	hash, err := HashPassword("correct horse battery")
	if err != nil || !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("hash = %q %v", hash, err)
	}
	if !CheckPassword(hash, "correct horse battery") || CheckPassword(hash, "correct horse batterY") {
		t.Fatal("check")
	}
	if again, _ := HashPassword("correct horse battery"); again == hash {
		t.Fatal("no salt")
	}
	if _, err := HashPassword("short"); err != ErrWeakPassword {
		t.Fatalf("short: %v", err)
	}
	if CheckPassword("$bcrypt$x", "anything") || CheckPassword("", "") {
		t.Fatal("a malformed hash matched")
	}
}

func TestLimiter(t *testing.T) {
	l := NewLimiter(3, time.Minute)
	now := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		if l.Blocked("sam") {
			t.Fatalf("blocked after %d", i)
		}
		l.Fail("sam")
	}
	if !l.Blocked("sam") || l.Blocked("alex") {
		t.Fatal("blocking")
	}
	now = now.Add(61 * time.Second)
	if l.Blocked("sam") {
		t.Fatal("still blocked after the window")
	}
	l.Fail("sam")
	l.Forget("sam")
	if len(l.fails["sam"]) != 0 {
		t.Fatal("forget")
	}
}

// Sessions, invites, and people's sign-ins (#206).
func TestSessionsInvitesAndSignIn(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	people := NewPeople(db.SQL)
	sam, err := people.Create(ctx, "  Sam  ", RoleMember)
	if err != nil || sam.Name != "Sam" || sam.SignIn {
		t.Fatalf("create = %+v %v", sam, err)
	}
	if _, err := people.Create(ctx, "Two", RoleOwner); err != ErrOneOwner {
		t.Fatalf("second owner: %v", err)
	}
	if _, err := people.Create(ctx, "", RoleMember); err != ErrBadName {
		t.Fatalf("no name: %v", err)
	}

	// An invite: peeked, used once, then gone.
	invites := NewInvites(db.SQL)
	token, _, err := invites.Create(ctx, sam.ID, InviteNew, OwnerID)
	if err != nil {
		t.Fatal(err)
	}
	if id, kind, err := invites.Peek(ctx, token); err != nil || id != sam.ID || kind != InviteNew {
		t.Fatalf("peek = %q %q %v", id, kind, err)
	}
	if _, _, err := invites.Use(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, _, err := invites.Use(ctx, token); err != ErrNoInvite {
		t.Fatalf("used twice: %v", err)
	}
	// A new link replaces an unused one; an old link expires.
	first, _, _ := invites.Create(ctx, sam.ID, InviteReset, OwnerID)
	second, _, _ := invites.Create(ctx, sam.ID, InviteReset, OwnerID)
	if _, _, err := invites.Peek(ctx, first); err != ErrNoInvite {
		t.Fatal("a replaced link still works")
	}
	invites.now = func() time.Time { return time.Now().Add(25 * time.Hour) }
	if _, _, err := invites.Peek(ctx, second); err != ErrNoInvite {
		t.Fatal("an expired reset link still works")
	}

	// Sign-in: checked before it's set, then used.
	if err := people.CheckSignIn(ctx, sam.ID, "s", "long enough password"); err != ErrBadUsername {
		t.Fatalf("short username: %v", err)
	}
	if err := people.SetSignIn(ctx, sam.ID, "Sam.W", "long enough password"); err != nil {
		t.Fatal(err)
	}
	alex, _ := people.Create(ctx, "Alex", RoleVisitor)
	if err := people.CheckSignIn(ctx, alex.ID, "sam.w", "long enough password"); err != ErrUsernameTaken {
		t.Fatalf("taken, other case: %v", err)
	}
	if got, err := people.SignIn(ctx, "SAM.W", "long enough password"); err != nil || got.ID != sam.ID || !got.SignIn {
		t.Fatalf("sign in = %+v %v", got, err)
	}
	for _, bad := range [][2]string{{"sam.w", "wrong password!!"}, {"nobody", "long enough password"}} {
		if _, err := people.SignIn(ctx, bad[0], bad[1]); err != ErrSignIn {
			t.Fatalf("%v: %v", bad, err)
		}
	}

	// Roles: the Owner is fixed; a disabled person can't sign in.
	member := RoleMember
	if _, err := people.Update(ctx, OwnerID, Change{Role: &member}); err != ErrOwnerFixed {
		t.Fatalf("demote owner: %v", err)
	}
	yes := true
	if _, err := people.Update(ctx, OwnerID, Change{Disabled: &yes}); err != ErrOwnerFixed {
		t.Fatalf("disable owner: %v", err)
	}
	owner := RoleOwner
	if _, err := people.Update(ctx, sam.ID, Change{Role: &owner}); err != ErrOneOwner {
		t.Fatalf("second owner by update: %v", err)
	}
	if p, err := people.Update(ctx, sam.ID, Change{Disabled: &yes}); err != nil || p.Disabled == nil {
		t.Fatalf("disable = %+v %v", p, err)
	}
	if _, err := people.SignIn(ctx, "sam.w", "long enough password"); err != ErrSignIn {
		t.Fatal("a disabled person signed in")
	}

	// Sessions: found while fresh, gone when expired or signed out.
	sessions := NewSessions(db.SQL)
	cookie, _, err := sessions.Create(ctx, alex.ID, "Safari", "192.168.1.9")
	if err != nil {
		t.Fatal(err)
	}
	if id, err := sessions.Person(ctx, cookie); err != nil || id != alex.ID {
		t.Fatalf("person = %q %v", id, err)
	}
	var stored string
	_ = db.SQL.QueryRow(`SELECT id_hash FROM sessions`).Scan(&stored)
	if stored == cookie || len(stored) != 64 {
		t.Fatal("the cookie itself was stored")
	}
	sessions.now = func() time.Time { return time.Now().Add(SessionTTL + time.Hour) }
	if _, err := sessions.Person(ctx, cookie); err != ErrNoSession {
		t.Fatal("an expired session still works")
	}
	sessions.now = time.Now
	again, _, _ := sessions.Create(ctx, alex.ID, "", "")
	if err := sessions.DeleteFor(ctx, alex.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := sessions.Person(ctx, again); err != ErrNoSession {
		t.Fatal("signed out everywhere, still signed in")
	}
}

// A phone paired with a code gets the key of whoever showed the code.
func TestDeviceKeyIsTheStarters(t *testing.T) {
	var keyFor string
	p := &DevicePairer{CreateKey: func(ctx context.Context, name string) (APIKeyRecord, string, error) {
		keyFor = PersonID(ctx)
		return APIKeyRecord{ID: "k", PersonID: keyFor}, "ygg_secret000", nil
	}}
	code, err := p.StartFor(Person{ID: "sam", Role: RoleMember})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.Pair(context.Background(), code.Code, "Sam's phone", "192.168.1.5"); err != nil {
		t.Fatal(err)
	}
	if keyFor != "sam" {
		t.Fatalf("key for %q", keyFor)
	}
}
