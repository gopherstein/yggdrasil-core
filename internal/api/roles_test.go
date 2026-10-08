package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/config"
	"github.com/yeixio/toskar-core/internal/store"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

// Every route given a role is one the daemon serves, so a typo can't leave
// a route at Admin by mistake.
func TestRoleRoutesAreServed(t *testing.T) {
	srv := NewServer(Dependencies{})
	served := map[string]bool{}
	_ = srv.router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		path, err := route.GetPathTemplate()
		if err != nil {
			return nil
		}
		methods, _ := route.GetMethods()
		for _, m := range methods {
			served[m+" "+path] = true
		}
		return nil
	})
	for _, r := range auth.RoleRoutes() {
		if !served[r] {
			t.Errorf("%s is given a role but isn't served", r)
		}
	}
}

// Each role reaches what it may (#203): Visitors chat, Members use Toskar
// for themselves, Admins run it, and only the Owner erases it.
func TestRolesReachWhatTheyMay(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.SQL.Exec(`INSERT INTO people (id, name, role) VALUES
		('vee', 'Vee', 'visitor'), ('sam', 'Sam', 'member'), ('ada', 'Ada', 'admin')`); err != nil {
		t.Fatal(err)
	}
	mgr, err := config.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Update(func(c *config.Config) { c.APIHost = "0.0.0.0" }); err != nil {
		t.Fatal(err)
	}
	srv := NewServer(Dependencies{
		Config: mgr,
		People: auth.NewPeople(db.SQL),
		VerifyAPIKey: func(_ context.Context, secret string) (auth.APIKeyRecord, error) {
			id, ok := strings.CutPrefix(secret, "ygg_key_")
			if !ok {
				return auth.APIKeyRecord{}, auth.ErrNoPerson
			}
			return auth.APIKeyRecord{ID: "k-" + id, PersonID: id}, nil
		},
	})
	call := func(person, method, path string) (int, string) {
		r := httptest.NewRequest(method, path, strings.NewReader("{}"))
		r.Header.Set("Authorization", "Bearer ygg_key_"+person)
		r.Header.Set("X-Forwarded-For", "192.168.1.20") // from another device
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, r)
		var body struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.NewDecoder(rec.Body).Decode(&body)
		return rec.Code, body.Error.Code
	}
	cases := []struct {
		person, method, path string
		allowed              bool
	}{
		{"vee", http.MethodGet, "/api/v1/conversations", true},
		{"vee", http.MethodPost, "/api/v1/chat", true},
		{"vee", http.MethodGet, "/api/v1/memory", false},
		{"vee", http.MethodGet, "/api/v1/automations", false},
		{"sam", http.MethodGet, "/api/v1/memory", true},
		{"sam", http.MethodPost, "/api/v1/automations", true},
		{"sam", http.MethodGet, "/api/v1/knowledge/sources", true},
		{"sam", http.MethodPost, "/api/v1/knowledge/sources", false},
		{"sam", http.MethodPatch, "/api/v1/settings", false},
		{"sam", http.MethodPost, "/api/v1/models/llama/install", false},
		{"sam", http.MethodGet, "/api/v1/api-keys", false},
		{"sam", http.MethodGet, "/api/v1/notifications", true},
		{"vee", http.MethodGet, "/api/v1/notifications", false},
		{"sam", http.MethodGet, "/api/v1/notifications/destinations", false},
		{"sam", http.MethodGet, "/api/v1/people", false},
		{"ada", http.MethodPatch, "/api/v1/settings", true},
		{"ada", http.MethodGet, "/api/v1/api-keys", true},
		{"ada", http.MethodPost, "/api/v1/settings/reset", false},
		{"owner", http.MethodPost, "/api/v1/settings/reset", true},
	}
	for _, c := range cases {
		code, errCode := call(c.person, c.method, c.path)
		refused := code == http.StatusForbidden && errCode == "ROLE_REQUIRED"
		if refused == c.allowed {
			t.Errorf("%s %s %s: %d %s, allowed %v", c.person, c.method, c.path, code, errCode, c.allowed)
		}
	}
}

// Anyone saves their own languages, and nothing else, there (#206).
func TestMyPreferences(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.SQL.Exec(`INSERT INTO people (id, name, role) VALUES ('vee', 'Vee', 'visitor')`); err != nil {
		t.Fatal(err)
	}
	mgr, err := config.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Update(func(c *config.Config) { c.APIHost = "0.0.0.0" }); err != nil {
		t.Fatal(err)
	}
	var saved map[string]any
	var as string
	srv := NewServer(Dependencies{
		Config: mgr,
		People: auth.NewPeople(db.SQL),
		VerifyAPIKey: func(_ context.Context, secret string) (auth.APIKeyRecord, error) {
			return auth.APIKeyRecord{ID: "k", PersonID: "vee"}, nil
		},
		UpdateSettings: func(ctx context.Context, patch map[string]any) (contracts.SettingsView, error) {
			saved, as = patch, auth.PersonID(ctx)
			return contracts.SettingsView{}, nil
		},
	})
	patch := func(body string) int {
		r := httptest.NewRequest(http.MethodPatch, "/api/v1/me/preferences", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer ygg_key_vee")
		r.Header.Set("X-Forwarded-For", "192.168.1.20")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, r)
		return rec.Code
	}
	if code := patch(`{"ui_locale":"fr","assistant_language_mode":"app"}`); code != http.StatusOK || saved["ui_locale"] != "fr" || as != "vee" {
		t.Fatalf("own languages: %d %v as %q", code, saved, as)
	}
	saved = nil
	if code := patch(`{"ui_locale":"fr","lan_api_enabled":true}`); code != http.StatusBadRequest || saved != nil {
		t.Fatalf("another setting: %d %v", code, saved)
	}
}
