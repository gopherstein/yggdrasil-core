package imagegen

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A model not yet tested on real hardware is offered but never recommended
// (#548).
func TestUntestedIsNeverRecommended(t *testing.T) {
	for _, mem := range []int64{8 << 30, 16 << 30, 64 << 30} {
		if m := Recommend(mem); m.Untested {
			t.Errorf("%d GB recommends untested %s", mem>>30, m.ID)
		}
	}
	z, ok := Lookup("z-image-turbo")
	if !ok || !z.Untested || z.Edits || z.License != "Apache-2.0" {
		t.Fatalf("z-image-turbo = %+v", z)
	}
}

// With a model that can't edit set up as the active one, an edit uses an
// installed model that can; with none, editing is unavailable and says
// what to set up (#548).
func TestEditsUseAModelThatEdits(t *testing.T) {
	f := newFixture(t)
	base := f.model(t)
	editor, photo := base, base
	editor.ID, editor.Name, editor.Edits = "editor", "Editor", true
	photo.ID, photo.Name, photo.Edits = "photo", "Photo", false
	editor.Files = append([]File(nil), base.Files...)
	photo.Files = append([]File(nil), base.Files...)
	f.setup.Catalog = func() []Model { return []Model{editor, photo} }
	eng := &Engine{Setup: f.setup, WorkDir: t.TempDir()}
	edits := &EditTool{Engine: eng}
	ctx := context.Background()
	archive, _ := f.setup.platform()

	if err := f.setup.install(ctx, &job{}, photo, true, archive); err != nil {
		t.Fatal(err)
	}
	if ok, why := edits.Available(); ok || !strings.Contains(why, "Photo can't edit pictures. Set up Editor") {
		t.Fatalf("edit available = %v, %q", ok, why)
	}
	if _, err := eng.Generate(ctx, Request{Prompt: "make it blue", Reference: samplePNG(t, 64, 64), RefName: "a.png"}); err == nil || !strings.Contains(err.Error(), "can't edit pictures") {
		t.Fatalf("edit without an editor: %v", err)
	}

	if err := f.setup.install(ctx, &job{}, editor, false, archive); err != nil {
		t.Fatal(err)
	}
	// The picture model stays the active one.
	if err := os.WriteFile(filepath.Join(f.setup.ModelsDir, "active"), []byte("photo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if ok, why := edits.Available(); !ok {
		t.Fatalf("edit unavailable: %s", why)
	}
	if _, err := eng.Generate(ctx, Request{Prompt: "make it blue", Reference: samplePNG(t, 64, 64), RefName: "a.png"}); err != nil {
		t.Fatal(err)
	}
	if got := flag(args(t, f), "--diffusion-model"); !strings.Contains(got, string(filepath.Separator)+"editor"+string(filepath.Separator)) {
		t.Fatalf("edit used %s", got)
	}
	if _, err := eng.Generate(ctx, Request{Prompt: "a dog"}); err != nil {
		t.Fatal(err)
	}
	if got := flag(args(t, f), "--diffusion-model"); !strings.Contains(got, string(filepath.Separator)+"photo"+string(filepath.Separator)) {
		t.Fatalf("a new picture used %s", got)
	}
}
