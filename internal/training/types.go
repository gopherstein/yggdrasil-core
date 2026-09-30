// Package training builds specialized AIs: a base model, an optional LoRA
// adapter trained on the user's examples, system instructions, and connected
// knowledge from Mimir. Training teaches behavior. Knowledge supplies current
// facts. The package keeps the two apart and guides the user between them.
package training

import (
	"errors"
	"time"
)

// Use is how a piece of material feeds a specialized AI.
type Use string

const (
	// UseTraining turns material into examples that teach behavior.
	UseTraining Use = "training"
	// UseKnowledge connects material through Mimir so it stays current.
	UseKnowledge Use = "knowledge"
	// UseBoth does both: examples teach the pattern, knowledge supplies facts.
	UseBoth Use = "both"
)

// Valid reports whether u is a known use.
func (u Use) Valid() bool { return u == UseTraining || u == UseKnowledge || u == UseBoth }

func (u Use) trains() bool   { return u == UseTraining || u == UseBoth }
func (u Use) connects() bool { return u == UseKnowledge || u == UseBoth }

// Preset is a training effort level. Raw hyperparameters stay behind Advanced.
type Preset string

const (
	PresetQuick    Preset = "quick"
	PresetBalanced Preset = "balanced"
	PresetQuality  Preset = "quality"
)

// Method is the fine-tuning technique.
type Method string

const (
	// MethodLoRA trains an adapter on full-precision base weights.
	MethodLoRA Method = "lora"
	// MethodQLoRA trains an adapter on 4-bit base weights. It needs less memory
	// and a smaller download.
	MethodQLoRA Method = "qlora"
)

// Hyper holds raw training settings. Presets fill them; Advanced mode may
// override individual fields.
type Hyper struct {
	Method       Method  `json:"method,omitempty"`
	Epochs       int     `json:"epochs,omitempty"`
	Rank         int     `json:"rank,omitempty"`
	Scale        float64 `json:"scale,omitempty"`
	Layers       int     `json:"layers,omitempty"`
	LearningRate float64 `json:"learning_rate,omitempty"`
	BatchSize    int     `json:"batch_size,omitempty"`
	MaxSeqLength int     `json:"max_seq_length,omitempty"`
	// Iters is derived from epochs, examples, and batch size.
	Iters          int  `json:"iters,omitempty"`
	GradCheckpoint bool `json:"grad_checkpoint,omitempty"`
	Seed           int  `json:"seed,omitempty"`
}

// Message is one chat turn in a training example.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Flag describes a problem with an example.
type Flag string

const (
	FlagEmpty     Flag = "empty"
	FlagNoAnswer  Flag = "no_answer"
	FlagDuplicate Flag = "duplicate"
	FlagTooLong   Flag = "too_long"
	FlagShort     Flag = "short_answer"
	// FlagVolatile means the answer states facts that change, such as a
	// price or stock count. The model may memorize a stale value.
	FlagVolatile Flag = "volatile_facts"
)

// Blocking reports whether the flag keeps an example out of training.
func (f Flag) Blocking() bool {
	return f == FlagEmpty || f == FlagNoAnswer || f == FlagDuplicate || f == FlagTooLong
}

// Example is one training example.
type Example struct {
	ID         string    `json:"id"`
	AIID       string    `json:"ai_id"`
	MaterialID string    `json:"material_id,omitempty"`
	Messages   []Message `json:"messages"`
	Flags      []Flag    `json:"flags,omitempty"`
	// Excluded is the user's choice to leave the example out.
	Excluded  bool      `json:"excluded"`
	CreatedAt time.Time `json:"created_at"`
}

// Usable reports whether the example goes into training.
func (e Example) Usable() bool {
	if e.Excluded {
		return false
	}
	for _, f := range e.Flags {
		if f.Blocking() {
			return false
		}
	}
	return true
}

// Signal is one observation behind a classification.
type Signal struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

// Recommendation is Yggdrasil's advice for one piece of material.
type Recommendation struct {
	Use     Use      `json:"use"`
	Reasons []string `json:"reasons"`
	Signals []Signal `json:"signals,omitempty"`
	// Warning is shown when the chosen use conflicts with the advice.
	Warning string `json:"warning,omitempty"`
	// ExampleCount is how many training examples the material yields.
	ExampleCount int `json:"example_count"`
	// CanTrain is false when no examples could be read, so the material can
	// only be connected knowledge.
	CanTrain bool `json:"can_train"`
}

