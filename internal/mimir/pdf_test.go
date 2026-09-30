package mimir

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPDFPagesBecomePassages(t *testing.T) {
	raw, err := os.ReadFile("testdata/warranty.pdf")
	if err != nil {
		t.Fatal(err)
	}
	docs, err := parsePDF("warranty.pdf", raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 || docs[0].Name != "warranty.pdf p.1" || docs[1].Name != "warranty.pdf p.2" {
		t.Fatalf("docs = %+v", docs)
	}
	if !strings.Contains(docs[0].Text, "60,000 mile treadwear warranty") {
		t.Fatalf("page 1 = %q", docs[0].Text)
	}
}

func TestPDFEmbeddedFontText(t *testing.T) {
	// Word and Google Docs exports embed TrueType fonts; text is decoded
	// through the font's ToUnicode map.
	raw, err := os.ReadFile("testdata/unicode.pdf")
	if err != nil {
		t.Fatal(err)
	}
	docs, err := parsePDF("unicode.pdf", raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(docs[0].Text, "Café hours") || !strings.Contains(docs[0].Text, "Crème brûlée") {
		t.Fatalf("text = %q", docs[0].Text)
	}
}

func TestPDFErrors(t *testing.T) {
	if _, err := parsePDF("x.pdf", []byte("%PDF-1.4 garbage")); err == nil {
		t.Fatal("a broken PDF must fail")
	}
}

func TestPDFSourceIsSearchable(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	dir := t.TempDir()
	raw, _ := os.ReadFile("testdata/warranty.pdf")
	if err := os.WriteFile(filepath.Join(dir, "warranty.pdf"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, CreateInput{Kind: KindPath, Path: dir}); err != nil {
		t.Fatal(err)
	}
	hits, err := s.Search(ctx, SearchInput{Query: "How much is winter storage?"})
	if err != nil || len(hits) == 0 || hits[0].Title != "warranty.pdf p.2" {
		t.Fatalf("hits = %+v %v", hits, err)
	}
}
