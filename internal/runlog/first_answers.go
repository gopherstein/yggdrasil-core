package runlog

import (
	"context"
	"encoding/json"
	"sort"
	"time"
)

// FirstAnswers is how long people waited for the first word of an answer
// (#498), by the app they asked from and whether a model had to load first.
type FirstAnswers struct {
	Since  time.Time           `json:"since"`
	Groups []FirstAnswersGroup `json:"groups"`
}

// FirstAnswersGroup is one app's answers, cold (a model loaded first) or
// warm (every model was already running).
type FirstAnswersGroup struct {
	// Client is as in Run; empty for requests that didn't say.
	Client string `json:"client"`
	Cold   bool   `json:"cold"`
	Runs   int    `json:"runs"`
	// Remote counts the runs that came from outside the home network.
	Remote int `json:"remote,omitempty"`
	// FirstWordMs is the median wait from the request to the first token;
	// FirstWordP90Ms is the wait nine in ten answers beat.
	FirstWordMs    float64 `json:"first_word_ms"`
	FirstWordP90Ms float64 `json:"first_word_p90_ms"`
	// LoadMs is the median time spent loading models; 0 when warm.
	LoadMs float64 `json:"load_ms,omitempty"`
}

// firstAnswersLimit caps how many runs a summary reads.
const firstAnswersLimit = 5000

// Since returns the runs people asked for (not automations) started at or
// after t, newest first, at most firstAnswersLimit of them.
func (s *Store) Since(ctx context.Context, t time.Time) ([]Run, error) {
	if s == nil || s.db == nil {
		return []Run{}, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT detail_json FROM runs WHERE started_at >= ? AND source != 'automation' ORDER BY started_at DESC LIMIT ?`,
		t.UTC().Format(time.RFC3339Nano), firstAnswersLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Run{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var r Run
		if json.Unmarshal([]byte(raw), &r) == nil {
			out = append(out, r)
		}
	}
	return out, rows.Err()
}

// SummarizeFirstAnswers groups finished runs that reached a first token by
// client and cold or warm, ordered by client then warm before cold.
func SummarizeFirstAnswers(since time.Time, runs []Run) FirstAnswers {
	type groupKey struct {
		client string
		cold   bool
	}
	type samples struct {
		first, load []float64
		remote      int
	}
	by := map[groupKey]*samples{}
	for _, r := range runs {
		if r.Status != StatusCompleted {
			continue
		}
		first, load := firstWord(r)
		if first <= 0 {
			continue
		}
		k := groupKey{r.Client, load > 0}
		g := by[k]
		if g == nil {
			g = &samples{}
			by[k] = g
		}
		g.first = append(g.first, first)
		if load > 0 {
			g.load = append(g.load, load)
		}
		if r.Remote {
			g.remote++
		}
	}
	out := FirstAnswers{Since: since.UTC(), Groups: []FirstAnswersGroup{}}
	for k, g := range by {
		out.Groups = append(out.Groups, FirstAnswersGroup{
			Client: k.client, Cold: k.cold, Runs: len(g.first), Remote: g.remote,
			FirstWordMs: percentile(g.first, 50), FirstWordP90Ms: percentile(g.first, 90), LoadMs: percentile(g.load, 50),
		})
	}
	sort.Slice(out.Groups, func(i, j int) bool {
		a, b := out.Groups[i], out.Groups[j]
		if a.Client != b.Client {
			return a.Client < b.Client
		}
		return !a.Cold && b.Cold
	})
	return out
}

// firstWord is a run's wait for its first token, the pipeline before the
// first model call plus that call's first token, and the time its models
// spent loading. A run that never reached a token has 0.
func firstWord(r Run) (first, load float64) {
	for _, m := range r.Models {
		load += m.LoadMs
		if m.FirstTokenMs > 0 && (first == 0 || m.FirstTokenMs < first) {
			first = m.FirstTokenMs
		}
	}
	if first == 0 {
		return 0, load
	}
	return r.PipelineMs + first, load
}

// percentile is the p-th percentile of xs by nearest rank, or 0 for none.
func percentile(xs []float64, p int) float64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	i := (p*len(s) + 99) / 100
	if i < 1 {
		i = 1
	}
	return s[i-1]
}
