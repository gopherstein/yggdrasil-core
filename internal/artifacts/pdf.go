package artifacts

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf16"

	"golang.org/x/text/encoding/charmap"
)

// MarkdownToPDF makes an A4 PDF from Markdown (Gungnir §21), in Noto Sans
// for text and Courier for code. Chinese, Japanese, and Korean need
// (*Fonts).MarkdownToPDF; here they're shown as "?".
func MarkdownToPDF(text string) ([]byte, error) {
	return markdownToPDF(text, nil)
}

func markdownToPDF(text string, extra []*ttfFont) ([]byte, error) {
	p, err := newPDFLayout(extra)
	if err != nil {
		return nil, err
	}
	for _, b := range parseMarkdown(text) {
		p.block(b)
	}
	return p.finish()
}

const (
	pageW, pageH = 595.0, 842.0
	marginX      = 56.0
	marginTop    = 60.0
	marginBottom = 60.0
	contentW     = pageW - 2*marginX
)

type pdfFont int

const (
	fontRegular pdfFont = iota
	fontBold
	fontItalic
	fontMono
)

// A face is one font in the PDF: an embedded TrueType font, of which only
// the glyphs used are kept, or Courier, a standard font nothing is
// embedded for.
type face struct {
	ttf     *ttfFont
	res     int      // its resource name is /F<res>
	scale   float64  // font units to thousandths of the font size
	gids    []uint16 // the font's glyph for each glyph in the PDF
	cid     map[uint16]uint16
	text    map[uint16]rune // the character each glyph in the PDF shows
	used    bool
	hasBold bool // false when bold text in it is drawn thicker
}

func newFace(f *ttfFont, res int, hasBold bool) *face {
	fc := &face{ttf: f, res: res, hasBold: hasBold}
	if f != nil {
		fc.scale = 1000 / float64(f.unitsPerEm)
		fc.gids = []uint16{0}
		fc.cid = map[uint16]uint16{0: 0}
		fc.text = map[uint16]rune{}
	}
	return fc
}

func (fc *face) has(r rune) bool {
	if fc.ttf == nil {
		_, ok := charmap.Windows1252.EncodeRune(r)
		return ok
	}
	_, ok := fc.ttf.cmap[r]
	return ok
}

// width is a character's width in thousandths of the font size.
func (fc *face) width(r rune) float64 {
	if fc.ttf == nil {
		return 600
	}
	return math.Round(float64(fc.ttf.advance(fc.ttf.cmap[r])) * fc.scale)
}

// encode adds a character to s as the font's code for it.
func (fc *face) encode(s []byte, r rune) []byte {
	fc.used = true
	if fc.ttf == nil {
		b, _ := charmap.Windows1252.EncodeRune(r)
		return append(s, b)
	}
	g := fc.ttf.cmap[r]
	c, ok := fc.cid[g]
	if !ok {
		c = uint16(len(fc.gids))
		fc.cid[g] = c
		fc.gids = append(fc.gids, g)
		fc.text[c] = r
	}
	return append(s, byte(c>>8), byte(c))
}

