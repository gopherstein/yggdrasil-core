package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/yeixio/toskar-core/internal/config"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

// POST /conversations/delete takes 1 to 1,000 distinct ids (#452) and
// answers with what was deleted and skipped.
func TestDeleteConversationsRoute(t *testing.T) {
	mgr, err := config.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	srv := NewServer(Dependencies{
		Config: mgr,
		DeleteConversations: func(_ context.Context, ids []string) (contracts.ConversationsDeleted, error) {
			got = ids
			return contracts.ConversationsDeleted{Deleted: ids[:1], Skipped: []contracts.ConversationSkipped{{ID: ids[1], Reason: "not_found"}}}, nil
		},
	})
	call := func(body string) (int, string) {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/delete", strings.NewReader(body))
		r.RemoteAddr, r.Host = "127.0.0.1:50000", "127.0.0.1:7331"
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, r)
		return rec.Code, rec.Body.String()
	}
	code, body := call(`{"ids":["c1"," c2 ","c1",""]}`)
	if code != http.StatusOK || !slices.Equal(got, []string{"c1", "c2"}) {
		t.Fatalf("delete: %d %s, ids %v", code, body, got)
	}
	if !strings.Contains(body, `"deleted":["c1"]`) || !strings.Contains(body, `{"id":"c2","reason":"not_found"}`) {
		t.Errorf("body = %s", body)
	}
	many := make([]string, maxBulkDelete+1)
	for i := range many {
		many[i] = fmt.Sprintf("%q", fmt.Sprint("c", i))
	}
	for _, bad := range []string{`{"ids":[]}`, `{"ids":["  "]}`, `{}`, `{"ids":[` + strings.Join(many, ",") + `]}`} {
		if code, body := call(bad); code != http.StatusBadRequest || !strings.Contains(body, "INVALID_CONVERSATION_IDS") {
			t.Errorf("%.40s: %d %s", bad, code, body)
		}
	}
	if code, _ := call(`not json`); code != http.StatusBadRequest {
		t.Errorf("not json: %d", code)
	}
}
