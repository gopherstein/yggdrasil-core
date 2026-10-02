package muninn

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"sort"
)

// Cross-language memory (multilingual spec §18). Words don't cross
// languages: "proyecto" never matches "project". When an embedding model is
// installed, each memory also gets a vector of its meaning, and a question
// finds memories by meaning as well as by words, whatever language either is
// written in. Without one, memory is found by words, as before.

// Embedder turns memories and questions into vectors. Mimir's embedder,
// from the installed embedding model, is one.
type Embedder interface {
	// ModelID names the embedding model; vectors from different models are
	// never compared.
	ModelID() string
	EmbedDocuments(ctx context.Context, texts []string) ([][]float32, error)
	EmbedQuery(ctx context.Context, text string) ([]float32, error)
}

// EmbedderSource finds the embedding model to use now, or returns nil when
// none is installed or it cannot run right now.
type EmbedderSource func(ctx context.Context) (Embedder, error)

// SetEmbedder lets memory be found by meaning. nil keeps it to words.
func (s *Store) SetEmbedder(src EmbedderSource) {
	s.mu.Lock()
	s.embedder = src
	s.mu.Unlock()
}

const (
	// semanticFloor is how similar in meaning a memory must be to a question
	// to come with it, as for knowledge passages.
	semanticFloor = 0.5
	// embedBatch caps the memories embedded while answering one message.
	embedBatch = 64
)

func contentHash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:16])
}

// encode normalizes a vector and packs it as little-endian float32s.
func encode(v []float32) []byte {
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	if norm == 0 {
		return nil
	}
	norm = math.Sqrt(norm)
	b := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(float32(float64(x)/norm)))
	}
	return b
}

func decode(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return v
}

func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
	}
	return dot
}

// vectors returns each memory's vector from emb's model, embedding the
// memories that have none yet or have changed since.
func (s *Store) vectors(ctx context.Context, emb Embedder, memories []Memory) (map[string][]float32, error) {
	model := emb.ModelID()
	stored := map[string][]float32{}
	fresh := map[string]bool{}
	rows, err := s.db.QueryContext(ctx, `SELECT memory_id, model_id, content_hash, vector FROM memory_vectors`)
	if err != nil {
		return nil, err
	}
	hashes := map[string]string{}
	for _, m := range memories {
		hashes[m.ID] = contentHash(m.Content)
	}
	for rows.Next() {
		var id, mid, hash string
		var blob []byte
		if err := rows.Scan(&id, &mid, &hash, &blob); err != nil {
			rows.Close()
			return nil, err
		}
		if mid == model && hash == hashes[id] {
			stored[id] = decode(blob)
			fresh[id] = true
		}
	}
	rows.Close()
	var missing []Memory
	for _, m := range memories {
		if !fresh[m.ID] && len(missing) < embedBatch {
			missing = append(missing, m)
		}
	}
	if len(missing) == 0 {
		return stored, nil
	}
	texts := make([]string, len(missing))
	for i, m := range missing {
		texts[i] = m.Content
	}
	vecs, err := emb.EmbedDocuments(ctx, texts)
	if err != nil || len(vecs) != len(missing) {
		return stored, err
	}
	for i, m := range missing {
		blob := encode(vecs[i])
		if blob == nil {
			continue
		}
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO memory_vectors (memory_id, model_id, content_hash, vector) VALUES (?, ?, ?, ?)
			ON CONFLICT(memory_id) DO UPDATE SET model_id = excluded.model_id, content_hash = excluded.content_hash, vector = excluded.vector`,
			m.ID, model, hashes[m.ID], blob); err != nil {
			return stored, err
		}
		stored[m.ID] = decode(blob)
	}
	return stored, nil
}

// byMeaning returns the enabled memories closest in meaning to message,
// most similar first, or nil when memory can't be searched by meaning now.
func (s *Store) byMeaning(ctx context.Context, message string, all []Memory) []Memory {
	s.mu.Lock()
	src := s.embedder
	s.mu.Unlock()
	if src == nil {
		return nil
	}
	emb, err := src(ctx)
	if err != nil || emb == nil {
		return nil
	}
	var enabled []Memory
	for _, m := range all {
		if m.Enabled {
			enabled = append(enabled, m)
		}
	}
	if len(enabled) == 0 {
		return nil
	}
	vecs, err := s.vectors(ctx, emb, enabled)
	if err != nil || len(vecs) == 0 {
		return nil
	}
	q, err := emb.EmbedQuery(ctx, message)
	if err != nil {
		return nil
	}
	qn := decode(encode(q))
	type scored struct {
		m     Memory
		score float64
	}
	var hits []scored
	for _, m := range enabled {
		if v, ok := vecs[m.ID]; ok {
			if sc := cosine(qn, v); sc >= semanticFloor {
				hits = append(hits, scored{m, sc})
			}
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	out := make([]Memory, len(hits))
	for i, h := range hits {
		out[i] = h.m
	}
	return out
}
