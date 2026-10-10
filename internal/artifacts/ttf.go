package artifacts

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
)

// A TrueType font, read far enough to lay text out in it and to embed the
// glyphs a PDF uses (#510): its character map, advance widths, and glyph
// outlines. Fonts with CFF outlines aren't read.

type ttfFont struct {
	name        string // PostScript name, such as NotoSans-Regular
	tables      map[string][]byte
	unitsPerEm  int
	numGlyphs   int
	numHMetrics int
	cmap        map[rune]uint16
	loca        []uint32
	ascent      int
	descent     int
	capHeight   int
	bbox        [4]int
}

var errFont = errors.New("not a TrueType font")

func u16(b []byte, off int) int { return int(binary.BigEndian.Uint16(b[off:])) }
func i16(b []byte, off int) int { return int(int16(binary.BigEndian.Uint16(b[off:]))) }
func u32(b []byte, off int) int { return int(binary.BigEndian.Uint32(b[off:])) }

// parseTTF reads a font file.
func parseTTF(data []byte) (*ttfFont, error) {
	if len(data) < 12 || (u32(data, 0) != 0x00010000 && string(data[:4]) != "true") {
		return nil, errFont
	}
	n := u16(data, 4)
	if len(data) < 12+16*n {
		return nil, errFont
	}
	f := &ttfFont{tables: map[string][]byte{}}
	for i := range n {
		rec := data[12+16*i:]
		off, length := u32(rec, 8), u32(rec, 12)
		if off < 0 || length < 0 || off+length > len(data) || off+length < off {
			return nil, fmt.Errorf("%w: table %q is out of bounds", errFont, rec[:4])
		}
		f.tables[string(rec[:4])] = data[off : off+length]
	}
	for _, tag := range []string{"head", "hhea", "maxp", "hmtx", "loca", "glyf", "cmap"} {
		if f.tables[tag] == nil {
			return nil, fmt.Errorf("%w: it has no %s table", errFont, tag)
		}
	}
	head, hhea, maxp := f.tables["head"], f.tables["hhea"], f.tables["maxp"]
	if len(head) < 54 || len(hhea) < 36 || len(maxp) < 6 {
		return nil, errFont
	}
	f.unitsPerEm = u16(head, 18)
	f.bbox = [4]int{i16(head, 36), i16(head, 38), i16(head, 40), i16(head, 42)}
	f.ascent, f.descent = i16(hhea, 4), i16(hhea, 6)
	f.capHeight = f.ascent * 7 / 10
	if os2 := f.tables["OS/2"]; len(os2) >= 90 && u16(os2, 0) >= 2 {
		f.capHeight = i16(os2, 88)
	}
	f.numHMetrics = u16(hhea, 34)
	f.numGlyphs = u16(maxp, 4)
	if f.unitsPerEm == 0 || f.numHMetrics == 0 || f.numHMetrics > f.numGlyphs ||
		len(f.tables["hmtx"]) < 4*f.numHMetrics+2*(f.numGlyphs-f.numHMetrics) {
		return nil, errFont
	}
	loca := f.tables["loca"]
	f.loca = make([]uint32, f.numGlyphs+1)
	long := i16(head, 50) == 1
	if (long && len(loca) < 4*(f.numGlyphs+1)) || (!long && len(loca) < 2*(f.numGlyphs+1)) {
		return nil, errFont
	}
	glyf := len(f.tables["glyf"])
	for i := range f.loca {
		if long {
			f.loca[i] = uint32(u32(loca, 4*i))
		} else {
			f.loca[i] = uint32(2 * u16(loca, 2*i))
		}
		if int(f.loca[i]) > glyf || (i > 0 && f.loca[i] < f.loca[i-1]) {
			return nil, fmt.Errorf("%w: glyph %d is out of bounds", errFont, i)
		}
	}
	var err error
	if f.cmap, err = parseCmap(f.tables["cmap"], f.numGlyphs); err != nil {
		return nil, err
	}
	f.name = postScriptName(f.tables["name"])
	return f, nil
}

