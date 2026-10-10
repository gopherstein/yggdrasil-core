package app

import (
	"strings"
	"testing"

	"github.com/yeixio/toskar-core/pkg/contracts"
)

// Deliberate's drafts and outcome reach the answer's meta, and the outcome
// becomes a step in the chat's language (#459).
func TestDeliberationTrace(t *testing.T) {
	tr := &turnTrace{}
	if tr.meta() != nil {
		t.Fatal("meta with nothing recorded")
	}
	tr.draft(contracts.DeliberationDraft{Role: "assistant", Final: "$26", Text: "Final answer: $26", Chosen: true})
	tr.draft(contracts.DeliberationDraft{Role: "drafter:2", NodeName: "Studio", Final: "26"})
	tr.draft(contracts.DeliberationDraft{Role: "drafter:3", Failed: true})
	tr.deliberated("agreed", "$26", 3, 2)
	m := tr.meta()
	if m == nil || m.Deliberation == nil || m.Deliberation.Outcome != "agreed" || m.Deliberation.Final != "$26" || len(m.Deliberation.Drafts) != 3 {
		t.Fatalf("meta: %+v", m)
	}
	if !m.Deliberation.Drafts[0].Chosen || !m.Deliberation.Drafts[2].Failed {
		t.Fatalf("drafts: %+v", m.Deliberation.Drafts)
	}
	if len(m.Steps) != 1 || m.Steps[0].Kind != "deliberate" || !strings.Contains(m.Steps[0].Text, "3 independent drafts: they agreed") {
		t.Fatalf("steps: %+v", m.Steps)
	}
	for outcome, want := range map[string]string{
		"majority":  "2 agreed",
		"disagreed": "they disagreed",
		"long":      "too long to compare",
	} {
		tr := &turnTrace{}
		tr.deliberated(outcome, "", 3, 2)
		if text := tr.meta().Steps[0].Text; !strings.Contains(text, want) {
			t.Errorf("%s: %q", outcome, text)
		}
	}
	// In the chat's language.
	de := &turnTrace{lang: "de"}
	de.deliberated("agreed", "", 3, 3)
	if text := de.meta().Steps[0].Text; !strings.Contains(text, "Entwürfe") {
		t.Errorf("German: %q", text)
	}
}
