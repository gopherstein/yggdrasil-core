package mimir

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Word documents and PowerPoint decks (#510) are zip files of XML; only
// their text is read: a document's paragraphs, and each slide's.

// maxOfficePart is the most of one XML part read, so a hostile file can't
// unzip into gigabytes.
const maxOfficePart = 64 << 20

func officePart(z *zip.Reader, name string) ([]byte, error) {
	for _, f := range z.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		data, err := io.ReadAll(io.LimitReader(rc, maxOfficePart+1))
		if err != nil {
			return nil, err
		}
		if len(data) > maxOfficePart {
			return nil, fmt.Errorf("%s is too large to read", name)
		}
		return data, nil
	}
	return nil, errNoPart
}

var errNoPart = errors.New("part not found")

// paragraphs reads the text of each paragraph in a Word or PowerPoint XML
// part: runs (w:t, a:t) joined, tabs and line breaks kept, one line each.
func paragraphs(data []byte, paragraph, text string) []string {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var out []string
	var cur strings.Builder
	inText := false
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case text:
				inText = true
			case "tab":
				cur.WriteString("\t")
			case "br", "cr":
				cur.WriteString("\n")
			}
		case xml.EndElement:
			switch t.Name.Local {
			case text:
				inText = false
			case paragraph:
				if line := strings.TrimRight(cur.String(), " \t"); strings.TrimSpace(line) != "" {
					out = append(out, line)
				}
				cur.Reset()
			}
		case xml.CharData:
			if inText {
				cur.Write(t)
			}
		}
	}
	return out
}

// parseDOCX reads a Word document's text, paragraph by paragraph.
func parseDOCX(name string, raw []byte) ([]document, error) {
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("not a Word document: %w", err)
	}
	data, err := officePart(z, "word/document.xml")
	if err != nil {
		return nil, fmt.Errorf("not a Word document: %w", err)
	}
	text := strings.Join(paragraphs(data, "p", "t"), "\n")
	if strings.TrimSpace(text) == "" {
		return nil, errNoWords
	}
	return []document{{Name: name, Text: text}}, nil
}

var slideRe = regexp.MustCompile(`^ppt/slides/slide(\d+)\.xml$`)

// parsePPTX reads each slide's text, in order, one document a slide.
func parsePPTX(name string, raw []byte) ([]document, error) {
	z, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("not a PowerPoint deck: %w", err)
	}
	type slide struct {
		n    int
		path string
	}
	var slides []slide
	for _, f := range z.File {
		if m := slideRe.FindStringSubmatch(path.Clean(f.Name)); m != nil {
			n, _ := strconv.Atoi(m[1])
			slides = append(slides, slide{n, f.Name})
		}
	}
	if len(slides) == 0 {
		return nil, errors.New("not a PowerPoint deck: it has no slides")
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].n < slides[j].n })
	var docs []document
	for _, s := range slides {
		data, err := officePart(z, s.path)
		if err != nil {
			return nil, err
		}
		if text := strings.Join(paragraphs(data, "p", "t"), "\n"); strings.TrimSpace(text) != "" {
			docs = append(docs, document{Name: fmt.Sprintf("%s slide %d", name, s.n), Text: text})
		}
	}
	if len(docs) == 0 {
		return nil, errNoWords
	}
	return docs, nil
}

// errNoWords is a document or deck with no text, such as one of pictures.
var errNoWords = errors.New("it has no text to read")