// parseCmap reads the Unicode character map: format 12 for every plane,
// else format 4 for the first.
func parseCmap(t []byte, numGlyphs int) (map[rune]uint16, error) {
	if len(t) < 4 {
		return nil, errFont
	}
	best, bestFormat := -1, 0
	for i := range u16(t, 2) {
		if 4+8*i+8 > len(t) {
			return nil, errFont
		}
		platform, encoding, off := u16(t, 4+8*i), u16(t, 6+8*i), u32(t, 8+8*i)
		unicode := platform == 0 || (platform == 3 && (encoding == 1 || encoding == 10))
		if off+2 > len(t) || !unicode {
			continue
		}
		if format := u16(t, off); (format == 12 || format == 4) && format > bestFormat {
			best, bestFormat = off, format
		}
	}
	if best < 0 {
		return nil, fmt.Errorf("%w: it has no Unicode character map", errFont)
	}
	m := map[rune]uint16{}
	add := func(r rune, g int) {
		if g > 0 && g < numGlyphs {
			m[r] = uint16(g)
		}
	}
	s := t[best:]
	if bestFormat == 12 {
		if len(s) < 16 {
			return nil, errFont
		}
		groups := u32(s, 12)
		if groups < 0 || 16+12*groups > len(s) {
			return nil, errFont
		}
		for i := range groups {
			start, end, g := u32(s, 16+12*i), u32(s, 20+12*i), u32(s, 24+12*i)
			if end < start || end > 0x10FFFF {
				continue
			}
			for c := start; c <= end; c++ {
				add(rune(c), g+c-start)
			}
		}
		return m, nil
	}
	if len(s) < 14 {
		return nil, errFont
	}
	segs := u16(s, 6) / 2
	if 16+8*segs > len(s) {
		return nil, errFont
	}
	ends, starts, deltas, offsets := 14, 16+2*segs, 16+4*segs, 16+6*segs
	for i := range segs {
		end, start, delta, ro := u16(s, ends+2*i), u16(s, starts+2*i), u16(s, deltas+2*i), u16(s, offsets+2*i)
		for c := start; c <= end && c != 0xFFFF; c++ {
			if ro == 0 {
				add(rune(c), (c+delta)&0xFFFF)
				continue
			}
			at := offsets + 2*i + ro + 2*(c-start)
			if at+2 > len(s) {
				break
			}
			if g := u16(s, at); g != 0 {
				add(rune(c), (g+delta)&0xFFFF)
			}
		}
	}
	return m, nil
}

// postScriptName is name ID 6, kept to the letters a PDF name allows.
func postScriptName(t []byte) string {
	if len(t) < 6 {
		return "Font"
	}
	count, strs := u16(t, 2), u16(t, 4)
	for i := range count {
		rec := 6 + 12*i
		if rec+12 > len(t) {
			break
		}
		platform, id, length, off := u16(t, rec), u16(t, rec+6), u16(t, rec+8), u16(t, rec+10)
		if id != 6 || strs+off+length > len(t) {
			continue
		}
		raw := t[strs+off : strs+off+length]
		var name []byte
		for j := 0; j < len(raw); j++ {
			c := raw[j]
			if platform == 3 || platform == 0 { // UTF-16BE
				if j%2 == 0 {
					continue
				}
			}
			if c > ' ' && c < 127 && c != '/' && c != '(' && c != ')' && c != '[' && c != ']' && c != '<' && c != '>' && c != '{' && c != '}' && c != '%' {
				name = append(name, c)
			}
		}
		if len(name) > 0 {
			return string(name)
		}
	}
	return "Font"
}

// advance is a glyph's advance width, in font units.
func (f *ttfFont) advance(g uint16) int {
	hmtx := f.tables["hmtx"]
	if int(g) >= f.numHMetrics {
		return u16(hmtx, 4*(f.numHMetrics-1))
	}
	return u16(hmtx, 4*int(g))
}

func (f *ttfFont) lsb(g uint16) int {
	hmtx := f.tables["hmtx"]
	if int(g) < f.numHMetrics {
		return u16(hmtx, 4*int(g)+2)
	}
	return u16(hmtx, 4*f.numHMetrics+2*(int(g)-f.numHMetrics))
}

func (f *ttfFont) glyph(g uint16) []byte {
	return f.tables["glyf"][f.loca[g]:f.loca[g+1]]
}

// Composite glyph flags.
const (
	argsAreWords   = 0x0001
	haveScale      = 0x0008
	moreComponents = 0x0020
	haveXYScale    = 0x0040
	haveTwoByTwo   = 0x0080
)

// components calls fn with the offset of each component's glyph index in
// a composite glyph.
func components(g []byte, fn func(at int)) {
	if len(g) < 10 || i16(g, 0) >= 0 {
		return
	}
	at := 10
	for at+4 <= len(g) {
		flags := u16(g, at)
		fn(at + 2)
		at += 4
		if flags&argsAreWords != 0 {
			at += 4
		} else {
			at += 2
		}
		switch {
		case flags&haveScale != 0:
			at += 2
		case flags&haveXYScale != 0:
			at += 4
		case flags&haveTwoByTwo != 0:
			at += 8
		}
		if flags&moreComponents == 0 {
			return
		}
	}
}

