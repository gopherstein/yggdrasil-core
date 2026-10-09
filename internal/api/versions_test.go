package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yeixio/toskar-core/internal/config"
	"github.com/yeixio/toskar-core/internal/turnopts"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

// POST /chat carries where a turn answers from, one way at a time, and
// PUT …/shown switches versions (#447).
func TestChatVersionsRoutes(t *testing.T) {
	mgr, err := config.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var got turnopts.Branch
	srv := NewServer(Dependencies{
		Config: mgr,
		Chat: func(w http.ResponseWriter, r *http.Request, _, _, _, _ string, _ bool, _ string) error {
			got = turnopts.BranchFrom(r.Context())
			w.WriteHeader(http.StatusOK)
			return nil
		},
		ShowVersion: func(_ context.Context, conv, id string) ([]contracts.Message, error) {
			if id != "m1" {
				return nil, errors.New("no such message in this chat")
			}
			return []contracts.Message{{ID: "m1", ConversationID: conv}}, nil
		},
	})
	call := func(method, path, body string) (int, string) {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.RemoteAddr, r.Host = "127.0.0.1:50000", "127.0.0.1:7331"
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, r)
		return rec.Code, rec.Body.String()
	}
	if code, _ := call(http.MethodPost, "/api/v1/chat", `{"conversation_id":"c1","retry_of":"a1"}`); code != http.StatusOK || got.RetryOf != "a1" {
		t.Fatalf("retry: %d %+v", code, got)
	}
	if code, _ := call(http.MethodPost, "/api/v1/chat", `{"conversation_id":"c1","edit_of":"u1","message":"x"}`); code != http.StatusOK || got.EditOf != "u1" {
		t.Fatalf("edit: %d %+v", code, got)
	}
	if code, _ := call(http.MethodPost, "/api/v1/chat", `{"conversation_id":"c1","retry_of":"a1","edit_of":"u1"}`); code != http.StatusBadRequest {
		t.Fatalf("both: %d", code)
	}
	if code, _ := call(http.MethodPost, "/api/v1/chat", `{"retry_of":"a1"}`); code != http.StatusBadRequest {
		t.Fatalf("no conversation: %d", code)
	}
	if code, body := call(http.MethodPut, "/api/v1/conversations/c1/messages/m1/shown", ``); code != http.StatusOK || !strings.Contains(body, `"m1"`) {
		t.Fatalf("show: %d %s", code, body)
	}
	if code, body := call(http.MethodPut, "/api/v1/conversations/c1/messages/nope/shown", ``); code != http.StatusNotFound || !strings.Contains(body, "MESSAGE_NOT_FOUND") {
		t.Fatalf("show missing: %d %s", code, body)
	}
}
