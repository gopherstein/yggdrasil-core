package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/config"
	"github.com/yeixio/toskar-core/internal/store"
)

// Signing in end to end (#206): the Owner adds Sam and gets a link; Sam
// opens it on another device, chooses a username and password, and is
// signed in; what Sam may do follows the role; signing out and being
// disabled end the session; too many wrong passwords wait.
func TestSignInEndToEnd(t *testing.T) {
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
	if err := mgr.Update(func(c *config.Config) { c.APIHost = "0.0.0.0" }); err != nil {
		t.Fatal(err)
	}
	people := auth.NewPeople(db.SQL)
	keys := auth.NewAPIKeyManager(db.SQL, auth.NewSecretStore(dir))
	srv := NewServer(Dependencies{Config: mgr, People: people, Sessions: auth.NewSessions(db.SQL), Invites: auth.NewInvites(db.SQL),
		VerifyAPIKey: keys.Verify})

	type reply struct {
		code   int
		body   map[string]any
		cookie *http.Cookie
	}
	send := func(r *http.Request) reply {
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, r)
		out := reply{code: rec.Code, body: map[string]any{}}
		_ = json.Unmarshal(rec.Body.Bytes(), &out.body)
		for _, c := range rec.Result().Cookies() {
			if c.Name == auth.SessionCookie {
				out.cookie = c
			}
		}
		return out
	}
	// A browser on another device, at the computer's network name.
	device := func(method, path, body string, cookie *http.Cookie, origin bool) *http.Request {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Host = "toskar.local:7331"
		r.RemoteAddr = "192.168.1.30:51000"
		if cookie != nil {
			r.AddCookie(cookie)
		}
		if origin {
			r.Header.Set("Origin", "http://toskar.local:7331")
		}
		return r
	}
	errCode := func(r reply) string {
		e, _ := r.body["error"].(map[string]any)
		c, _ := e["code"].(string)
		return c
	}

	// The Owner, at the computer, adds Sam.
	added := send(localRequest(http.MethodPost, "/api/v1/people", strings.NewReader(`{"name":"Sam","role":"member"}`)))
	if added.code != http.StatusCreated {
		t.Fatalf("add: %d %v", added.code, added.body)
	}
	path := added.body["link"].(map[string]any)["path"].(string)
	token := strings.TrimPrefix(path, "/invite/")
	if !strings.HasPrefix(path, "/invite/") || len(token) < 40 {
		t.Fatalf("link = %q", path)
	}

	// Sam opens it: who it's for, then a choice that's fixed and tried again.
	if r := send(device(http.MethodGet, "/api/v1/invites/"+token, "", nil, false)); r.code != http.StatusOK || r.body["name"] != "Sam" || r.body["kind"] != "invite" {
		t.Fatalf("peek: %d %v", r.code, r.body)
	}
	if r := send(device(http.MethodPost, "/api/v1/invites/"+token, `{"username":"sam","password":"short"}`, nil, true)); errCode(r) != "PASSWORD_TOO_SHORT" {
		t.Fatalf("short password: %d %v", r.code, r.body)
	}
	accepted := send(device(http.MethodPost, "/api/v1/invites/"+token, `{"username":"sam","password":"a long enough password"}`, nil, true))
	if accepted.code != http.StatusOK || accepted.cookie == nil || !accepted.cookie.HttpOnly || accepted.cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("accept: %d %v %+v", accepted.code, accepted.body, accepted.cookie)
	}
	if r := send(device(http.MethodPost, "/api/v1/invites/"+token, `{"username":"sam2","password":"a long enough password"}`, nil, true)); errCode(r) != "INVITE_INVALID" {
		t.Fatalf("link used twice: %d %v", r.code, r.body)
	}

	// Signed in as Sam, a Member.
	me := send(device(http.MethodGet, "/api/v1/me", "", accepted.cookie, false))
	person, _ := me.body["person"].(map[string]any)
	if me.code != http.StatusOK || me.body["via"] != auth.ViaSession || person["name"] != "Sam" || person["role"] != "member" {
		t.Fatalf("me: %d %v", me.code, me.body)
	}
	if r := send(device(http.MethodGet, "/api/v1/people", "", accepted.cookie, false)); r.code != http.StatusForbidden || errCode(r) != "ROLE_REQUIRED" {
		t.Fatalf("member lists people: %d %v", r.code, r.body)
	}
	// A signed-in change has to come from Toskar's own pages.
	if r := send(device(http.MethodDelete, "/api/v1/session", "", accepted.cookie, false)); errCode(r) != "CROSS_SITE" {
		t.Fatalf("change without origin: %d %v", r.code, r.body)
	}
	if r := send(device(http.MethodDelete, "/api/v1/session", "", accepted.cookie, true)); r.code != http.StatusNoContent {
		t.Fatalf("sign out: %d", r.code)
	}
	if r := send(device(http.MethodGet, "/api/v1/me", "", accepted.cookie, false)); r.code != http.StatusUnauthorized {
		t.Fatalf("signed out, still in: %d", r.code)
	}

	// Signing in again, then being disabled, ends it.
	signedIn := send(device(http.MethodPost, "/api/v1/session", `{"username":"SAM","password":"a long enough password"}`, nil, true))
	if signedIn.code != http.StatusOK || signedIn.cookie == nil {
		t.Fatalf("sign in: %d %v", signedIn.code, signedIn.body)
	}
	samID := person["id"].(string)
	if r := send(localRequest(http.MethodPatch, "/api/v1/people/"+samID, strings.NewReader(`{"disabled":true}`))); r.code != http.StatusOK {
		t.Fatalf("disable: %d %v", r.code, r.body)
	}
	if r := send(device(http.MethodGet, "/api/v1/me", "", signedIn.cookie, false)); r.code != http.StatusUnauthorized {
		t.Fatalf("disabled, still in: %d", r.code)
	}

	// An Admin adds Members, not Admins, and can't touch the Owner or
	// their own role.
	ada, _ := people.Create(context.Background(), "Ada", auth.RoleAdmin)
	_, adaKey, err := keys.Create(auth.AsPerson(context.Background(), ada), "ada's laptop")
	if err != nil {
		t.Fatal(err)
	}
	asAda := func(method, path, body string) reply {
		r := device(method, path, body, nil, false)
		r.Header.Set("Authorization", "Bearer "+adaKey)
		return send(r)
	}
	if r := asAda(http.MethodPost, "/api/v1/people", `{"name":"Bo","role":"admin"}`); r.code != http.StatusForbidden {
		t.Fatalf("admin adds admin: %d", r.code)
	}
	if r := asAda(http.MethodPost, "/api/v1/people", `{"name":"Bo","role":"visitor"}`); r.code != http.StatusCreated {
		t.Fatalf("admin adds visitor: %d %v", r.code, r.body)
	}
	if r := asAda(http.MethodPatch, "/api/v1/people/"+auth.OwnerID, `{"name":"Not the owner"}`); r.code != http.StatusForbidden {
		t.Fatalf("admin renames owner: %d", r.code)
	}
	if r := asAda(http.MethodPatch, "/api/v1/people/"+ada.ID, `{"role":"member"}`); errCode(r) != "OWNER_FIXED" {
		t.Fatalf("admin demotes self: %d %v", r.code, r.body)
	}

	// Too many wrong passwords: the username waits, even with the right one.
	_, _ = people.Update(context.Background(), samID, auth.Change{Disabled: new(bool)})
	for i := 0; i < 5; i++ {
		send(device(http.MethodPost, "/api/v1/session", `{"username":"sam","password":"not the password"}`, nil, true))
	}
	if r := send(device(http.MethodPost, "/api/v1/session", `{"username":"sam","password":"a long enough password"}`, nil, true)); r.code != http.StatusTooManyRequests {
		t.Fatalf("after five misses: %d %v", r.code, r.body)
	}
}
