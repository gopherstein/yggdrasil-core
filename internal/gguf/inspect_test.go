package gguf_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yeixio/toskar-core/internal/gguf"
	"github.com/yeixio/toskar-core/internal/gguf/gguftest"
)

// Inspect reads what an added model says about itself, and refuses a file
// that isn't whole (#467).
func TestInspect(t *testing.T) {
	dir := t.TempDir()
	path := gguftest.Write(t, dir, "My-Model-Q4_K_M.gguf", gguftest.Options{Name: "My Model", FileType: 15, Context: 8192, Elements: 256})
	d, err := gguf.Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "My Model" || d.Architecture != "llama" || d.Quantization != "Q4_K_M" || d.ContextLength != 8192 || d.Parameters != 256 || d.Projector() {
		t.Errorf("details = %+v", d)
	}

	// No file type in the header: the name says Q5_K_S.
	if d, err := gguf.Inspect(gguftest.Write(t, dir, "other.Q5_K_S.gguf", gguftest.Options{})); err != nil || d.Quantization != "Q5_K_S" {
		t.Errorf("from the name: %+v, %v", d, err)
	}
	if d, _ := gguf.Inspect(gguftest.Write(t, dir, "mmproj.gguf", gguftest.Options{Architecture: "clip", Type: "mmproj"})); !d.Projector() {
		t.Error("a projector isn't seen as one")
	}

	for name, cut := range map[string]int{"data": 8, "header": 300} {
		if _, err := gguf.Inspect(gguftest.Write(t, dir, name+".gguf", gguftest.Options{Name: "Cut", Cut: cut})); !errors.Is(err, gguf.ErrTruncated) {
			t.Errorf("cut in the %s: err = %v", name, err)
		}
	}
	notGGUF := filepath.Join(dir, "notes.gguf")
	if err := os.WriteFile(notGGUF, []byte("hello, this is text"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gguf.Inspect(notGGUF); err == nil || err.Error() != "not a GGUF file" {
		t.Errorf("text file: %v", err)
	}
}
