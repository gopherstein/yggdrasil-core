package hfclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseResolveURL(t *testing.T) {
	repo, rev, file, ok := ParseResolveURL("https://huggingface.co/org/Some-GGUF/resolve/main/sub/model-Q4_K_M.gguf")
	if !ok || repo != "org/Some-GGUF" || rev != "main" || file != "sub/model-Q4_K_M.gguf" {
		t.Fatalf("got %q %q %q %v", repo, rev, file, ok)
	}
	for _, bad := range []string{
		"http://huggingface.co/org/r/resolve/main/m.gguf",
		"https://example.com/org/r/resolve/main/m.gguf",
		"https://huggingface.co/org/r/blob/main/m.gguf",
		"https://huggingface.co/org/r/resolve/main/",
		"https://huggingface.co.evil.test/org/r/resolve/main/m.gguf",
	} {
		if _, _, _, ok := ParseResolveURL(bad); ok {
			t.Errorf("accepted %s", bad)
		}
	}
}

// A vision model's projector comes from its own repository: the one in the
// same folder, full precision first (#191).
func TestResolveFindsTheProjector(t *testing.T) {
	var asked string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.String()
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"type": "directory", "path": "GGUF"},
			{"type": "file", "path": "README.md", "size": 10},
			{"type": "file", "path": "mmproj-other-f16.gguf", "size": 5, "lfs": map[string]any{"oid": "root"}},
			{"type": "file", "path": "GGUF/model-Q4_K_M.gguf", "size": 2000, "lfs": map[string]any{"oid": "abc"}},
			{"type": "file", "path": "GGUF/mmproj-model-Q8_0.gguf", "size": 600, "lfs": map[string]any{"oid": "q8"}},
			{"type": "file", "path": "GGUF/mmproj-model-f16.gguf", "size": 900, "lfs": map[string]any{"oid": "f16"}},
		})
	}))
	defer srv.Close()
	c := New()
	c.BaseURL, c.HTTP = srv.URL, srv.Client()

	repo, err := c.Resolve(context.Background(), "https://huggingface.co/org/VL-GGUF/resolve/main/GGUF/model-Q4_K_M.gguf")
	if err != nil {
		t.Fatal(err)
	}
	if asked != "/models/org/VL-GGUF/tree/main?recursive=true" {
		t.Fatalf("asked %s", asked)
	}
	if repo.Model.SHA256 != "abc" || repo.Model.Size != 2000 {
		t.Fatalf("model = %+v", repo.Model)
	}
	p := repo.Projector
	if p == nil || p.Path != "GGUF/mmproj-model-f16.gguf" || p.SHA256 != "f16" || p.Size != 900 ||
		p.URL != "https://huggingface.co/org/VL-GGUF/resolve/main/GGUF/mmproj-model-f16.gguf" {
		t.Fatalf("projector = %+v", p)
	}

	if _, err := c.Resolve(context.Background(), "https://huggingface.co/org/VL-GGUF/resolve/main/missing.gguf"); err == nil {
		t.Fatal("a file the repository doesn't have was resolved")
	}
}

func TestTextModelHasNoProjector(t *testing.T) {
	if _, ok := pickProjector([]File{{Path: "model-Q4_K_M.gguf"}, {Path: "README.md"}}, "model-Q4_K_M.gguf"); ok {
		t.Fatal("found a projector in a text model's repository")
	}
}

func TestPickProjectorPrefersF16OverBF16(t *testing.T) {
	files := []File{{Path: "mmproj-BF16.gguf"}, {Path: "mmproj-F32.gguf"}, {Path: "mmproj-F16.gguf"}, {Path: "m.gguf"}}
	if p, ok := pickProjector(files, "m.gguf"); !ok || p.Path != "mmproj-F16.gguf" {
		t.Fatalf("picked %+v", p)
	}
	if p, _ := pickProjector(files[:2], "m.gguf"); p.Path != "mmproj-BF16.gguf" {
		t.Fatalf("without f16 picked %+v", p)
	}
}

// The model file is the model, not a draft head, a split part, or an extra
// in a subfolder.
func TestPickGGUFSkipsExtras(t *testing.T) {
	name, _, _ := pickGGUF([]hfFile{
		{RFilename: "MTP/mtp-gemma-4-31B-it-Q4_0.gguf"},
		{RFilename: "MTP/mtp-gemma-4-31B-it-Q8_0.gguf"},
		{RFilename: "mtp-gemma-4-31B-it.gguf"},
		{RFilename: "big-Q4_K_M-00001-of-00002.gguf"},
		{RFilename: "gemma-4-31B-it-qat-UD-Q4_K_XL.gguf"},
		{RFilename: "mmproj-F16.gguf"},
	})
	if name != "gemma-4-31B-it-qat-UD-Q4_K_XL.gguf" {
		t.Fatalf("picked %s", name)
	}
	if name, _, _ := pickGGUF([]hfFile{{RFilename: "GGUF/m-Q4_K_M.gguf"}, {RFilename: "m-Q8_0.gguf"}}); name != "m-Q8_0.gguf" {
		t.Fatalf("subfolder: picked %s", name)
	}
}