// Material is a file, paste, or set of conversations the user added.
type Material struct {
	ID       string `json:"id"`
	AIID     string `json:"ai_id"`
	Name     string `json:"name"`
	Filename string `json:"filename,omitempty"`
	// Use is the user's decision. Recommended is Yggdrasil's advice.
	Use         Use            `json:"use"`
	Recommended Recommendation `json:"recommended"`
	// Warning explains a conflict between Use and the recommendation.
	Warning           string    `json:"warning,omitempty"`
	KnowledgeSourceID string    `json:"knowledge_source_id,omitempty"`
	ExampleCount      int       `json:"example_count"`
	CreatedAt         time.Time `json:"created_at"`
}

// SpecializedAI is what the user builds and deploys.
type SpecializedAI struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
	// Goal is the user's description of what the AI should do.
	Goal string `json:"goal"`
	// Instructions are the system instructions sent with every turn and
	// prepended to every training example.
	Instructions string   `json:"instructions"`
	BaseModelID  string   `json:"base_model_id"`
	Preset       Preset   `json:"preset"`
	Advanced     *Hyper   `json:"advanced,omitempty"`
	Knowledge    []string `json:"knowledge_sources"`
	// Example marks the built-in example AI.
	Example bool `json:"example,omitempty"`
	// DeployedRevision is 0 until a revision is deployed.
	DeployedRevision int       `json:"deployed_revision"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ModelID is how chat and the API address a deployed specialized AI.
func (a SpecializedAI) ModelID() string { return ModelPrefix + a.Slug }

// ModelPrefix marks specialized AI model ids.
const ModelPrefix = "sai:"

// Revision is one trained adapter.
type Revision struct {
	AIID         string    `json:"ai_id"`
	Revision     int       `json:"revision"`
	JobID        string    `json:"job_id"`
	BaseModelID  string    `json:"base_model_id"`
	Backend      string    `json:"backend"`
	Hyper        Hyper     `json:"hyper"`
	ExampleCount int       `json:"example_count"`
	ExamplesHash string    `json:"examples_hash"`
	FinalLoss    *float64  `json:"final_train_loss,omitempty"`
	ValLoss      *float64  `json:"final_val_loss,omitempty"`
	Evaluated    bool      `json:"evaluated"`
	CreatedAt    time.Time `json:"created_at"`
	adapterPath  string
}

// AdapterID names a revision's adapter inside a running model.
func (r Revision) AdapterID() string { return AdapterID(r.AIID, r.Revision) }

// AdapterID names the adapter for an AI revision.
func AdapterID(aiID string, revision int) string {
	return aiID + "@" + itoa(revision)
}

// State is a training job state.
type State string

const (
	StateQueued    State = "queued"
	StatePreparing State = "preparing_dataset"
	StateLoading   State = "loading_model"
	StateTraining  State = "training"
	StateEvaluate  State = "evaluating"
	StateExporting State = "exporting"
	StateComplete  State = "complete"
	StateFailed    State = "failed"
	StateCancelled State = "cancelled"
)

// Terminal reports whether the job has finished.
func (s State) Terminal() bool {
	return s == StateComplete || s == StateFailed || s == StateCancelled
}

// Progress is the live view of a job.
type Progress struct {
	Iter          int      `json:"iter,omitempty"`
	Iters         int      `json:"iters,omitempty"`
	Epoch         float64  `json:"epoch,omitempty"`
	Epochs        int      `json:"epochs,omitempty"`
	TrainLoss     *float64 `json:"train_loss,omitempty"`
	ValLoss       *float64 `json:"val_loss,omitempty"`
	TokensPerSec  float64  `json:"tokens_per_sec,omitempty"`
	PeakMemoryGB  float64  `json:"peak_memory_gb,omitempty"`
	DownloadBytes int64    `json:"download_bytes,omitempty"`
	DownloadTotal int64    `json:"download_total,omitempty"`
	// RemainingSec is set once the rate is steady enough to trust.
	RemainingSec *int `json:"remaining_sec,omitempty"`
	// Detail is a short human sentence for the current step.
	Detail string `json:"detail,omitempty"`
}

// Job is one training run.
type Job struct {
	ID         string     `json:"id"`
	AIID       string     `json:"ai_id"`
	Revision   int        `json:"revision"`
	NodeID     string     `json:"node_id"`
	NodeName   string     `json:"node_name,omitempty"`
	Backend    string     `json:"backend"`
	State      State      `json:"state"`
	Progress   Progress   `json:"progress"`
	Hyper      Hyper      `json:"hyper"`
	Error      string     `json:"error,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

// ElapsedSec is the job's running time.
func (j Job) ElapsedSec(now time.Time) int {
	if j.StartedAt == nil {
		return 0
	}
	end := now
	if j.FinishedAt != nil {
		end = *j.FinishedAt
	}
	return int(end.Sub(*j.StartedAt).Seconds())
}

// Errors the API maps to status codes.
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict")
)

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
