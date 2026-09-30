package mimir

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

// maxPDFPages caps the pages read from one file.
const maxPDFPages = 2000

// PDFText returns a PDF's text, pages separated by blank lines, for the
// training classifier.
func PDFText(name string, raw []byte) (string, error) {
	docs, err := parsePDF(name, raw)
	if err != nil {
		return "", err
	}
	parts := make([]string, len(docs))
	for i, d := range docs {
		parts[i] = d.Text
	}
	return strings.Join(parts, "\n\n"), nil
}

// parsePDF extracts the text of each page. A page becomes its own document so
// retrieved passages cite the page they came from. Scanned PDFs without a
// text layer have nothing to read and are reported as an error.
func parsePDF(name string, raw []byte) (docs []document, err error) {
	// The parser panics on some malformed files; report those as errors.
	defer func() {
		if r := recover(); r != nil {
			docs, err = nil, fmt.Errorf("could not read the PDF: %v", r)
		}
	}()
	r, err := pdf.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, fmt.Errorf("not a readable PDF: %w", err)
	}
	pages := r.NumPage()
	if pages > maxPDFPages {
		return nil, fmt.Errorf("more than %d pages", maxPDFPages)
	}
	for i := 1; i <= pages; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", i, err)
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		docs = append(docs, document{Name: fmt.Sprintf("%s p.%d", name, i), Text: text})
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("no text found. Scanned PDFs need text recognition (OCR) before Yggdrasil can read them")
	}
	return docs, nil
}
