package models

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/yeixio/toskar-core/internal/gguf/gguftest"
)

// Before a download from a link, the preview says its size, the free
// space, and that another app already has the same file (#611).
func TestPreviewURL(t *testing.T) {
	m, _, dir := importManager(t)
	home := filepath.Join(dir, "home")
	m.Home = home
	t.Setenv("OLLAMA_MODELS", "")
	ctx := context.Background()
	lm := filepath.Join(home, ".lmstudio", "models", "unsloth", "Qwen3.5-35B-A3B-GGUF")
	copyPath := gguftest.Write(t, lm, "Qwen3.5-35B-A3B-Q4_K_M.gguf", gguftest.Options{Name: "Qwen3.5 35B A3B"})
	ollama := filepath.Join(home, ".ollama", "models")
	gguftest.Write(t, filepath.Join(ollama, "blobs"), "sha256-feedface", gguftest.Options{Name: "Other"})
	manifest := filepath.Join(ollama, "manifests", "registry.ollama.ai", "library", "qwen3.5", "35b")
	if err := os.MkdirAll(filepath.Dir(manifest), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(manifest, []byte(`{"layers":[{"mediaType":"application/vnd.ollama.image.model","digest":"sha256:feedface"}]}`), 0o644)
	m.Resolve = func(context.Context, string) (ModelFile, *ModelFile, error) {
		return ModelFile{SizeBytes: 22_000_000_000, SHA256: "FEEDFACE"}, &ModelFile{SizeBytes: 900_000_000}, nil
	}
	m.FreeDisk = func(string) (uint64, error) { return 120_000_000_000, nil }

	const link = "https://huggingface.co/unsloth/Qwen3.5-35B-A3B-GGUF/resolve/main/Qwen3.5-35B-A3B-Q4_K_M.gguf"
	p, err := m.PreviewURL(ctx, link)
	if err != nil {
		t.Fatal(err)
	}
	if p.Filename != "Qwen3.5-35B-A3B-Q4_K_M.gguf" || p.SizeBytes != 22_000_000_000 || p.ProjectorBytes != 900_000_000 || p.FreeBytes != 120_000_000_000 || p.Installed {
		t.Fatalf("preview = %+v", p)
	}
	// LM Studio's copy by name, and Ollama's blob by the file's SHA-256.
	if len(p.Elsewhere) != 2 {
		t.Fatalf("elsewhere = %+v", p.Elsewhere)
	}
	var byName bool
	for _, f := range p.Elsewhere {
		byName = byName || f.Path == copyPath
	}
	if !byName {
		t.Fatalf("LM Studio's copy not offered: %+v", p.Elsewhere)
	}

	// Added from LM Studio where it is: the preview says it's already here.
	imported, err := m.ImportFile(ctx, ImportRequest{Path: copyPath, InPlace: true})
	if err != nil {
		t.Fatal(err)
	}
	m.Resolve = nil
	p, _ = m.PreviewURL(ctx, link)
	if !p.Installed || p.InstalledAs != imported.ID {
		t.Fatalf("after adding: %+v (imported %s)", p, imported.ID)
	}

	if _, err := m.PreviewURL(ctx, "https://example.com/model.bin"); err == nil {
		t.Fatal("a link that isn't a GGUF")
	}
}

func TestSameFile(t *testing.T) {
	f := FoundModel{Path: "/x/Model-Q4_K_M.gguf", SizeBytes: 5 << 30}
	for _, c := range []struct {
		name, sha string
		size      uint64
		want      bool
	}{
		{"model-q4_k_m.gguf", "", 0, true},
		{"other.gguf", "", 5 << 30, true},
		{"other.gguf", "", 50 << 20, false},
		{"other.gguf", "", 4 << 30, false},
	} {
		if got := sameFile(f, c.name, c.sha, c.size); got != c.want {
			t.Errorf("sameFile(%q, %d) = %v", c.name, c.size, got)
		}
	}
	if !sameFile(FoundModel{Path: "/b/sha256-abc"}, "x.gguf", "abc", 0) {
		t.Error("Ollama blob by SHA-256")
	}
}
