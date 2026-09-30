package training

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Repo persists specialized AIs and their training state.
type Repo struct {
	db  *sql.DB
	now func() time.Time
}

// NewRepo returns a repository over the daemon database.
func NewRepo(db *sql.DB) *Repo { return &Repo{db: db, now: time.Now} }

func ts(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTS(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}

func nullTS(s sql.NullString) *time.Time {
	if !s.Valid || s.String == "" {
		return nil
	}
	t := parseTS(s.String)
	return &t
}

func jsonOrNull(v any) any {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	if string(b) == "null" || string(b) == "[]" {
		return nil
	}
	return string(b)
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	s := strings.Trim(slugRe.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if len(s) > 48 {
		s = strings.Trim(s[:48], "-")
	}
	if s == "" {
		s = "assistant"
	}
	return s
}

// uniqueSlug returns name's slug, suffixed until no other AI uses it.
func (r *Repo) uniqueSlug(ctx context.Context, name, selfID string) (string, error) {
	base := slugify(name)
	for i := 1; ; i++ {
		slug := base
		if i > 1 {
			slug = fmt.Sprintf("%s-%d", base, i)
		}
		var id string
		err := r.db.QueryRowContext(ctx, `SELECT id FROM specialized_ais WHERE slug = ?`, slug).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) || id == selfID {
			return slug, nil
		}
		if err != nil {
			return "", err
		}
	}
}

// CreateAI stores a new specialized AI.
func (r *Repo) CreateAI(ctx context.Context, ai SpecializedAI) (SpecializedAI, error) {
	ai.ID = uuid.NewString()
	slug, err := r.uniqueSlug(ctx, ai.Name, "")
	if err != nil {
		return SpecializedAI{}, err
	}
	ai.Slug = slug
	if ai.Preset == "" {
		ai.Preset = PresetBalanced
	}
	now := r.now()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO specialized_ais (id, slug, name, goal, instructions, base_model_id, preset, advanced_json, knowledge_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ai.ID, ai.Slug, ai.Name, ai.Goal, ai.Instructions, ai.BaseModelID, string(ai.Preset),
		jsonOrNull(ai.Advanced), jsonOrNull(ai.Knowledge), ts(now), ts(now))
	if err != nil {
		return SpecializedAI{}, err
	}
	return r.GetAI(ctx, ai.ID)
}

// UpdateAI saves every editable field. A rename changes the slug.
func (r *Repo) UpdateAI(ctx context.Context, ai SpecializedAI) (SpecializedAI, error) {
	slug, err := r.uniqueSlug(ctx, ai.Name, ai.ID)
	if err != nil {
		return SpecializedAI{}, err
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE specialized_ais SET slug=?, name=?, goal=?, instructions=?, base_model_id=?, preset=?, advanced_json=?,
			knowledge_json=?, deployed_revision=?, updated_at=? WHERE id=?`,
		slug, ai.Name, ai.Goal, ai.Instructions, ai.BaseModelID, string(ai.Preset), jsonOrNull(ai.Advanced),
		jsonOrNull(ai.Knowledge), ai.DeployedRevision, ts(r.now()), ai.ID)
	if err != nil {
		return SpecializedAI{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return SpecializedAI{}, fmt.Errorf("specialized AI %w", ErrNotFound)
	}
	return r.GetAI(ctx, ai.ID)
}

const aiColumns = `id, slug, name, goal, instructions, base_model_id, preset, COALESCE(advanced_json, ''),
	COALESCE(knowledge_json, ''), deployed_revision, created_at, updated_at`

func scanAI(row interface{ Scan(...any) error }) (SpecializedAI, error) {
	var ai SpecializedAI
	var preset, adv, know, created, updated string
	if err := row.Scan(&ai.ID, &ai.Slug, &ai.Name, &ai.Goal, &ai.Instructions, &ai.BaseModelID, &preset, &adv, &know,
		&ai.DeployedRevision, &created, &updated); err != nil {
		return SpecializedAI{}, err
	}
	ai.Preset = Preset(preset)
	if adv != "" {
		ai.Advanced = &Hyper{}
		_ = json.Unmarshal([]byte(adv), ai.Advanced)
	}
	ai.Knowledge = []string{}
	if know != "" {
		_ = json.Unmarshal([]byte(know), &ai.Knowledge)
	}
	ai.CreatedAt, ai.UpdatedAt = parseTS(created), parseTS(updated)
	return ai, nil
}

// GetAI returns one AI by id.
func (r *Repo) GetAI(ctx context.Context, id string) (SpecializedAI, error) {
	ai, err := scanAI(r.db.QueryRowContext(ctx, `SELECT `+aiColumns+` FROM specialized_ais WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return SpecializedAI{}, fmt.Errorf("specialized AI %w", ErrNotFound)
	}
	return ai, err
}

// GetAIBySlug resolves a model id such as sai:tire-assistant.
func (r *Repo) GetAIBySlug(ctx context.Context, slug string) (SpecializedAI, error) {
	ai, err := scanAI(r.db.QueryRowContext(ctx, `SELECT `+aiColumns+` FROM specialized_ais WHERE slug = ?`, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return SpecializedAI{}, fmt.Errorf("specialized AI %q %w", slug, ErrNotFound)
	}
	return ai, err
}

// ListAIs returns every AI, most recently updated first.
func (r *Repo) ListAIs(ctx context.Context) ([]SpecializedAI, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+aiColumns+` FROM specialized_ais ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SpecializedAI{}
	for rows.Next() {
		ai, err := scanAI(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, ai)
	}
	return out, rows.Err()
}

// DeleteAI removes an AI and, by cascade, its materials, examples, jobs,
// revisions, and evaluations.
func (r *Repo) DeleteAI(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM specialized_ais WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("specialized AI %w", ErrNotFound)
	}
	return nil
}

// AddMaterial stores material and its examples in one transaction.
func (r *Repo) AddMaterial(ctx context.Context, m Material, examples [][]Message) (Material, error) {
	m.ID = uuid.NewString()
	m.CreatedAt = r.now()
	if m.Use.trains() {
		m.ExampleCount = len(examples)
	}
	rec, _ := json.Marshal(m.Recommended)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Material{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO training_materials (id, ai_id, name, filename, use, recommendation_json, warning, knowledge_source_id, example_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.AIID, m.Name, m.Filename, string(m.Use), string(rec), m.Warning, nullString(m.KnowledgeSourceID),
		m.ExampleCount, ts(m.CreatedAt)); err != nil {
		return Material{}, err
	}
	if m.Use.trains() {
		if err := insertExamples(ctx, tx, m.AIID, m.ID, examples, r.now()); err != nil {
			return Material{}, err
		}
	}
	return m, tx.Commit()
}

func insertExamples(ctx context.Context, tx *sql.Tx, aiID, materialID string, examples [][]Message, now time.Time) error {
	var pos int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), 0) FROM training_examples WHERE ai_id = ?`, aiID).Scan(&pos); err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO training_examples (id, ai_id, material_id, position, messages_json, excluded, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, msgs := range examples {
		pos++
		b, _ := json.Marshal(msgs)
		if _, err := stmt.ExecContext(ctx, uuid.NewString(), aiID, nullString(materialID), pos, string(b), ts(now)); err != nil {
			return err
		}
	}
	return nil
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ListMaterials returns an AI's material in the order it was added.
func (r *Repo) ListMaterials(ctx context.Context, aiID string) ([]Material, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, ai_id, name, COALESCE(filename, ''), use, recommendation_json, COALESCE(warning, ''),
			COALESCE(knowledge_source_id, ''), example_count, created_at
		FROM training_materials WHERE ai_id = ? ORDER BY created_at`, aiID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Material{}
	for rows.Next() {
		var m Material
		var use, rec, created string
		if err := rows.Scan(&m.ID, &m.AIID, &m.Name, &m.Filename, &use, &rec, &m.Warning, &m.KnowledgeSourceID,
			&m.ExampleCount, &created); err != nil {
			return nil, err
		}
		m.Use = Use(use)
		_ = json.Unmarshal([]byte(rec), &m.Recommended)
		m.CreatedAt = parseTS(created)
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetMaterial returns one material.
func (r *Repo) GetMaterial(ctx context.Context, aiID, id string) (Material, error) {
	list, err := r.ListMaterials(ctx, aiID)
	if err != nil {
		return Material{}, err
	}
	for _, m := range list {
		if m.ID == id {
			return m, nil
		}
	}
	return Material{}, fmt.Errorf("material %w", ErrNotFound)
}

// DeleteMaterial removes material and the examples it produced.
func (r *Repo) DeleteMaterial(ctx context.Context, aiID, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM training_materials WHERE id = ? AND ai_id = ?`, id, aiID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("material %w", ErrNotFound)
	}
	return nil
}

// AddExamples stores examples the user wrote or imported from chats.
func (r *Repo) AddExamples(ctx context.Context, aiID string, examples [][]Message) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := insertExamples(ctx, tx, aiID, "", examples, r.now()); err != nil {
		return err
	}
	return tx.Commit()
}

// ListExamples returns an AI's examples in order, without flags.
func (r *Repo) ListExamples(ctx context.Context, aiID string) ([]Example, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, ai_id, COALESCE(material_id, ''), messages_json, excluded, created_at
		FROM training_examples WHERE ai_id = ? ORDER BY position`, aiID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Example{}
	for rows.Next() {
		var ex Example
		var msgs, created string
		var excluded int
		if err := rows.Scan(&ex.ID, &ex.AIID, &ex.MaterialID, &msgs, &excluded, &created); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(msgs), &ex.Messages)
		ex.Excluded = excluded != 0
		ex.CreatedAt = parseTS(created)
		out = append(out, ex)
	}
	return out, rows.Err()
}

// UpdateExample edits an example's turns or its excluded flag.
func (r *Repo) UpdateExample(ctx context.Context, aiID, id string, messages []Message, excluded *bool) error {
	sets, args := []string{}, []any{}
	if messages != nil {
		b, _ := json.Marshal(messages)
		sets, args = append(sets, "messages_json = ?"), append(args, string(b))
	}
	if excluded != nil {
		sets, args = append(sets, "excluded = ?"), append(args, boolInt(*excluded))
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id, aiID)
	res, err := r.db.ExecContext(ctx, `UPDATE training_examples SET `+strings.Join(sets, ", ")+` WHERE id = ? AND ai_id = ?`, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("example %w", ErrNotFound)
	}
	return nil
}

// DeleteExample removes one example.
func (r *Repo) DeleteExample(ctx context.Context, aiID, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM training_examples WHERE id = ? AND ai_id = ?`, id, aiID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("example %w", ErrNotFound)
	}
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// CreateJob stores a queued job and reserves the next revision number.
func (r *Repo) CreateJob(ctx context.Context, j Job) (Job, error) {
	j.ID = uuid.NewString()
	j.State = StateQueued
	j.CreatedAt = r.now()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, err
	}
	defer func() { _ = tx.Rollback() }()
	var active int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(1) FROM training_jobs WHERE ai_id = ? AND state NOT IN (?, ?, ?)`,
		j.AIID, StateComplete, StateFailed, StateCancelled).Scan(&active); err != nil {
		return Job{}, err
	}
	if active > 0 {
		return Job{}, fmt.Errorf("this AI is already training: %w", ErrConflict)
	}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision), 0) + 1 FROM training_jobs WHERE ai_id = ?`, j.AIID).Scan(&j.Revision); err != nil {
		return Job{}, err
	}
	hyper, _ := json.Marshal(j.Hyper)
	prog, _ := json.Marshal(j.Progress)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO training_jobs (id, ai_id, revision, node_id, node_name, backend, state, progress_json, hyper_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.ID, j.AIID, j.Revision, j.NodeID, j.NodeName, j.Backend, string(j.State), string(prog), string(hyper), ts(j.CreatedAt)); err != nil {
		return Job{}, err
	}
	return j, tx.Commit()
}

