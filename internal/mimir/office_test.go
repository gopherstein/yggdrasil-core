package mimir_test

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/yeixio/toskar-core/internal/artifacts"
	"github.com/yeixio/toskar-core/internal/mimir"
)

// A Word document Toskar made reads back, paragraph by paragraph (#510):
// it made .docx files it then refused to read.
func TestReadDOCX(t *testing.T) {
	doc, err := artifacts.MarkdownToDOCX("# Invoice 4471\n\nTotal due: $1,284.50\n\n- Winter tires\n- Alignment")
	if err != nil {
		t.Fatal(err)
	}
	if !mimir.Attachable("invoice.docx") {
		t.Fatal("a .docx can't be attached")
	}
	passages, err := mimir.FilePassages("invoice.docx", doc)
	if err != nil {
		t.Fatal(err)
	}
	text := joined(passages)
	for _, want := range []string{"Invoice 4471", "Total due: $1,284.50", "Winter tires", "Alignment"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in %q", want, text)
		}
	}
	if _, err := mimir.FilePassages("broken.docx", []byte("not a zip")); err == nil {
		t.Error("a broken .docx was read")
	}
}

// A PowerPoint deck reads slide by slide, in slide order, not zip order.
func TestReadPPTX(t *testing.T) {
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	slide := func(n, text string) {
		w, _ := z.Create("ppt/slides/slide" + n + ".xml")
		_, _ = w.Write([]byte(`<p:sld xmlns:p="p" xmlns:a="a"><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>` + text + `</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`))
	}
	slide("10", "Ten: questions")
	slide("2", "Two: winter tires save lives")
	slide("1", "One: Dana's Tire Shop")
	_ = z.Close()
	passages, err := mimir.FilePassages("deck.pptx", b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	text := joined(passages)
	one, two, ten := strings.Index(text, "One:"), strings.Index(text, "Two:"), strings.Index(text, "Ten:")
	if one < 0 || two < one || ten < two {
		t.Errorf("slides out of order: %q", text)
	}
	if !strings.Contains(text, "slide 2") {
		t.Errorf("passages don't name their slide: %+v", passages)
	}
}

func joined(ps []mimir.Passage) string {
	var b strings.Builder
	for _, p := range ps {
		b.WriteString(p.Title + "\n" + p.Body + "\n")
	}
	return b.String()
}
