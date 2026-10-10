package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeixio/toskar-core/internal/config"
	"github.com/yeixio/toskar-core/internal/events"
	"github.com/yeixio/toskar-core/internal/gguf/gguftest"
	"github.com/yeixio/toskar-core/internal/models"
	"github.com/yeixio/toskar-core/internal/store"
)

// POST /models/import adds a GGUF model from a path on this computer, or
// from the file sent as the body, and refuses one that isn't whole (#467).
func TestImportModelRoute(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	storage := models.NewStorage(db.SQL, filepath.Join(dir, "models"))
	bus := events.NewBus(16)
	mgr := models.NewManager(&models.Catalog{}, storage, models.NewDownloader(storage, bus, db.SQL), bus)
	cfg, err := config.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mgr.Home = filepath.Join(dir, "home")
	t.Setenv("OLLAMA_MODELS", "")
	srv := NewServer(Dependencies{Config: cfg, ImportModel: mgr.ImportFile, ModelUploadPath: mgr.UploadPath, AdoptModelUpload: mgr.AdoptUpload, FindModelsInOtherApps: mgr.FindOtherApps})
	call := func(path, contentType string, body []byte) (int, map[string]any) {
		r := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		r.Header.Set("Content-Type", contentType)
		r.RemoteAddr, r.Host = "127.0.0.1:50000", "127.0.0.1:7331"
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, r)
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}

	src := gguftest.Write(t, filepath.Join(dir, "lmstudio"), "Small-Q4_K_M.gguf", gguftest.Options{Name: "Small", FileType: 15, Context: 2048})
	body, _ := json.Marshal(map[string]any{"path": src, "in_place": true})
	code, out := call("/api/v1/models/import", "application/json", body)
	details, _ := out["details"].(map[string]any)
	if code != http.StatusOK || out["model_id"] != "small-q4-k-m" || out["status"] != "installed" || details["quantization"] != "Q4_K_M" {
		t.Fatalf("path import: %d %v", code, out)
	}

	code, out = call("/api/v1/models/import?filename=phone.gguf&display_name=From+the+phone", "application/octet-stream", gguftest.Bytes(gguftest.Options{Name: "Sent"}))
	if code != http.StatusOK || out["model_id"] != "from-the-phone" {
		t.Fatalf("upload: %d %v", code, out)
	}

	code, out = call("/api/v1/models/import?filename=cut.gguf", "application/octet-stream", gguftest.Bytes(gguftest.Options{Cut: 10}))
	if code != http.StatusBadRequest || !strings.Contains(out["error"].(map[string]any)["code"].(string), "MODEL_FILE_INVALID") {
		t.Fatalf("truncated upload: %d %v", code, out)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "models", ".partial", "upload-*")); len(left) != 0 {
		t.Errorf("a refused upload was left behind: %v", left)
	}
	if code, _ := call("/api/v1/models/import", "application/octet-stream", []byte("x")); code != http.StatusBadRequest {
		t.Errorf("upload without a filename: %d", code)
	}
	if _, err := os.Stat(src); err != nil {
		t.Errorf("the original file: %v", err)
	}

	gguftest.Write(t, filepath.Join(dir, "home", ".lmstudio", "models", "x"), "Found-Q8_0.gguf", gguftest.Options{Name: "Found"})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/models/import/found", nil)
	r.RemoteAddr, r.Host = "127.0.0.1:50000", "127.0.0.1:7331"
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, r)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"app":"lmstudio"`) || !strings.Contains(rec.Body.String(), `"name":"Found"`) {
		t.Errorf("found: %d %s", rec.Code, rec.Body.String())
	}
}
