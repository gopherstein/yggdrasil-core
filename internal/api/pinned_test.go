package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/config"
	"github.com/yeixio/toskar-core/internal/turnopts"
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
