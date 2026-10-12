package imagegen

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// A computer set up with an earlier stable-diffusion.cpp release stays
// ready, and its next picture installs this release's program, keeps its
// model, and removes the earlier program (#548).
func TestEarlierReleaseUpdatesOnUse(t *testing.T) {
	f, eng, _, _ := setupTools(t)
	dir := f.setup.ProgramDir
	old := filepath.Join(dir, "master-1-0000000")
	if err := os.Rename(filepath.Join(dir, Release), old); err != nil {
		t.Fatal(err)
	}
	if f.setup.CLI() != "" || !f.setup.stale() {
		t.Fatal("not seen as an earlier release's setup")
	}
	if st := f.setup.Status(); !st.Ready || st.Program {
		t.Fatalf("status %+v", st)
	}
	if ok, why := eng.Available(); !ok {
		t.Fatalf("unavailable: %s", why)
	}
	if _, err := eng.Generate(context.Background(), Request{Prompt: "a dog"}); err != nil {
		t.Fatal(err)
	}
	if f.setup.CLI() == "" || f.setup.stale() {
		t.Fatal("this release wasn't installed")
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("the earlier release is still there: %v", err)
	}
	// A computer never set up isn't stale, and stays not ready.
	fresh := newFixture(t)
	if fresh.setup.stale() || fresh.setup.Status().Ready {
		t.Fatal("a fresh computer counts as set up")
	}
}
