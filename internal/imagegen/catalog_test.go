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

// A model that only edits is preferred for edits, never used for new
// pictures, never made the active model, and gets its own arguments: its
// text encoder's image encoder, flow shift, and model args (#548).
func TestEditOnlyModel(t *testing.T) {
	q, ok := Lookup("qwen-image-edit-2511")
	if !ok || !q.EditOnly || !q.Edits || !q.Untested || q.ModelArgs != "qwen_image_zero_cond_t=true" {
		t.Fatalf("qwen-image-edit-2511 = %+v", q)
	}
	f := newFixture(t)
	base := f.model(t)
	picture, editor := base, base
	picture.ID, picture.Name, picture.Edits = "picture", "Picture", true
	editor.ID, editor.Name, editor.Edits, editor.EditOnly = "editor", "Editor", true, true
	editor.FlowShift, editor.ModelArgs = 3, "zero_cond_t=true"
	f.files["vision.gguf"] = []byte("vision")
	editor.Files = append(append([]File(nil), base.Files...), File{Role: "llm_vision", Path: "vision.gguf", Size: 6, SHA256: sum([]byte("vision"))})
	f.setup.Catalog = func() []Model { return []Model{picture, editor} }
	eng := &Engine{Setup: f.setup, WorkDir: t.TempDir()}
	ctx := context.Background()
	archive, _ := f.setup.platform()
	if err := f.setup.install(ctx, &job{}, picture, true, archive); err != nil {
		t.Fatal(err)
	}
	if err := f.setup.install(ctx, &job{}, editor, false, archive); err != nil {
		t.Fatal(err)
	}
	if got := f.setup.ActiveModel(); got != "picture" {
		t.Fatalf("active = %s", got)
	}
	if _, err := eng.Generate(ctx, Request{Prompt: "make it blue", Reference: samplePNG(t, 64, 64), RefName: "a.png"}); err != nil {
		t.Fatal(err)
	}
	got := args(t, f)
	if !strings.Contains(flag(got, "--diffusion-model"), string(filepath.Separator)+"editor"+string(filepath.Separator)) ||
		filepath.Base(flag(got, "--llm_vision")) != "vision.gguf" || flag(got, "--flow-shift") != "3" || flag(got, "--model-args") != "zero_cond_t=true" {
		t.Fatalf("edit args %q", got)
	}
	if _, err := eng.Generate(ctx, Request{Prompt: "a dog"}); err != nil {
		t.Fatal(err)
	}
	got = args(t, f)
	if !strings.Contains(flag(got, "--diffusion-model"), string(filepath.Separator)+"picture"+string(filepath.Separator)) ||
		flag(got, "--llm_vision") != "" || flag(got, "--model-args") != "" {
		t.Fatalf("new picture args %q", got)
	}
	// With only the editor installed, nothing is active, so new pictures
	// aren't made with it.
	os.RemoveAll(f.setup.modelDir("picture"))
	if got := f.setup.ActiveModel(); got != "" {
		t.Fatalf("the editor became active: %s", got)
	}
}

// The light clip model is untested and takes no picture; on a computer
// where no model that takes one fits, the message says so instead of
// naming one that can't be set up (#548).
func TestLightClipModel(t *testing.T) {
	m, ok := lookupIn(VideoCatalog(), "wan2.1-t2v-1.3b")
	if !ok || !m.Untested || m.Edits || m.MemoryBytes >= 16<<30 {
		t.Fatalf("wan2.1-t2v-1.3b = %+v", m)
	}
	for _, v := range VideoCatalog() {
		for _, file := range v.Files {
			if file.Source == "" && revisions[file.Repo] == "" || len(file.SHA256) != 64 || file.Size == 0 {
				t.Errorf("%s: %s is not pinned", v.ID, file.Path)
			}
		}
	}
	small := &Setup{ModelsDir: t.TempDir(), Catalog: VideoCatalog, Memory: func() int64 { return 8 << 30 }}
	if got := editModelMissing(small); !strings.Contains(got, "no model that can fits this computer") {
		t.Fatalf("8 GB: %q", got)
	}
	big := &Setup{ModelsDir: t.TempDir(), Catalog: VideoCatalog, Memory: func() int64 { return 64 << 30 }}
	if got := editModelMissing(big); !strings.Contains(got, "Set up Wan 2.2 TI2V 5B") {
		t.Fatalf("64 GB: %q", got)
	}
}
