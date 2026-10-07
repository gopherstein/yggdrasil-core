package hfclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/yeixio/toskar-core/internal/models/hfclient"
)

func TestSearchFiltersGGUF(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"id":        "org/Cool-Model-GGUF",
				"downloads": 1000,
				"tags":      []string{"gguf", "text-generation"},
				"siblings": []map[string]any{
					{"rfilename": "Cool-Model-Q4_K_M.gguf", "size": 4_000_000_000},
					{"rfilename": "Cool-Model-Q8_0.gguf", "size": 8_000_000_000},
				},
			},
		})
	}))
	defer srv.Close()

	c := hfclient.New()
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()

	out, err := c.Search(context.Background(), "cool", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1, got %d", len(out))
	}
	if out[0].Variant != "Q4_K_M" {
		t.Fatalf("preferred quant, got %s", out[0].Variant)
	}
	if out[0].SourceURL == "" {
		t.Fatal("expected source url")
	}
}

// Search lists each repository's files, and one with a projector is shown as
// a vision model.
func TestSearchMarksVisionRepos(t *testing.T) {
	var full string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		full = r.URL.Query().Get("full")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "org/Seeing-3B-GGUF", "siblings": []map[string]any{
				{"rfilename": "Seeing-3B-Q4_K_M.gguf"}, {"rfilename": "mmproj-Seeing-3B-f16.gguf"},
			}},
		})
	}))
	defer srv.Close()
	c := hfclient.New()
	c.BaseURL, c.HTTP = srv.URL, srv.Client()
	out, err := c.Search(context.Background(), "seeing", 10)
	if err != nil {
		t.Fatal(err)
	}
	if full != "true" || len(out) != 1 || out[0].Filename != "Seeing-3B-Q4_K_M.gguf" {
		t.Fatalf("full=%q out=%+v", full, out)
	}
	if !slices.Contains(out[0].Tags, "vision") {
		t.Fatalf("tags = %v", out[0].Tags)
	}
}
