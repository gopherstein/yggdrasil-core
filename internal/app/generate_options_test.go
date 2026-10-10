package app

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yeixio/toskar-core/internal/runtimes/external"
	"github.com/yeixio/toskar-core/pkg/pluginapi"
)

// A temperature and a token cap set on a call's context, as Deliberate's
// drafts will (#459), reach the model's server; a call without them sends
// neither, leaving the model's defaults.
func TestGenerateOptionsReachTheModel(t *testing.T) {
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	t.Setenv("TOSKAR_DISCOVERY_ENABLED", "false")
	a, err := New(Options{DataDir: t.TempDir(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.DB.Close() })
	a.External.SetConfig(external.Config{BaseURL: srv.URL + "/v1"})

	drain := func(ctx context.Context) {
		t.Helper()
		ch, err := a.generateOnNode(ctx, "", external.ModelPrefix+"some-model", "drafter:1", "", []pluginapi.ChatMessage{{Role: "user", Content: "2+2?"}})
		if err != nil {
			t.Fatal(err)
		}
		for range ch {
		}
	}
	drain(pluginapi.WithGenerateOptions(context.Background(), pluginapi.GenerateOptions{Temperature: 0.7, MaxTokens: 300}))
	drain(context.Background())
	if len(bodies) != 2 {
		t.Fatalf("calls: %d", len(bodies))
	}
	if bodies[0]["temperature"] != 0.7 || bodies[0]["max_tokens"] != float64(300) {
		t.Fatalf("with options: %v", bodies[0])
	}
	if _, ok := bodies[1]["temperature"]; ok {
		t.Fatalf("a temperature sent without one set: %v", bodies[1])
	}
	if _, ok := bodies[1]["max_tokens"]; ok {
		t.Fatalf("a token cap sent without one set: %v", bodies[1])
	}
}