// clean is text as the PDF shows it: tabs as four spaces, no-break spaces
// as spaces, and no control characters.
func clean(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteString("    ")
		case r == '\u00a0':
			b.WriteByte(' ')
		case unicode.IsControl(r):
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func spanFont(s span) pdfFont {
	switch {
	case s.Code:
		return fontMono
	case s.Bold:
		return fontBold
	case s.Italic:
		return fontItalic
	}
	return fontRegular
}

type pdfLayout struct {
	pages  []*bytes.Buffer
	cur    *bytes.Buffer
	y      float64
	faces  []*face
	chains [4][]*face // the faces tried for each style, in order
}

func newPDFLayout(extra []*ttfFont) (*pdfLayout, error) {
	builtin, err := builtinFonts()
	if err != nil {
		return nil, err
	}
	regular, bold, mono := newFace(builtin[0], 1, true), newFace(builtin[1], 2, true), newFace(nil, 3, true)
	p := &pdfLayout{faces: []*face{regular, bold, mono}}
	var more []*face
	for _, f := range extra {
		fc := newFace(f, len(p.faces)+1, false)
		p.faces = append(p.faces, fc)
		more = append(more, fc)
	}
	chain := func(fs ...*face) []*face { return append(fs, more...) }
	p.chains = [4][]*face{
		fontRegular: chain(regular),
		fontBold:    chain(bold),
		fontItalic:  chain(regular),
		fontMono:    chain(mono, regular),
	}
	p.newPage()
	return p, nil
}

// scratch is a layout that shares p's fonts, to measure text on.
func (p *pdfLayout) scratch() *pdfLayout {
	return &pdfLayout{cur: &bytes.Buffer{}, y: 1e6, faces: p.faces, chains: p.chains}
}

// faceFor is the first face in a style that has r, and the character it
// shows: "?" when none has it.
func (p *pdfLayout) faceFor(f pdfFont, r rune) (*face, rune) {
	for _, fc := range p.chains[f] {
		if fc.has(r) {
			return fc, r
		}
	}
	return p.chains[f][0], '?'
}

func (p *pdfLayout) newPage() {
	p.cur = &bytes.Buffer{}
	p.pages = append(p.pages, p.cur)
	p.y = pageH - marginTop
}

// room starts a new page when h does not fit.
func (p *pdfLayout) room(h float64) {
	if p.y-h < marginBottom {
		p.newPage()
	}
}

func (p *pdfLayout) textWidth(f pdfFont, size float64, s string) float64 {
	w := 0.0
	for _, r := range s {
		fc, r := p.faceFor(f, r)
		w += fc.width(r)
	}
	return w * size / 1000
}

// fit cuts s until it is no wider than width.
func (p *pdfLayout) fit(f pdfFont, size float64, s string, width float64) string {
	rs := []rune(s)
	for len(rs) > 1 && p.textWidth(f, size, string(rs)) > width {
		rs = rs[:len(rs)-1]
	}
	return string(rs)
}

func pdfString(b []byte, literal bool) string {
	if !literal {
		return "<" + hex.EncodeToString(b) + ">"
	}
	var s strings.Builder
	s.WriteByte('(')
	for _, c := range b {
		if c == '(' || c == ')' || c == '\\' {
			s.WriteByte('\\')
		}
		s.WriteByte(c)
	}
	s.WriteByte(')')
	return s.String()
}

// text writes s at x, y, each character in the first of the style's faces
// that has it. Italic is slanted; bold in a face without a bold weight is
// drawn with an outline.
func (p *pdfLayout) text(x, y float64, f pdfFont, size float64, s string) {
	p.write(p.cur, x, y, f, size, s)
}

func (p *pdfLayout) write(w *bytes.Buffer, x, y float64, f pdfFont, size float64, s string) {
	if f == fontItalic {
		fmt.Fprintf(w, "BT 1 0 0.2 1 %.2f %.2f Tm", x, y)
	} else {
		fmt.Fprintf(w, "BT %.2f %.2f Td", x, y)
	}
	var run []byte
	var cur *face
	flush := func() {
		if cur == nil || len(run) == 0 {
			return
		}
		fake := f == fontBold && !cur.hasBold
		if fake {
			fmt.Fprintf(w, " 2 Tr %.2f w", size*0.03)
		}
		fmt.Fprintf(w, " /F%d %.1f Tf %s Tj", cur.res, size, pdfString(run, cur.ttf == nil))
		if fake {
			w.WriteString(" 0 Tr")
		}
		run = run[:0]
	}
	for _, r := range s {
		fc, r := p.faceFor(f, r)
		if fc != cur {
			flush()
			cur = fc
		}
		run = fc.encode(run, r)
	}
	flush()
	w.WriteString(" ET\n")
}

type word struct {
	s     string
	font  pdfFont
	space bool // a space comes before it
}

// words splits spans into words that keep their font.
func words(spans []span, base pdfFont) []word {
	var out []word
	space := false
	for _, s := range spans {
		f := spanFont(s)
		if f == fontRegular {
			f = base
		}
		for _, part := range strings.SplitAfter(clean(s.Text), " ") {
			if part == "" {
				continue
			}
			trimmed := strings.TrimRight(part, " ")
			if trimmed != "" {
				out = append(out, word{s: trimmed, font: f, space: space})
			}
			space = strings.HasSuffix(part, " ")
		}
	}
	return out
}

// flow writes spans wrapped to width from x, one line every lead points.
func (p *pdfLayout) flow(spans []span, base pdfFont, size, lead, x, width float64) {
	ws := words(spans, base)
	for len(ws) > 0 {
		lineW := 0.0
		n := 0
		for n < len(ws) {
			w := p.textWidth(ws[n].font, size, ws[n].s)
			if n > 0 && ws[n].space {
				w += p.textWidth(ws[n].font, size, " ")
			}
			if n > 0 && lineW+w > width {
				break
			}
			lineW += w
			n++
		}
		p.room(lead)
		p.y -= lead
		// Words in the same font go out as one string with their spaces,
		// so the text can be copied and searched.
		cx := x
		var run strings.Builder
		runFont, runX := ws[0].font, x
		for i, w := range ws[:n] {
			// A single word wider than the line is cut to fit.
			s := p.fit(w.font, size, w.s, width)
			if i > 0 && w.space {
				// The space goes with the words before it, in their font.
				run.WriteByte(' ')
				cx += p.textWidth(runFont, size, " ")
			}
			if i > 0 && w.font != runFont {
				p.text(runX, p.y, runFont, size, run.String())
				run.Reset()
				runFont, runX = w.font, cx
			}
			run.WriteString(s)
			cx += p.textWidth(w.font, size, s)
		}
		if run.Len() > 0 {
			p.text(runX, p.y, runFont, size, run.String())
		}
		ws = ws[n:]
	}
}

func (p *pdfLayout) block(b block) {
	switch b.Kind {
	case blockHeading:
		size := map[int]float64{1: 20, 2: 16, 3: 13}[b.Level]
		p.room(size * 2.2)
		p.y -= size * 0.6
		p.flow(b.Spans, fontBold, size, size*1.25, marginX, contentW)
		p.y -= size * 0.3
	case blockParagraph:
		indent := 0.0
		base := fontRegular
		if b.Level > 0 {
			indent, base = 24, fontItalic
		}
		p.flow(b.Spans, base, 11, 15, marginX+indent, contentW-indent)
		p.y -= 6
	case blockBullet, blockNumbered:
		indent := 18 + 18*float64(b.Level)
		marker := "•"
		if b.Kind == blockNumbered {
			marker = fmt.Sprintf("%d.", b.Number)
		}
		p.room(15)
		top := p.y
		p.flow(b.Spans, fontRegular, 11, 15, marginX+indent, contentW-indent)
		if top > p.y { // marker on the item's first line
			p.text(marginX+indent-14, top-15, fontRegular, 11, marker)
		}
		p.y -= 2
	case blockCode:
		for _, line := range b.Lines {
			p.room(12)
			p.y -= 12
			p.text(marginX+8, p.y, fontMono, 9.5, p.fit(fontMono, 9.5, clean(line), contentW))
		}
		p.y -= 8
	case blockRule:
		p.room(14)
		p.y -= 7
		fmt.Fprintf(p.cur, "0.6 G 0.5 w %.2f %.2f m %.2f %.2f l S 0 G\n", marginX, p.y, marginX+contentW, p.y)
		p.y -= 7
	case blockTable:
		p.table(b.Rows)
	}
}

func (p *pdfLayout) table(rows [][][]span) {
	cols := 0
	for _, r := range rows {
		cols = max(cols, len(r))
	}
	if cols == 0 {
		return
	}
	colW := contentW / float64(cols)
	const size, lead, pad = 9.5, 12.5, 4.0
	p.y -= 8
	for i, row := range rows {
		base := fontRegular
		if i == 0 {
			base = fontBold
		}
		// Lay the row out on a scratch page to learn its height.
		heights := make([]float64, cols)
		for c := 0; c < cols && c < len(row); c++ {
			scratch := p.scratch()
			scratch.flow(row[c], base, size, lead, 0, colW-2*pad)
			heights[c] = 1e6 - scratch.y
		}
		h := lead
		for _, x := range heights {
			h = max(h, x)
		}
		h += 2 * pad
		p.room(h)
		top := p.y
		for c := 0; c < cols; c++ {
			x := marginX + float64(c)*colW
			fmt.Fprintf(p.cur, "0.7 G 0.5 w %.2f %.2f %.2f %.2f re S 0 G\n", x, top-h, colW, h)
			if c < len(row) {
				p.y = top - pad + 2
				p.flow(row[c], base, size, lead, x+pad, colW-2*pad)
			}
		}
		p.y = top - h
	}
	p.y -= 10
}

func deflate(b []byte) []byte {
	var out bytes.Buffer
	z := zlib.NewWriter(&out)
	_, _ = z.Write(b)
	_ = z.Close()
	return out.Bytes()
}

// finish writes the PDF file: catalog, page tree, fonts, and each page.
func (p *pdfLayout) finish() ([]byte, error) {
	// Page number at the foot of each page, when there is more than one.
	if len(p.pages) > 1 {
		for i, page := range p.pages {
			num := fmt.Sprintf("%d / %d", i+1, len(p.pages))
			page.WriteString("0.5 g ")
			p.write(page, pageW/2-p.textWidth(fontRegular, 8, num)/2, marginBottom/2, fontRegular, 8, num)
			page.WriteString("0 g\n")
		}
	}
	var objs []string
	add := func(body string) int {
		objs = append(objs, body)
		return len(objs)
	}
	stream := func(dict string, data []byte) int {
		z := deflate(data)
		return add(fmt.Sprintf("<< %s /Filter /FlateDecode /Length %d >>\nstream\n%s\nendstream", dict, len(z), z))
	}
	add("<< /Type /Catalog /Pages 2 0 R >>")
	pagesAt := add("") // written once the pages are
	var fonts strings.Builder
	for _, fc := range p.faces {
		if !fc.used {
			continue
		}
		var ref int
		if fc.ttf == nil {
			ref = add("<< /Type /Font /Subtype /Type1 /BaseFont /Courier /Encoding /WinAnsiEncoding >>")
		} else {
			var err error
			if ref, err = p.embed(fc, add, stream); err != nil {
				return nil, err
			}
		}
		fmt.Fprintf(&fonts, "/F%d %d 0 R ", fc.res, ref)
	}
	var kids []string
	for _, page := range p.pages {
		contents := stream("", page.Bytes())
		kids = append(kids, fmt.Sprintf("%d 0 R", add(fmt.Sprintf("<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /Font << %s>> >> /Contents %d 0 R >>",
			pagesAt, pageW, pageH, fonts.String(), contents))))
	}
	objs[pagesAt-1] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(p.pages))

	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(objs))
	for i, body := range objs {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, o := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return out.Bytes(), nil
}

