package api

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/config"
	"github.com/yeixio/toskar-core/internal/events"
	"github.com/yeixio/toskar-core/internal/store"
)

// Every request says who it's from (#206): this computer's are the
// Owner's, a key's are its person's, and a disabled person's key stops
// working.
func TestRequestsCarryTheirPerson(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.SQL.Exec(`INSERT INTO people (id, name, role) VALUES ('sam', 'Sam', 'member')`); err != nil {
		t.Fatal(err)
	}
	mgr, err := config.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Update(func(c *config.Config) { c.APIHost = "0.0.0.0" }); err != nil {
		t.Fatal(err)
	}
	keys := map[string]auth.APIKeyRecord{
		"ygg_ownerkey00": {ID: "k1", PersonID: auth.OwnerID},
		"ygg_samkey0000": {ID: "k2", PersonID: "sam"},
	}
	srv := NewServer(Dependencies{
		Config: mgr,
		People: auth.NewPeople(db.SQL),
		VerifyAPIKey: func(_ context.Context, secret string) (auth.APIKeyRecord, error) {
			if rec, ok := keys[secret]; ok {
				return rec, nil
			}
			return auth.APIKeyRecord{}, auth.ErrNoPerson
		},
	})
	me := func(r *http.Request) (int, auth.Principal) {
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, r)
		var p auth.Principal
		_ = json.NewDecoder(rec.Body).Decode(&p)
		return rec.Code, p
	}

	if code, p := me(localRequest(http.MethodGet, "/api/v1/me", nil)); code != http.StatusOK || p.Person.ID != auth.OwnerID || p.Via != auth.ViaThisComputer || p.Person.Role != auth.RoleOwner {
		t.Fatalf("this computer: %d %+v", code, p)
	}
	keyed := func(secret string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		r.Header.Set("Authorization", "Bearer "+secret)
		return r
	}
	if code, p := me(keyed("ygg_ownerkey00")); code != http.StatusOK || p.Person.ID != auth.OwnerID || p.Via != auth.ViaAPIKey || p.KeyID != "k1" {
		t.Fatalf("owner's key: %d %+v", code, p)
	}
	if code, p := me(keyed("ygg_samkey0000")); code != http.StatusOK || p.Person.ID != "sam" || p.Person.Role != auth.RoleMember || p.Person.Name != "Sam" {
		t.Fatalf("sam's key: %d %+v", code, p)
	}
	if _, err := db.SQL.Exec(`UPDATE people SET disabled_at = '2026-10-08T00:00:00Z' WHERE id = 'sam'`); err != nil {
		t.Fatal(err)
	}
	if code, _ := me(keyed("ygg_samkey0000")); code != http.StatusUnauthorized {
		t.Fatalf("disabled person's key: %d", code)
	}
}

// The live events say only the listener's own chats and tasks, with the
// computers' and models' events everyone sees (#206).
func TestEventsReachOnlyTheirPerson(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.SQL.Exec(`INSERT INTO people (id, name, role) VALUES ('sam', 'Sam', 'member')`); err != nil {
		t.Fatal(err)
	}
	mgr, err := config.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Update(func(c *config.Config) { c.APIHost = "0.0.0.0" }); err != nil {
		t.Fatal(err)
	}
	bus := events.NewBus(16)
	srv := NewServer(Dependencies{
		Config: mgr,
		Bus:    bus,
		People: auth.NewPeople(db.SQL),
		VerifyAPIKey: func(_ context.Context, secret string) (auth.APIKeyRecord, error) {
			if secret == "ygg_samkey0000" {
				return auth.APIKeyRecord{ID: "k2", PersonID: "sam"}, nil
			}
			return auth.APIKeyRecord{}, auth.ErrNoPerson
		},
	})
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer ygg_samkey0000")
	req.Header.Set("X-Forwarded-For", "192.168.1.20") // from another device
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("events: %d", resp.StatusCode)
	}

	bus.Publish(events.New(events.ChatToken, map[string]any{"content": "the owner's secret"}).For(auth.OwnerID))
	bus.Publish(events.New(events.ToolRequested, map[string]any{"tool_id": "untagged"}))
	bus.Publish(events.New(events.ModelLoadCompleted, map[string]any{"model_id": "llama"}))
	bus.Publish(events.New(events.ChatToken, map[string]any{"content": "sam's words"}).For("sam"))
	bus.Publish(events.New(events.ChatComplete, map[string]any{"end": true}).For("sam"))

	var got []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			got = append(got, line)
			if strings.Contains(line, `"end":true`) {
				break
			}
		}
	}
	all := strings.Join(got, "\n")
	if len(got) != 3 || !strings.Contains(all, "llama") || !strings.Contains(all, "sam's words") {
		t.Fatalf("sam heard:\n%s", all)
	}
	if strings.Contains(all, "secret") || strings.Contains(all, "untagged") {
		t.Fatalf("sam heard someone else's:\n%s", all)
	}
}

