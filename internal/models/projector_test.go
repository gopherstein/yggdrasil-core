package models

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/yeixio/toskar-core/internal/events"
	"github.com/yeixio/toskar-core/internal/store"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

func sha(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// A vision model is downloaded with its projector, and a model installed
// before its projector was known gets just the projector (#191).
func TestVisionModelDownloadsItsProjector(t *testing.T) {
	weights, projector := []byte("weights"), []byte("projector")
	var weightGets, projectorGets atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/model.gguf":
			weightGets.Add(1)
			_, _ = w.Write(weights)
		case "/mmproj.gguf":
			projectorGets.Add(1)
			_, _ = w.Write(projector)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	storage := NewStorage(db.SQL, filepath.Join(dir, "models"))
	bus := events.NewBus(64)
	catalog := &Catalog{}
	entry := CatalogEntry{
		ID: "see", DisplayName: "See", SizeBytes: uint64(len(weights) + len(projector)),
		Capabilities: contracts.ModelCapabilities{Vision: true},
		Source:       contracts.ModelSource{URL: srv.URL + "/model.gguf", SHA256: sha(weights)},
		Projector:    &ModelFile{URL: srv.URL + "/mmproj.gguf", SHA256: sha(projector), SizeBytes: uint64(len(projector))},
	}
	catalog.Upsert(entry)
	m := NewManager(catalog, storage, NewDownloader(storage, bus, db.SQL), bus)
	ctx := context.Background()

	if err := m.Install(ctx, "see", true); err != nil {
		t.Fatal(err)
	}
	if !m.SeesImages("see") || m.ProjectorPath("see") != storage.ProjectorPath("see") {
		t.Fatalf("projector = %q", m.ProjectorPath("see"))
	}
	if got, _ := os.ReadFile(m.ProjectorPath("see")); string(got) != "projector" {
		t.Fatalf("projector file = %q", got)
	}

	// Installed without it, as from an older catalog: only the projector is
	// fetched.
	_ = os.Remove(storage.ProjectorPath("see"))
	if m.SeesImages("see") {
		t.Fatal("sees without a projector")
	}
	if err := m.Install(ctx, "see", true); err != nil {
		t.Fatal(err)
	}
	if !m.SeesImages("see") || weightGets.Load() != 1 || projectorGets.Load() != 2 {
		t.Fatalf("sees=%v weights=%d projector=%d", m.SeesImages("see"), weightGets.Load(), projectorGets.Load())
	}

	// A bad projector fails the download and leaves the model unable to see.
	_ = os.Remove(storage.ProjectorPath("see"))
	bad := entry
	bad.Projector = &ModelFile{URL: srv.URL + "/mmproj.gguf", SHA256: sha([]byte("other"))}
	catalog.Upsert(bad)
	if err := m.Install(ctx, "see", true); err == nil || m.SeesImages("see") {
		t.Fatalf("bad projector: err=%v sees=%v", err, m.SeesImages("see"))
	}
	catalog.Upsert(entry)
	if err := m.Install(ctx, "see", true); err != nil {
		t.Fatal(err)
	}

	// Deleting the model deletes its projector.
	if err := m.Delete(ctx, "see"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(storage.ProjectorPath("see")); !os.IsNotExist(err) {
		t.Fatal("the projector was left behind")
	}
}

func TestNoSystemRole(t *testing.T) {
	catalog := &Catalog{}
	catalog.Upsert(CatalogEntry{ID: "marked", NoSystemRole: true})
	catalog.Upsert(CatalogEntry{ID: "qwen2.5-7b-q4", Family: "qwen2.5"})
	catalog.Upsert(CatalogEntry{ID: "hf-model", Family: "gemma3"})
	m := NewManager(catalog, nil, nil, nil)
	for id, want := range map[string]bool{"marked": true, "qwen2.5-7b-q4": false, "hf-model": true, "gemma-3-12b-it-q4": true, "unknown": false} {
		if got := m.NoSystemRole(id); got != want {
			t.Errorf("%s: %v, want %v", id, got, want)
		}
	}
}
