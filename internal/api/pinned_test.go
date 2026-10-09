package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/config"
	"github.com/yeixio/toskar-core/internal/store"
	"github.com/yeixio/toskar-core/internal/turnopts"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

// A key pinned to a profile chats and runs tasks with it alone on the
// control API too (#345).
func TestPinnedKeyChats(t *testing.T) {
	mgr, err := config.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var gotProfile string
	var fixed bool
	srv := NewServer(Dependencies{
		Config: mgr,
		VerifyAPIKey: func(_ context.Context, secret string) (auth.APIKeyRecord, error) {
			rec := auth.APIKeyRecord{ID: "k1", PersonID: auth.OwnerID, Permissions: auth.DefaultAPIKeyPermissions()}
			if secret == "ygg_key_pinned" {
				rec.Permissions.Profile = "tires"
			}
			return rec, nil
		},
		Chat: func(w http.ResponseWriter, r *http.Request, _, profileID, _, _ string, _ bool, _ string) error {
			gotProfile = profileID
			o := turnopts.From(r.Context())
			fixed = o != nil && o.FixedProfile
			w.WriteHeader(http.StatusOK)
			return nil
		},
	})
	chat := func(key, body string) int {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/chat", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+key)
		r.RemoteAddr = "127.0.0.1:50000"
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, r)
		return rec.Code
	}
	if code := chat("ygg_key_pinned", `{"message":"hi"}`); code != http.StatusOK || gotProfile != "tires" || !fixed {
		t.Fatalf("pinned: %d %q %v", code, gotProfile, fixed)
	}
	if code := chat("ygg_key_pinned", `{"message":"hi","profile_id":"tires","model_id":"auto"}`); code != http.StatusOK || gotProfile != "tires" {
		t.Fatalf("naming its profile: %d %q", code, gotProfile)
	}
	for _, body := range []string{`{"message":"hi","profile_id":"general"}`, `{"message":"hi","model_id":"qwen3-8b"}`} {
		if code := chat("ygg_key_pinned", body); code != http.StatusForbidden {
			t.Errorf("%s: %d", body, code)
		}
	}
	if code := chat("ygg_key_free", `{"message":"hi","profile_id":"general"}`); code != http.StatusOK || gotProfile != "general" || fixed {
		t.Fatalf("unpinned: %d %q %v", code, gotProfile, fixed)
	}
}

// Admins pin roles and people to profiles; /me says what a pinned person
// may use (#345).
func TestProfilePinRoutes(t *testing.T) {
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
	people := auth.NewPeople(db.SQL)
	sam, _ := people.Create(context.Background(), "Sam", auth.RoleMember)
	srv := NewServer(Dependencies{
		Config: mgr,
		People: people,
		GetProfile: func(_ context.Context, id string) (contracts.AIProfile, error) {
			if id == "tires" || id == "wheels" {
				return contracts.AIProfile{ID: id}, nil
			}
			return contracts.AIProfile{}, errors.New("not found")
		},
		VerifyAPIKey: func(_ context.Context, _ string) (auth.APIKeyRecord, error) {
			return auth.APIKeyRecord{ID: "k", PersonID: sam.ID, Permissions: auth.DefaultAPIKeyPermissions()}, nil
		},
	})
	call := func(method, path, body, key string) (int, string) {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.RemoteAddr, r.Host = "127.0.0.1:50000", "127.0.0.1:7331"
		if key != "" {
			r.Header.Set("Authorization", "Bearer "+key)
		}
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, r)
		return rec.Code, rec.Body.String()
	}
	if code, body := call(http.MethodPut, "/api/v1/profile-pins/member", `{"profiles":["nope"]}`, ""); code != http.StatusBadRequest || !strings.Contains(body, "PIN_INVALID") {
		t.Fatalf("missing profile: %d %s", code, body)
	}
	if code, body := call(http.MethodPut, "/api/v1/profile-pins/admin", `{"profiles":["tires"]}`, ""); code != http.StatusBadRequest {
		t.Fatalf("admin role: %d %s", code, body)
	}
	if code, body := call(http.MethodPut, "/api/v1/profile-pins/member", `{"profiles":["tires","wheels"]}`, ""); code != http.StatusOK || !strings.Contains(body, `"member":["tires","wheels"]`) {
		t.Fatalf("role pin: %d %s", code, body)
	}
	if _, body := call(http.MethodGet, "/api/v1/me", "", "ygg_key_sam"); !strings.Contains(body, `"profiles":["tires","wheels"]`) {
		t.Fatalf("me: %s", body)
	}
	if code, body := call(http.MethodPut, "/api/v1/people/"+sam.ID+"/profiles", `{"profiles":["wheels"]}`, ""); code != http.StatusOK || !strings.Contains(body, `"profiles":["wheels"]`) {
		t.Fatalf("person pin: %d %s", code, body)
	}
	if _, body := call(http.MethodGet, "/api/v1/me", "", "ygg_key_sam"); !strings.Contains(body, `"profiles":["wheels"]`) {
		t.Fatalf("me, own pin: %s", body)
	}
	// A Member can't pin anyone.
	if code, _ := call(http.MethodPut, "/api/v1/people/"+sam.ID+"/profiles", `{"profiles":null}`, "ygg_key_sam"); code != http.StatusForbidden {
		t.Fatalf("member pinning: %d", code)
	}
	if code, body := call(http.MethodPut, "/api/v1/people/"+sam.ID+"/profiles", `{"profiles":null}`, ""); code != http.StatusOK || strings.Contains(body, `"profiles"`) {
		t.Fatalf("back to role: %d %s", code, body)
	}
}