// embed writes a TrueType face as a composite font of the glyphs used,
// numbered in the order they were first used, with each glyph's width and
// the character it shows, so its text can be copied. It returns the
// font's object.
func (p *pdfLayout) embed(fc *face, add func(string) int, stream func(string, []byte) int) (int, error) {
	f := fc.ttf
	file, _, err := f.subset(fc.gids)
	if err != nil {
		return 0, err
	}
	// A subset's name starts with six capital letters of its own.
	h := fnv.New32a()
	for _, g := range fc.gids {
		_, _ = h.Write([]byte{byte(g >> 8), byte(g)})
	}
	sum := h.Sum32()
	tag := make([]byte, 6)
	for i := range tag {
		tag[i] = 'A' + byte(sum%26)
		sum /= 26
	}
	name := string(tag) + "+" + f.name
	s := func(v int) int { return int(math.Round(float64(v) * fc.scale)) }

	fileRef := stream(fmt.Sprintf("/Length1 %d", len(file)), file)
	descriptor := add(fmt.Sprintf("<< /Type /FontDescriptor /FontName /%s /Flags 4 /FontBBox [%d %d %d %d] /ItalicAngle 0 /Ascent %d /Descent %d /CapHeight %d /StemV 80 /FontFile2 %d 0 R >>",
		name, s(f.bbox[0]), s(f.bbox[1]), s(f.bbox[2]), s(f.bbox[3]), s(f.ascent), s(f.descent), s(f.capHeight), fileRef))
	var widths strings.Builder
	for i, g := range fc.gids {
		if i > 0 {
			widths.WriteByte(' ')
		}
		fmt.Fprintf(&widths, "%d", s(f.advance(g)))
	}
	cid := add(fmt.Sprintf("<< /Type /Font /Subtype /CIDFontType2 /BaseFont /%s /CIDSystemInfo << /Registry (Adobe) /Ordering (Identity) /Supplement 0 >> /FontDescriptor %d 0 R /DW 0 /W [0 [%s]] /CIDToGIDMap /Identity >>",
		name, descriptor, widths.String()))
	toUnicode := stream("", toUnicodeCMap(fc.text))
	return add(fmt.Sprintf("<< /Type /Font /Subtype /Type0 /BaseFont /%s /Encoding /Identity-H /DescendantFonts [%d 0 R] /ToUnicode %d 0 R >>",
		name, cid, toUnicode)), nil
}

// toUnicodeCMap maps each glyph in the PDF back to its character.
func toUnicodeCMap(text map[uint16]rune) []byte {
	cids := make([]int, 0, len(text))
	for c := range text {
		cids = append(cids, int(c))
	}
	sort.Ints(cids)
	var b bytes.Buffer
	b.WriteString("/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n" +
		"/CIDSystemInfo << /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n" +
		"/CMapName /Adobe-Identity-UCS def\n/CMapType 2 def\n" +
		"1 begincodespacerange\n<0000> <FFFF>\nendcodespacerange\n")
	for len(cids) > 0 {
		n := min(len(cids), 100)
		fmt.Fprintf(&b, "%d beginbfchar\n", n)
		for _, c := range cids[:n] {
			fmt.Fprintf(&b, "<%04X> <", c)
			for _, u := range utf16.Encode([]rune{text[uint16(c)]}) {
				fmt.Fprintf(&b, "%04X", u)
			}
			b.WriteString(">\n")
		}
		b.WriteString("endbfchar\n")
		cids = cids[n:]
	}
	b.WriteString("endcmap\nCMapName currentdict /CMap defineresource pop\nend\nend\n")
	return b.Bytes()
}