// SaveJob stores a job's state, progress, and timestamps.
func (r *Repo) SaveJob(ctx context.Context, j Job) error {
	prog, _ := json.Marshal(j.Progress)
	hyper, _ := json.Marshal(j.Hyper)
	var started, finished any
	if j.StartedAt != nil {
		started = ts(*j.StartedAt)
	}
	if j.FinishedAt != nil {
		finished = ts(*j.FinishedAt)
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE training_jobs SET state=?, progress_json=?, hyper_json=?, error=?, started_at=?, finished_at=? WHERE id=?`,
		string(j.State), string(prog), string(hyper), nullString(j.Error), started, finished, j.ID)
	return err
}

const jobColumns = `id, ai_id, revision, node_id, COALESCE(node_name, ''), backend, state, COALESCE(progress_json, '{}'),
	hyper_json, COALESCE(error, ''), created_at, started_at, finished_at`

func scanJob(row interface{ Scan(...any) error }) (Job, error) {
	var j Job
	var state, prog, hyper, created string
	var started, finished sql.NullString
	if err := row.Scan(&j.ID, &j.AIID, &j.Revision, &j.NodeID, &j.NodeName, &j.Backend, &state, &prog, &hyper,
		&j.Error, &created, &started, &finished); err != nil {
		return Job{}, err
	}
	j.State = State(state)
	_ = json.Unmarshal([]byte(prog), &j.Progress)
	_ = json.Unmarshal([]byte(hyper), &j.Hyper)
	j.CreatedAt = parseTS(created)
	j.StartedAt, j.FinishedAt = nullTS(started), nullTS(finished)
	return j, nil
}

// GetJob returns one job.
func (r *Repo) GetJob(ctx context.Context, id string) (Job, error) {
	j, err := scanJob(r.db.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM training_jobs WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, fmt.Errorf("training job %w", ErrNotFound)
	}
	return j, err
}

// ListJobs returns jobs, newest first. An empty aiID lists every AI's jobs.
func (r *Repo) ListJobs(ctx context.Context, aiID string) ([]Job, error) {
	q := `SELECT ` + jobColumns + ` FROM training_jobs`
	var args []any
	if aiID != "" {
		q += ` WHERE ai_id = ?`
		args = append(args, aiID)
	}
	rows, err := r.db.QueryContext(ctx, q+` ORDER BY created_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// ActiveJobs returns jobs that have not finished.
func (r *Repo) ActiveJobs(ctx context.Context) ([]Job, error) {
	all, err := r.ListJobs(ctx, "")
	if err != nil {
		return nil, err
	}
	var out []Job
	for _, j := range all {
		if !j.State.Terminal() {
			out = append(out, j)
		}
	}
	return out, nil
}

// AddRevision records a trained adapter.
func (r *Repo) AddRevision(ctx context.Context, rev Revision) error {
	hyper, _ := json.Marshal(rev.Hyper)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ai_revisions (ai_id, revision, job_id, base_model_id, backend, hyper_json, example_count, examples_hash,
			adapter_path, final_train_loss, final_val_loss, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rev.AIID, rev.Revision, rev.JobID, rev.BaseModelID, rev.Backend, string(hyper), rev.ExampleCount, rev.ExamplesHash,
		rev.adapterPath, rev.FinalLoss, rev.ValLoss, ts(r.now()))
	return err
}

// ListRevisions returns an AI's revisions, newest first, with whether each
// has a finished evaluation.
func (r *Repo) ListRevisions(ctx context.Context, aiID string) ([]Revision, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT rv.ai_id, rv.revision, rv.job_id, rv.base_model_id, rv.backend, rv.hyper_json, rv.example_count, rv.examples_hash,
			rv.adapter_path, rv.final_train_loss, rv.final_val_loss, rv.created_at,
			EXISTS (SELECT 1 FROM eval_runs e WHERE e.ai_id = rv.ai_id AND e.revision = rv.revision AND e.status = 'complete')
		FROM ai_revisions rv WHERE rv.ai_id = ? ORDER BY rv.revision DESC`, aiID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Revision{}
	for rows.Next() {
		var rv Revision
		var hyper, created string
		var train, val sql.NullFloat64
		var evaluated int
		if err := rows.Scan(&rv.AIID, &rv.Revision, &rv.JobID, &rv.BaseModelID, &rv.Backend, &hyper, &rv.ExampleCount,
			&rv.ExamplesHash, &rv.adapterPath, &train, &val, &created, &evaluated); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(hyper), &rv.Hyper)
		if train.Valid {
			rv.FinalLoss = &train.Float64
		}
		if val.Valid {
			rv.ValLoss = &val.Float64
		}
		rv.Evaluated = evaluated != 0
		rv.CreatedAt = parseTS(created)
		out = append(out, rv)
	}
	return out, rows.Err()
}

// GetRevision returns one revision.
func (r *Repo) GetRevision(ctx context.Context, aiID string, revision int) (Revision, error) {
	list, err := r.ListRevisions(ctx, aiID)
	if err != nil {
		return Revision{}, err
	}
	for _, rv := range list {
		if rv.Revision == revision {
			return rv, nil
		}
	}
	return Revision{}, fmt.Errorf("revision %d %w", revision, ErrNotFound)
}

// EvalPrompt is one test prompt.
type EvalPrompt struct {
	ID     string `json:"id"`
	Prompt string `json:"prompt"`
}

// SetEvalPrompts replaces an AI's test set.
func (r *Repo) SetEvalPrompts(ctx context.Context, aiID string, prompts []string) ([]EvalPrompt, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM eval_prompts WHERE ai_id = ?`, aiID); err != nil {
		return nil, err
	}
	n := 0
	for _, p := range prompts {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		n++
		if _, err := tx.ExecContext(ctx, `INSERT INTO eval_prompts (id, ai_id, position, prompt, created_at) VALUES (?, ?, ?, ?, ?)`,
			uuid.NewString(), aiID, n, p, ts(r.now())); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.EvalPrompts(ctx, aiID)
}

// EvalPrompts returns an AI's test set.
func (r *Repo) EvalPrompts(ctx context.Context, aiID string) ([]EvalPrompt, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, prompt FROM eval_prompts WHERE ai_id = ? ORDER BY position`, aiID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EvalPrompt{}
	for rows.Next() {
		var p EvalPrompt
		if err := rows.Scan(&p.ID, &p.Prompt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// EvalResult is one prompt answered by the base and the specialized model.
type EvalResult struct {
	Prompt      string `json:"prompt"`
	Base        string `json:"base"`
	Specialized string `json:"specialized"`
	Error       string `json:"error,omitempty"`
}

// EvalRun is one comparison of a revision against its base model.
type EvalRun struct {
	ID         string       `json:"id"`
	AIID       string       `json:"ai_id"`
	Revision   int          `json:"revision"`
	Status     string       `json:"status"`
	Results    []EvalResult `json:"results"`
	Error      string       `json:"error,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
	FinishedAt *time.Time   `json:"finished_at,omitempty"`
}

// SaveEvalRun inserts or updates an evaluation.
func (r *Repo) SaveEvalRun(ctx context.Context, run EvalRun) (EvalRun, error) {
	if run.ID == "" {
		run.ID = uuid.NewString()
		run.CreatedAt = r.now()
	}
	results, _ := json.Marshal(run.Results)
	var finished any
	if run.FinishedAt != nil {
		finished = ts(*run.FinishedAt)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO eval_runs (id, ai_id, revision, status, results_json, error, created_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET status=excluded.status, results_json=excluded.results_json, error=excluded.error,
			finished_at=excluded.finished_at`,
		run.ID, run.AIID, run.Revision, run.Status, string(results), nullString(run.Error), ts(run.CreatedAt), finished)
	return run, err
}

// ListEvalRuns returns an AI's evaluations, newest first.
func (r *Repo) ListEvalRuns(ctx context.Context, aiID string) ([]EvalRun, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, ai_id, revision, status, COALESCE(results_json, '[]'), COALESCE(error, ''), created_at, finished_at
		FROM eval_runs WHERE ai_id = ? ORDER BY created_at DESC`, aiID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EvalRun{}
	for rows.Next() {
		var run EvalRun
		var results, created string
		var finished sql.NullString
		if err := rows.Scan(&run.ID, &run.AIID, &run.Revision, &run.Status, &results, &run.Error, &created, &finished); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(results), &run.Results)
		run.CreatedAt = parseTS(created)
		run.FinishedAt = nullTS(finished)
		out = append(out, run)
	}
	return out, rows.Err()
}
