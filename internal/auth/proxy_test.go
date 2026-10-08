package auth_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/store"
)

func TestProxySettings(t *testing.T) {
	if p, err := auth.NewProxy(auth.ProxySettings{}); p != nil || err != nil {
		t.Fatalf("no proxies: %v %v", p, err)
	}
	if _, err := auth.NewProxy(auth.ProxySettings{Trusted: []string{"proxy.lan"}}); err == nil {
		t.Fatal("a host name was taken as an address")
	}
	if _, err := auth.NewProxy(auth.ProxySettings{Trusted: []string{"10.0.0.2"}, DefaultRole: "owner"}); err == nil {
		t.Fatal("owner was taken as a default role")
	}
	p, err := auth.NewProxy(auth.ProxySettings{Trusted: []string{"10.0.0.2", "172.18.0.0/16"}})
	if err != nil {
		t.Fatal(err)
	}
	from := func(remote, forwarded string) bool {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = remote
		if forwarded != "" {
			r.Header.Set("X-Forwarded-For", forwarded)
		}
		return p.FromProxy(r)
	}
	if !from("10.0.0.2:5000", "") || !from("172.18.4.9:5000", "") || !from("[::ffff:10.0.0.2]:5000", "") {
		t.Fatal("a trusted proxy wasn't recognized")
	}
	// Trust is by the connection, never by an address a client writes.
	if from("10.0.0.3:5000", "10.0.0.2") {
		t.Fatal("a forwarded address was trusted")
	}
}

func TestProxyIdentityAndRoles(t *testing.T) {
	p, err := auth.NewProxy(auth.ProxySettings{
		Trusted: []string{"10.0.0.2"}, AdminGroups: []string{"toskar-admins"}, MemberGroups: []string{"family"},
		DefaultRole: "none", OwnerUser: "mike@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Remote-User", "sam@example.com")
	r.Header.Set("Remote-Groups", "family|friends")
	id, ok := p.Identity(r)
	if !ok || id.User != "sam@example.com" || len(id.Groups) != 2 {
		t.Fatalf("identity %+v %v", id, ok)
	}
	if role, ok := p.Role(id); !ok || role != auth.RoleMember {
		t.Fatalf("family: %v %v", role, ok)
	}
	if role, _ := p.Role(auth.ProxyIdentity{Groups: []string{"family", "toskar-admins"}}); role != auth.RoleAdmin {
		t.Fatalf("admins: %v", role)
	}
	if _, ok := p.Role(auth.ProxyIdentity{Groups: []string{"friends"}}); ok {
		t.Fatal("someone in no group got in with default_role none")
	}
	if !p.IsOwner(auth.ProxyIdentity{User: "Mike@Example.com"}) {
		t.Fatal("the owner wasn't recognized")
	}
	if _, ok := p.Identity(httptest.NewRequest("GET", "/", nil)); ok {
		t.Fatal("a request naming nobody had an identity")
	}
}

func TestProxiedPeople(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	people := auth.NewPeople(db.SQL)
	p, err := auth.NewProxy(auth.ProxySettings{
		Trusted: []string{"10.0.0.2"}, AdminGroups: []string{"admins"}, DefaultRole: "visitor", OwnerUser: "mike",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	sam, err := people.Proxied(ctx, p, auth.ProxyIdentity{User: "sam@example.com"})
	if err != nil || sam.Role != auth.RoleVisitor || sam.Name != "sam" {
		t.Fatalf("first visit: %+v %v", sam, err)
	}
	again, err := people.Proxied(ctx, p, auth.ProxyIdentity{User: "sam@example.com", Groups: []string{"admins"}})
	if err != nil || again.ID != sam.ID || again.Role != auth.RoleAdmin {
		t.Fatalf("joined admins: %+v %v", again, err)
	}
	if owner, err := people.Proxied(ctx, p, auth.ProxyIdentity{User: "mike"}); err != nil || owner.ID != auth.OwnerID {
		t.Fatalf("owner: %+v %v", owner, err)
	}
	disabled := true
	if _, err := people.Update(ctx, sam.ID, auth.Change{Disabled: &disabled}); err != nil {
		t.Fatal(err)
	}
	if _, err := people.Proxied(ctx, p, auth.ProxyIdentity{User: "sam@example.com"}); !errors.Is(err, auth.ErrNoPerson) {
		t.Fatalf("disabled: %v", err)
	}
}
