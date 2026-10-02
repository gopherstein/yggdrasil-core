package mimir

import (
	"context"
	"strings"
	"testing"
)

const politicas = `# Políticas de la tienda

## Garantía

Cada neumático que vendemos está cubierto durante cinco años contra defectos de fabricación.

## Devoluciones

Los artículos sin usar se pueden devolver dentro de los 30 días para un reembolso completo.
`

// A source records the languages it is written in (multilingual spec §19).
func TestSourcesRecordTheirLanguages(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	en, err := s.Create(ctx, CreateInput{Kind: KindText, Filename: "policies.md", Text: policies})
	if err != nil {
		t.Fatal(err)
	}
	es, err := s.Create(ctx, CreateInput{Kind: KindText, Filename: "politicas.md", Text: politicas})
	if err != nil {
		t.Fatal(err)
	}
	en, _ = s.Get(ctx, en.ID)
	es, _ = s.Get(ctx, es.ID)
	if en.Language != "en" || es.Language != "es" {
		t.Fatalf("languages = %q, %q (%+v, %+v)", en.Language, es.Language, en.Languages, es.Languages)
	}
	if len(es.Languages) == 0 || es.Languages[0].Passages == 0 {
		t.Fatalf("Spanish passages = %+v", es.Languages)
	}
}

// A question in Spanish finds an English passage by meaning (§19).
func TestQuestionFindsPassageInAnotherLanguage(t *testing.T) {
	ctx := context.Background()
	s, _ := semanticStore(t)
	src, err := s.Create(ctx, CreateInput{Kind: KindText, Filename: "policies.md", Text: policies})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.EmbedPending(ctx, nil); err != nil {
		t.Fatal(err)
	}
	hits, err := s.Search(ctx, SearchInput{Query: "¿Qué garantía tienen los neumáticos?", SourceIDs: []string{src.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) == 0 || !strings.Contains(hits[0].Body, "five years") {
		t.Fatalf("want the English guarantee passage; got %+v", hits)
	}
}