// A trusted proxy names the person; its header means nothing from
// anywhere else, and a request through a proxy on this computer is never
// the Owner's by default (#206).
func TestProxySignIn(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	mgr, err := config.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Listening only here, behind a proxy on this computer.
	if err := mgr.Update(func(c *config.Config) {
		c.APIHost = "127.0.0.1"
		c.TrustedProxies = []string{"127.0.0.1"}
		c.ProxyAuth = config.ProxyAuth{MemberGroups: []string{"family"}, DefaultRole: "none"}
	}); err != nil {
		t.Fatal(err)
	}
	srv := NewServer(Dependencies{Config: mgr, People: auth.NewPeople(db.SQL)})
	call := func(method, path, remote string, headers map[string]string) (int, auth.Principal) {
		r := httptest.NewRequest(method, path, nil)
		r.RemoteAddr = remote
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, r)
		var p auth.Principal
		_ = json.NewDecoder(rec.Body).Decode(&p)
		return rec.Code, p
	}
	fromProxy := "127.0.0.1:40000"
	if code, p := call(http.MethodGet, "/api/v1/me", fromProxy, map[string]string{"Remote-User": "sam@example.com", "Remote-Groups": "family"}); code != http.StatusOK || p.Via != auth.ViaProxy || p.Person.Role != auth.RoleMember || p.Person.Name != "sam" {
		t.Fatalf("sam through the proxy: %d %+v", code, p)
	}
	if code, _ := call(http.MethodGet, "/api/v1/me", fromProxy, map[string]string{"Remote-User": "eve@example.com"}); code != http.StatusForbidden {
		t.Fatalf("someone in no group: %d", code)
	}
	if code, _ := call(http.MethodGet, "/api/v1/me", fromProxy, nil); code != http.StatusUnauthorized {
		t.Fatalf("through the proxy naming nobody: %d", code)
	}
	// From the proxied page itself, renamed by the proxy: a Member's
	// change is still refused, by role.
	if code, _ := call(http.MethodPatch, "/api/v1/settings", fromProxy, map[string]string{"Remote-User": "sam@example.com", "Remote-Groups": "family",
		"Origin": "https://toskar.example.com", "X-Forwarded-Host": "toskar.example.com"}); code != http.StatusForbidden {
		t.Fatalf("a member changing settings through the proxy: %d", code)
	}

	// With the API on the network, the header from anyone but the proxy
	// names nobody.
	if err := mgr.Update(func(c *config.Config) { c.APIHost = "0.0.0.0"; c.TrustedProxies = []string{"10.0.0.2"} }); err != nil {
		t.Fatal(err)
	}
	if code, _ := call(http.MethodGet, "/api/v1/me", "192.168.1.30:5000", map[string]string{"Remote-User": "mike", "X-Forwarded-For": "10.0.0.2"}); code != http.StatusUnauthorized {
		t.Fatalf("a header from outside the proxy: %d", code)
	}
}
