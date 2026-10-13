package app

import (
	"context"

	"github.com/yeixio/toskar-core/internal/huginn"
)

// loadStartedPayload is what model.load.started says about a model a turn
// is about to use (#498): which model and computer, the conversation, and,
// when this computer runs it, whether it has to load first (cold) and about
// how long that took recently (expected_ms), so a device can say "Loading
// Qwen 2.5 7B on Office… about 20 s" instead of a bare spinner.
func (a *App) loadStartedPayload(ctx context.Context, conversationID, modelID, nodeID, role string) map[string]any {
	node := a.nodeDisplayName(nodeID)
	p := map[string]any{"model_id": modelID, "node_id": nodeID, "role": role, "node_name": node}
	if conversationID != "" {
		p["conversation_id"] = conversationID
	}
	name := modelID
	for _, m := range a.installedModels(ctx) {
		if m.ID == modelID {
			name = huginn.Name(m)
		}
	}
	p["model_name"] = name
	if nodeID != "" && nodeID != a.Config.Get().NodeID {
		// Whether a paired computer has it loaded isn't known here.
		if ms, ok := a.RunLog.LoadTime(ctx, modelID, node); ok {
			p["expected_ms"] = ms
		}
		return p
	}
	cold := !a.runningHere(ctx, modelID)
	p["cold"] = cold
	if cold {
		if ms, ok := a.RunLog.LoadTime(ctx, modelID, node); ok {
			p["expected_ms"] = ms
		}
	}
	return p
}

// runningHere reports whether modelID is loaded on this computer for chat.
func (a *App) runningHere(ctx context.Context, modelID string) bool {
	if a.stubInference {
		return true
	}
	if a.Runtimes == nil {
		return false
	}
	running, err := a.Runtimes.ListRunning(ctx, "llamacpp")
	if err != nil {
		return false
	}
	for _, r := range running {
		if r.ModelID == modelID && r.Mode == "" {
			return true
		}
	}
	return false
}
