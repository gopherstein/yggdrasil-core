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
	"github.com/yeixio/toskar-core/internal/portals"
	"github.com/yeixio/toskar-core/internal/store"
	"github.com/yeixio/toskar-core/internal/turnopts"
)

// A visitor enters a portal with its passcode and chats there, with the
// portal's settings, and reaches nothing else; the main app in the same
// browser is untouched, and turning the portal off stops them (#205).
func TestPortalGuests(t *testing.T) {
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
	store := portals.NewStore(db.SQL)
	code, name := "tide-pool", "Help desk"
	slug, tools, lang := "support", portals.ToolsReadOnly, "es"
	portal, err := store.Create(context.Background(), portals.Input{Slug: &slug, Name: &name, Passcode: &code, Tools: &tools, Language: &lang})
	if err != nil {
		t.Fatal(err)
	}
	type chatCall struct {
		person, profile, model string
		opts                   *turnopts.Options
	}
	var calls []chatCall
	srv := NewServer(Dependencies{
		Config: mgr, People: auth.NewPeople(db.SQL), Sessions: auth.NewSessions(db.SQL), Invites: auth.NewInvites(db.SQL), Portals: store,
		Chat: func(w http.ResponseWriter, r *http.Request, conversationID, profileID, modelID, message string, stream bool, execution string) error {
			calls = append(calls, chatCall{auth.PersonID(r.Context()), profileID, modelID, turnopts.From(r.Context())})
			writeJSON(w, http.StatusOK, map[string]string{"ok": "yes"})
			return nil
		},
	})
	do := func(method, path, body string, headers map[string]string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.RemoteAddr = "192.168.1.40:5000"
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		for _, c := range cookies {
			r.AddCookie(c)
		}
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, r)
		return rec
	}
	portalHeaders := map[string]string{"X-Toskar-Portal": "support", "Origin": "http://example.com"}

	page := do(http.MethodGet, "/api/v1/portals/support/page", "", nil)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `"name":"Help desk"`) || strings.Contains(page.Body.String(), "argon2") {
		t.Fatalf("page: %d %s", page.Code, page.Body)
	}
	if rec := do(http.MethodPost, "/api/v1/portals/support/enter", `{"passcode":"wrong"}`, portalHeaders); rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong passcode: %d", rec.Code)
	}
	enter := do(http.MethodPost, "/api/v1/portals/support/enter", `{"passcode":"tide-pool"}`, portalHeaders)
	if enter.Code != http.StatusOK {
		t.Fatalf("enter: %d %s", enter.Code, enter.Body)
	}
	var guestCookie *http.Cookie
	for _, c := range enter.Result().Cookies() {
		if c.Name == "toskar_portal_support" {
			guestCookie = c
		}
	}
	if guestCookie == nil || !guestCookie.HttpOnly {
		t.Fatalf("guest cookie %+v", guestCookie)
	}
	var guest auth.Principal
	_ = json.NewDecoder(enter.Body).Decode(&guest)
	if guest.Person.PortalID != portal.ID {
		t.Fatalf("guest %+v", guest)
	}

	// Chatting takes the portal's profile, tools, and language, whatever
	// the request asks for.
	if rec := do(http.MethodPost, "/api/v1/chat", `{"message":"hola","profile_id":"programming","model_id":"big-model"}`, portalHeaders, guestCookie); rec.Code != http.StatusOK {
		t.Fatalf("chat: %d %s", rec.Code, rec.Body)
	}
	if len(calls) != 1 || calls[0].person != guest.Person.ID || calls[0].profile != "" || calls[0].model != "" || calls[0].opts == nil ||
		!calls[0].opts.FixedProfile || !calls[0].opts.ReadOnlyTools || calls[0].opts.Memory || calls[0].opts.Language != "es" {
		t.Fatalf("chat call %+v %+v", calls, calls[0].opts)
	}
	if rec := do(http.MethodGet, "/api/v1/conversations", "", portalHeaders, guestCookie); rec.Code == http.StatusForbidden || rec.Code == http.StatusUnauthorized {
		t.Fatalf("conversations: %d", rec.Code)
	}
	for _, path := range []string{"/api/v1/settings", "/api/v1/models", "/api/v1/memory", "/api/v1/people"} {
		if rec := do(http.MethodGet, path, "", portalHeaders, guestCookie); rec.Code != http.StatusForbidden {
			t.Errorf("guest reached %s: %d", path, rec.Code)
		}
	}
	// The same session as an ordinary sign-in is still the portal's guest.
	stolen := &http.Cookie{Name: auth.SessionCookie, Value: guestCookie.Value}
	if rec := do(http.MethodGet, "/api/v1/settings", "", nil, stolen); rec.Code != http.StatusForbidden {
		t.Fatalf("the guest's session as a sign-in: %d", rec.Code)
	}
	// Without the portal's header, the guest cookie is nobody.
	if rec := do(http.MethodGet, "/api/v1/conversations", "", nil, guestCookie); rec.Code != http.StatusUnauthorized {
		t.Fatalf("guest cookie without the header: %d", rec.Code)
	}
	// Entering again is the same guest.
	again := do(http.MethodPost, "/api/v1/portals/support/enter", `{}`, portalHeaders, guestCookie)
	var same auth.Principal
	_ = json.NewDecoder(again.Body).Decode(&same)
	if same.Person.ID != guest.Person.ID {
		t.Fatalf("entered again as %+v", same)
	}

	off := false
	if _, err := store.Update(context.Background(), portal.ID, portals.Input{Enabled: &off}); err != nil {
		t.Fatal(err)
	}
	if rec := do(http.MethodPost, "/api/v1/chat", `{"message":"still there?"}`, portalHeaders, guestCookie); rec.Code != http.StatusForbidden || len(calls) != 1 {
		t.Fatalf("chat after turning it off: %d, %d calls", rec.Code, len(calls))
	}
	if rec := do(http.MethodGet, "/api/v1/portals/support/page", "", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("page after turning it off: %d", rec.Code)
	}
}
