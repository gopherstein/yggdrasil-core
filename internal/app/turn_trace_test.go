package app

import (
	"strings"
	"testing"

	"github.com/yeixio/yggdrasil-core/internal/mimir"
	"github.com/yeixio/yggdrasil-core/internal/tools"
	"github.com/yeixio/yggdrasil-core/internal/tools/internet"
)

func TestTurnTraceRecordsSourcesAndSteps(t *testing.T) {
	tr := &turnTrace{}
	if tr.meta() != nil || tr.sawUntrusted() {
		t.Fatal("an empty turn has no meta and no untrusted content")
	}
	tr.knowledge([]mimir.Hit{
		{SourceName: "inventory.csv", Title: "inventory.csv row 1", Body: "sku: MP-1; price: 189.99"},
		{SourceName: "inventory.csv", Title: "inventory.csv row 2", Body: "sku: MP-2"},
	})
	tr.tool("internet.search", map[string]any{"query": "tire pressure"}, map[string]any{"results": []internet.Result{
		{Title: "PSI guide", URL: "https://example.com/psi", Snippet: "Use the door sticker."},
		{Title: "", URL: "https://www.tires.example/faq"},
	}})
	tr.tool("internet.open", map[string]any{"url": "https://example.com/psi"}, map[string]any{
		"url": "https://example.com/psi", "title": "Tire pressure, explained", "content": strings.Repeat("word ", 200)})

	meta := tr.meta()
	if meta == nil || !tr.sawUntrusted() {
		t.Fatal("expected meta and untrusted content")
	}
	var steps []string
	for _, s := range meta.Steps {
		steps = append(steps, s.Text)
	}
	want := "Found 2 passages in inventory.csv|Searched the web for “tire pressure”|Read “Tire pressure, explained”"
	if strings.Join(steps, "|") != want {
		t.Fatalf("steps = %v", steps)
	}
	// Two knowledge passages, two search hits; the opened page renames its hit.
	if len(meta.Sources) != 4 {
		t.Fatalf("sources = %+v", meta.Sources)
	}
	if meta.Sources[2].Title != "Tire pressure, explained" || meta.Sources[3].Title != "tires.example" {
		t.Fatalf("web sources = %+v", meta.Sources[2:])
	}
}

func TestTurnTraceCapsSources(t *testing.T) {
	tr := &turnTrace{}
	var hits []mimir.Hit
	for i := 0; i < 20; i++ {
		hits = append(hits, mimir.Hit{SourceName: "kb", Title: strings.Repeat("x", i+1)})
	}
	tr.knowledge(hits)
	if got := len(tr.meta().Sources); got != maxTurnSources {
		t.Fatalf("sources = %d", got)
	}
}

func TestEffectivePolicyAsksAfterUntrustedContent(t *testing.T) {
	cases := []struct {
		policy, tool string
		untrusted    bool
		want         string
	}{
		{tools.PolicyAllow, "terminal", false, tools.PolicyAllow},
		{tools.PolicyAllow, "terminal", true, tools.PolicyAsk},
		{tools.PolicyAllow, "filesystem.write", true, tools.PolicyAsk},
		{tools.PolicyAllow, "git.push", true, tools.PolicyAsk},
		{tools.PolicyAllow, "internet.open", true, tools.PolicyAllow},
		{tools.PolicyAllow, "unknown.tool", true, tools.PolicyAsk},
		{tools.PolicyDeny, "terminal", true, tools.PolicyDeny},
	}
	for _, c := range cases {
		if got := effectivePolicy(c.policy, c.tool, c.untrusted); got != c.want {
			t.Errorf("%s %s untrusted=%v: got %s, want %s", c.policy, c.tool, c.untrusted, got, c.want)
		}
	}
}
