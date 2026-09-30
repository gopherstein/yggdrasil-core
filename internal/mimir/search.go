package mimir

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Hit is one retrieved passage.
type Hit struct {
	SourceID   string  `json:"source_id"`
	SourceName string  `json:"source_name"`
	Title      string  `json:"title"`
	Body       string  `json:"body"`
	Score      float64 `json:"score"`
}

// SearchInput queries connected knowledge.
type SearchInput struct {
	Query string `json:"query"`
	// SourceIDs limits the search. Empty searches every source.
	SourceIDs []string `json:"source_ids,omitempty"`
	Limit     int      `json:"limit,omitempty"`
}

var termRe = regexp.MustCompile(`[\p{L}\p{N}][\p{L}\p{N}/\-]*`)

var stopwords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true, "be": true, "but": true,
	"by": true, "can": true, "do": true, "does": true, "for": true, "from": true, "have": true,
	"how": true, "i": true, "if": true, "in": true, "is": true, "it": true, "me": true, "my": true,
	"of": true, "on": true, "or": true, "please": true, "should": true, "so": true, "tell": true,
	"that": true, "the": true, "there": true, "this": true, "to": true, "was": true, "we": true,
	"what": true, "when": true, "where": true, "which": true, "who": true, "why": true, "will": true,
	"with": true, "would": true, "you": true, "your": true, "any": true, "about": true, "our": true,
}

// matchQuery turns free text into an FTS5 query. Each term is quoted so
// punctuation in sizes and SKUs (225/45R17, AB-1002) is matched literally.
func matchQuery(q string) string {
	seen := map[string]bool{}
	var terms []string
	for _, t := range termRe.FindAllString(strings.ToLower(q), -1) {
		t = strings.Trim(t, "/-")
		if t == "" || stopwords[t] || seen[t] {
			continue
		}
		if utf8.RuneCountInString(t) < 2 && !strings.ContainsAny(t, "0123456789") {
			continue
		}
		seen[t] = true
		terms = append(terms, `"`+strings.ReplaceAll(t, `"`, `""`)+`"`)
		if len(terms) == 24 {
			break
		}
	}
	return strings.Join(terms, " OR ")
}

// Search returns the passages that best match the query. Sources whose files
// changed since they were indexed are refreshed first.
func (s *Store) Search(ctx context.Context, in SearchInput) ([]Hit, error) {
	limit := in.Limit
	if limit <= 0 || limit > 50 {
		limit = 6
	}
	match := matchQuery(in.Query)
	if match == "" {
		return []Hit{}, nil
	}
	ids := in.SourceIDs
	if len(ids) == 0 {
		all, err := s.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, src := range all {
			ids = append(ids, src.ID)
		}
	}
	if len(ids) == 0 {
		return []Hit{}, nil
	}
	s.refreshIfChanged(ctx, ids)

	args := []any{match}
	marks := make([]string, len(ids))
	for i, id := range ids {
		marks[i] = "?"
		args = append(args, id)
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT f.source_id, COALESCE(src.name, ''), f.title, f.body, bm25(knowledge_fts, 0.5, 1.0) AS score
		FROM knowledge_fts f
		LEFT JOIN knowledge_sources src ON src.id = f.source_id
		WHERE knowledge_fts MATCH ? AND f.source_id IN (%s)
		ORDER BY score LIMIT ?`, strings.Join(marks, ",")), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Hit{}
	for rows.Next() {
		var h Hit
		if err := rows.Scan(&h.SourceID, &h.SourceName, &h.Title, &h.Body, &h.Score); err != nil {
			return nil, err
		}
		// bm25 is lower-is-better and negative; report higher-is-better.
		h.Score = -h.Score
		out = append(out, h)
	}
	return out, rows.Err()
}

// ContextBudgetRunes caps how much retrieved text one turn receives.
const ContextBudgetRunes = 6000

// ContextBlock formats hits as instructions for the model. It returns "" when
// there is nothing to add.
func ContextBlock(hits []Hit, budget int) string {
	if len(hits) == 0 {
		return ""
	}
	if budget <= 0 {
		budget = ContextBudgetRunes
	}
	var b strings.Builder
	b.WriteString("Connected knowledge. These passages are current and authoritative. ")
	b.WriteString("Prefer them over what you remember for facts such as prices, stock, specifications, and policies. ")
	b.WriteString("If they do not answer the question, say so instead of guessing.\n")
	used := 0
	for i, h := range hits {
		entry := fmt.Sprintf("\n[%d] %s\n%s\n", i+1, h.Title, strings.TrimSpace(h.Body))
		n := utf8.RuneCountInString(entry)
		if used+n > budget {
			if used == 0 {
				r := []rune(entry)
				b.WriteString(string(r[:budget]))
			}
			break
		}
		b.WriteString(entry)
		used += n
	}
	return b.String()
}
