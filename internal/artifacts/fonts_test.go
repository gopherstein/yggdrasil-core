package artifacts

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ledongthuc/pdf"
)

func pdfText(t *testing.T, data []byte) string {
	t.Helper()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("a PDF reader cannot open it: %v", err)
	}
	var text strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		s, err := r.Page(i).GetPlainText(nil)
		if err != nil {
			t.Fatal(err)
		}
		text.WriteString(s)
	}
	return strings.ReplaceAll(text.String(), "\n", "")
}

// Text beyond Western European characters is written, and reads back
// (#510): accents, Greek, Cyrillic, and symbols, in text, bold, and code.
func TestPDFUnicodeText(t *testing.T) {
	data, err := MarkdownToPDF("# Ελληνικά και Русский\n\nTiếng Việt, **Łódź**, *Çeşme* ± 5 € ….\n\n```\nnaïve – “done”\n```\n")
	if err != nil {
		t.Fatal(err)
	}
	got := pdfText(t, data)
	for _, want := range []string{"Ελληνικά και Русский", "Tiếng Việt,", "Łódź", "Çeşme", "± 5 € ….", "naïve – “done”"} {
		if !strings.Contains(got, want) {
			t.Errorf("PDF text lacks %q; it is %q", want, got)
		}
	}
	if strings.Contains(got, "?") {
		t.Errorf("a character wasn't shown: %q", got)
	}
}

// Characters no font has are shown as "?", as before.
func TestPDFMissingCharacters(t *testing.T) {
	data, err := MarkdownToPDF("Made in 日本.")
	if err != nil {
		t.Fatal(err)
	}
	if got := pdfText(t, data); !strings.Contains(got, "Made in ??.") {
		t.Fatalf("PDF text is %q", got)
	}
}

// The embedded font holds only the glyphs used, and the glyphs composite
// ones are built from, each unchanged.
func TestFontSubset(t *testing.T) {
	builtin, err := builtinFonts()
	if err != nil {
		t.Fatal(err)
	}
	f := builtin[0]
	var gids []uint16
	composite := uint16(0)
	for _, r := range "AÅé€" {
		g := f.cmap[r]
		gids = append(gids, g)
		if d := f.glyph(g); len(d) > 0 && i16(d, 0) < 0 {
			composite = g
		}
	}
	file, order, err := f.subset(append([]uint16{0}, gids...))
	if err != nil {
		t.Fatal(err)
	}
	sub, err := parseTTF(file)
	if err != nil {
		t.Fatalf("the subset doesn't read: %v", err)
	}
	if sub.numGlyphs != len(order) || len(order) < 5 {
		t.Fatalf("%d glyphs, order %v", sub.numGlyphs, order)
	}
	if composite != 0 && len(order) == 5 {
		t.Fatal("a composite glyph's parts weren't kept")
	}
	for i, g := range order {
		if sub.advance(uint16(i)) != f.advance(g) {
			t.Errorf("glyph %d's width changed", g)
		}
		if d := f.glyph(g); len(d) > 0 && i16(d, 0) >= 0 && !bytes.Equal(sub.glyph(uint16(i))[:len(d)], d) {
			t.Errorf("glyph %d's outline changed", g)
		}
	}
	if checksum(file) != 0xB1B0AFBA {
		t.Error("the font's checksum is wrong")
	}
}

// A font for Chinese, Japanese, or Korean is downloaded the first time a
// PDF needs it, checked, and kept for the next.
func TestPDFFontDownload(t *testing.T) {
	var served atomic.Int32
	body := notoSansRegular
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served.Add(1)
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	saved := cjkFonts["ja"]
	defer func() { cjkFonts["ja"] = saved }()
	cjkFonts["ja"] = cjkFont{"Test-Regular.ttf", srv.URL, hashOf(notoSansRegular), int64(len(notoSansRegular))}

	dir := t.TempDir()
	ctx := context.Background()
	if _, err := (&Fonts{Dir: dir}).MarkdownToPDF(ctx, "ひらがな and plain text"); err != nil {
		t.Fatal(err)
	}
	if served.Load() != 1 {
		t.Fatalf("downloaded %d times", served.Load())
	}
	if _, err := os.Stat(filepath.Join(dir, "Test-Regular.ttf")); err != nil {
		t.Fatal(err)
	}
	// Another PDF, even in a new process, uses the copy kept.
	if _, err := (&Fonts{Dir: dir}).MarkdownToPDF(ctx, "カタカナ"); err != nil || served.Load() != 1 {
		t.Fatalf("downloaded again: %d, %v", served.Load(), err)
	}
	// Text that needs no other font downloads nothing.
	if _, err := (&Fonts{Dir: t.TempDir()}).MarkdownToPDF(ctx, "Только кириллица"); err != nil || served.Load() != 1 {
		t.Fatalf("downloaded for Cyrillic: %d, %v", served.Load(), err)
	}
	// A file that isn't the font expected is refused, and not kept.
	body = []byte("not a font")
	other := t.TempDir()
	if _, err := (&Fonts{Dir: other}).MarkdownToPDF(ctx, "ひらがな"); err == nil {
		t.Fatal("a wrong file was used as the font")
	}
	if entries, _ := os.ReadDir(other); len(entries) != 0 {
		t.Fatalf("kept %v", entries)
	}
}

func TestCJKNeeds(t *testing.T) {
	for text, want := range map[string]string{
		"plain":             "",
		"日本語のテキスト":          "ja",
		"한국어 텍스트":           "ko",
		"这是简体中文的说明":         "zh-Hans",
		"這是繁體中文的說明":         "zh-Hant",
		"한국어 and ひらがな":      "ja ko",
		"漢字 only, no clues": "zh-Hans",
	} {
		if got := strings.Join(cjkNeeds(text), " "); got != want {
			t.Errorf("%q needs %q, want %q", text, got, want)
		}
	}
}

// With the real fonts (TOSKAR_TEST_FONTS: a folder holding them), Chinese,
// Japanese, and Korean text reads back.
func TestPDFCJKText(t *testing.T) {
	dir := os.Getenv("TOSKAR_TEST_FONTS")
	if dir == "" {
		t.Skip("TOSKAR_TEST_FONTS is not set")
	}
	fs := &Fonts{Dir: dir, Client: &http.Client{Transport: failingTransport{}}}
	for _, text := range []string{"# 日本語の見出し\n\n本文はひらがなとカタカナ。", "# 한국어 제목\n\n본문입니다.", "# 简体中文\n\n这是说明。", "# 繁體中文\n\n這是說明。"} {
		data, err := fs.MarkdownToPDF(context.Background(), text)
		if err != nil {
			t.Fatal(err)
		}
		got := pdfText(t, data)
		for _, line := range strings.Split(text, "\n") {
			if line = strings.TrimPrefix(line, "# "); line != "" && !strings.Contains(got, line) {
				t.Errorf("PDF text lacks %q; it is %q", line, got)
			}
		}
	}
}

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, os.ErrPermission
}
