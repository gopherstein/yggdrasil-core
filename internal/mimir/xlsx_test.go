package mimir

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestXLSXFromOpenpyxl(t *testing.T) {
	raw, err := os.ReadFile("testdata/inventory.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	docs, err := parseXLSX("inventory.xlsx", raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 || docs[0].Name != "inventory.xlsx › Tires" || docs[1].Name != "inventory.xlsx › Services" {
		t.Fatalf("docs = %+v", docs)
	}
	chunks := chunkDocuments(docs[:1])
	// openpyxl saves formulas without a computed value, so Value is empty.
	if len(chunks) != 3 || chunks[0].Body != "SKU: MP-22545; Brand: Michelin; Size: 225/45R17; Price: 189.99; In stock: 12" {
		t.Fatalf("chunks = %+v", chunks)
	}
	// A sparse row keeps columns aligned.
	if chunks[2].Body != "SKU: GA-21560; Size: 215/60R16; Price: 156" {
		t.Fatalf("sparse row = %q", chunks[2].Body)
	}
}

// excelStyle builds a workbook the way Excel writes one: shared strings,
// rich text runs, cached formula values, and booleans.
func excelStyle(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name, body string) {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(body))
	}
	add("xl/workbook.xml", `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Parts" sheetId="1" r:id="rId7"/></sheets></workbook>`)
	add("xl/_rels/workbook.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId7" Target="worksheets/data.xml"/></Relationships>`)
	add("xl/sharedStrings.xml", `<sst><si><t>Part</t></si><si><t>Total</t></si><si><t>Active</t></si><si><r><t>Valve </t></r><r><t>stem</t></r></si></sst>`)
	add("xl/worksheets/data.xml", `<worksheet><sheetData>
		<row r="1"/>
		<row r="2"><c r="A2" t="s"><v>0</v></c><c r="B2" t="s"><v>1</v></c><c r="C2" t="s"><v>2</v></c></row>
		<row r="3"><c r="A3" t="s"><v>3</v></c><c r="B3"><f>2*3</f><v>6</v></c><c r="C3" t="b"><v>1</v></c></row>
	</sheetData></worksheet>`)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestXLSXExcelLayout(t *testing.T) {
	docs, err := parseXLSX("parts.xlsx", excelStyle(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 || docs[0].Name != "parts.xlsx" {
		t.Fatalf("docs = %+v", docs)
	}
	got := chunkDocuments(docs)
	if len(got) != 1 || got[0].Body != "Part: Valve stem; Total: 6; Active: TRUE" {
		t.Fatalf("chunks = %+v", got)
	}
}

func TestXLSXRejectsNonWorkbooks(t *testing.T) {
	if _, err := parseXLSX("x.xlsx", []byte("not a zip")); err == nil || !strings.Contains(err.Error(), "not an .xlsx") {
		t.Fatalf("err = %v", err)
	}
}

func TestXLSXFolderSourceIsSearchable(t *testing.T) {
	s := newTestStore(t)
	dir := t.TempDir()
	raw, _ := os.ReadFile("testdata/inventory.xlsx")
	if err := os.WriteFile(filepath.Join(dir, "inventory.xlsx"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := s.Create(ctx, CreateInput{Kind: KindPath, Path: dir}); err != nil {
		t.Fatal(err)
	}
	hits, err := s.Search(ctx, SearchInput{Query: "alignment price"})
	if err != nil || len(hits) == 0 || !strings.Contains(hits[0].Body, "Alignment; Price: 99") {
		t.Fatalf("hits = %+v %v", hits, err)
	}
}

func TestColumnIndex(t *testing.T) {
	for ref, want := range map[string]int{"A1": 0, "Z9": 25, "AA3": 26, "AB12": 27} {
		if got := columnIndex(ref); got != want {
			t.Errorf("%s = %d, want %d", ref, got, want)
		}
	}
}