// subset writes a font of only the glyphs in gids, in that order, so glyph
// i of the new font is gids[i]; gids[0] must be 0, the missing glyph. The
// glyphs composites are built from are added after them. It returns the
// font and the full list of glyphs it holds.
func (f *ttfFont) subset(gids []uint16) ([]byte, []uint16, error) {
	order := append([]uint16(nil), gids...)
	index := map[uint16]int{}
	for i, g := range order {
		index[g] = i
	}
	var glyf []byte
	loca := []uint32{0}
	for i := 0; i < len(order); i++ {
		g := order[i]
		if int(g) >= f.numGlyphs {
			return nil, nil, fmt.Errorf("glyph %d is not in the font", g)
		}
		data := append([]byte(nil), f.glyph(g)...)
		components(data, func(at int) {
			if at+2 > len(data) {
				return
			}
			c := uint16(u16(data, at))
			if int(c) >= f.numGlyphs {
				c = 0
			}
			n, ok := index[c]
			if !ok {
				n = len(order)
				index[c] = n
				order = append(order, c)
			}
			binary.BigEndian.PutUint16(data[at:], uint16(n))
		})
		glyf = append(glyf, data...)
		for len(glyf)%4 != 0 {
			glyf = append(glyf, 0)
		}
		loca = append(loca, uint32(len(glyf)))
	}
	n := len(order)
	if n > 0xFFFF {
		return nil, nil, errors.New("too many glyphs")
	}
	tables := map[string][]byte{"glyf": glyf}
	locaT := make([]byte, 4*len(loca))
	for i, o := range loca {
		binary.BigEndian.PutUint32(locaT[4*i:], o)
	}
	tables["loca"] = locaT
	hmtx := make([]byte, 4*n)
	for i, g := range order {
		binary.BigEndian.PutUint16(hmtx[4*i:], uint16(f.advance(g)))
		binary.BigEndian.PutUint16(hmtx[4*i+2:], uint16(f.lsb(g)))
	}
	tables["hmtx"] = hmtx
	head := append([]byte(nil), f.tables["head"]...)
	binary.BigEndian.PutUint32(head[8:], 0)  // checkSumAdjustment, set below
	binary.BigEndian.PutUint16(head[50:], 1) // long offsets in loca
	tables["head"] = head
	hhea := append([]byte(nil), f.tables["hhea"]...)
	binary.BigEndian.PutUint16(hhea[34:], uint16(n))
	tables["hhea"] = hhea
	maxp := append([]byte(nil), f.tables["maxp"]...)
	binary.BigEndian.PutUint16(maxp[4:], uint16(n))
	tables["maxp"] = maxp
	// A format 3 post table names no glyphs; an empty format 4 cmap maps no
	// characters, since the PDF picks glyphs by number.
	post := make([]byte, 32)
	binary.BigEndian.PutUint32(post, 0x00030000)
	if p := f.tables["post"]; len(p) >= 32 {
		copy(post[4:16], p[4:16]) // italic angle, underline
	}
	tables["post"] = post
	tables["cmap"] = []byte{0, 0, 0, 1, 0, 3, 0, 1, 0, 0, 0, 12,
		0, 4, 0, 24, 0, 0, 0, 2, 0, 2, 0, 0, 0, 0, 0xFF, 0xFF, 0, 0, 0xFF, 0xFF, 0, 1, 0, 0}
	for _, tag := range []string{"OS/2", "name", "cvt ", "fpgm", "prep"} {
		if t := f.tables[tag]; t != nil {
			tables[tag] = t
		}
	}
	return writeSFNT(tables), order, nil
}

func checksum(b []byte) uint32 {
	var sum uint32
	for i := 0; i < len(b); i += 4 {
		var w [4]byte
		copy(w[:], b[i:])
		sum += binary.BigEndian.Uint32(w[:])
	}
	return sum
}

// writeSFNT writes tables as a font file, with its checksums.
func writeSFNT(tables map[string][]byte) []byte {
	tags := make([]string, 0, len(tables))
	for tag := range tables {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	n := len(tags)
	entry, pow := 0, 1
	for pow*2 <= n {
		pow *= 2
		entry++
	}
	out := make([]byte, 12+16*n)
	binary.BigEndian.PutUint32(out, 0x00010000)
	binary.BigEndian.PutUint16(out[4:], uint16(n))
	binary.BigEndian.PutUint16(out[6:], uint16(pow*16))
	binary.BigEndian.PutUint16(out[8:], uint16(entry))
	binary.BigEndian.PutUint16(out[10:], uint16(n*16-pow*16))
	headAt := 0
	for i, tag := range tags {
		t := tables[tag]
		rec := out[12+16*i:]
		copy(rec, tag)
		binary.BigEndian.PutUint32(rec[4:], checksum(t))
		binary.BigEndian.PutUint32(rec[8:], uint32(len(out)))
		binary.BigEndian.PutUint32(rec[12:], uint32(len(t)))
		if tag == "head" {
			headAt = len(out)
		}
		out = append(out, t...)
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
	}
	binary.BigEndian.PutUint32(out[headAt+8:], 0xB1B0AFBA-checksum(out))
	return out
}
