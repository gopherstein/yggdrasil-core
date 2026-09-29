package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/yeixio/yggdrasil-core/internal/automations"
	"github.com/yeixio/yggdrasil-core/internal/tools"
	"github.com/yeixio/yggdrasil-core/pkg/contracts"
	"github.com/yeixio/yggdrasil-core/pkg/pluginapi"
)

// automationExecutor runs a scheduled prompt through the profile's orchestrator.
// Placement goes through Norn. A model started for the run is marked used afterward
// so the normal idle sweeper can unload it. The run does not pin the model.
type automationExecutor struct {
	app *App
}

func (e automationExecutor) Execute(ctx context.Context, automation automations.Automation) (automations.Execution, error) {
	if e.app == nil || e.app.Profiles == nil || e.app.OrchRegistry == nil {
		return automations.Execution{}, fmt.Errorf("automation executor is not configured")
	}
	if strings.TrimSpace(automation.ProfileID) == "" {
		return automations.Execution{}, fmt.Errorf("profile is required")
	}
	profile, err := e.app.Profiles.Get(ctx, automation.ProfileID)
	if err != nil {
		return automations.Execution{}, err
	}
	orch, err := e.app.OrchRegistry.Get(profile.OrchestratorID)
	if err != nil {
		return automations.Execution{}, err
	}
	if err := orch.ValidateProfile(ctx, profile); err != nil {
		return automations.Execution{}, err
	}

	var disabled map[string]struct{}
	if e.app.Tools != nil {
		disabled = e.app.Tools.Disabled()
	}
	env := &automationEnv{
		base: &chatExecEnv{
			app:     e.app,
			ctx:     ctx,
			profile: profile,
			taskID:  automation.ID,
		},
		granted: automation.Tools,
	}
	defer env.releaseModels()

	stream, err := orch.Run(ctx, contracts.Task{
		ID:        automation.ID,
		ProfileID: profile.ID,
		Prompt:    automation.Prompt,
		Status:    contracts.TaskRunning,
	}, tools.ForUnattended(profile, automation.Tools, disabled), env)
	if err != nil {
		return env.execution(""), err
	}
	text, nodeID, runErr := collectAutomationEvents(stream)
	out := env.execution(text)
	if nodeID != "" {
		out.NodeID = nodeID
	}
	return out, runErr
}

type automationEnv struct {
	base    *chatExecEnv
	granted []string
}

func (e *automationEnv) Generate(ctx context.Context, role string, messages []pluginapi.ChatMessage) (<-chan pluginapi.ChatChunk, error) {
	return e.base.Generate(ctx, role, messages)
}

func (e *automationEnv) ExecuteTool(ctx context.Context, toolID string, args map[string]any) (map[string]any, error) {
	if e.base.app != nil && e.base.app.Tools != nil && e.base.app.Tools.IsDisabled(toolID) {
		return nil, fmt.Errorf("tool %q is disabled", toolID)
	}
	policy, err := tools.UnattendedPolicy(e.base.profile, e.granted, toolID)
	if err != nil {
		return nil, err
	}
	return e.base.app.Tools.Execute(ctx, toolID, args, policy, "scheduled automation", map[string]any{
		"automation_id": e.base.taskID,
	})
}

func (e *automationEnv) Emit(eventType string, payload map[string]any) {
	e.base.Emit(eventType, payload)
}

func (e *automationEnv) NodeForRole(role string) (string, error) {
	return e.base.NodeForRole(role)
}

func (e *automationEnv) execution(text string) automations.Execution {
	return automations.Execution{Text: text, ModelID: e.base.modelID()}
}

// releaseModels returns every model this run started on this computer to the
// normal idle policy. It does not stop a model the user already had loaded.
func (e *automationEnv) releaseModels() {
	if e.base == nil || e.base.app == nil {
		return
	}
	e.base.mu.Lock()
	var models []string
	for role, modelID := range e.base.roleModels {
		if modelID == "" || !e.base.app.modelOnThisComputer(e.base.roleNodes[role]) {
			continue
		}
		models = append(models, modelID)
	}
	e.base.mu.Unlock()
	seen := map[string]struct{}{}
	for _, id := range models {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		e.base.app.markModelUsed(id)
	}
}

func (a *App) modelOnThisComputer(nodeID string) bool {
	if nodeID == "" || nodeID == "local" {
		return true
	}
	if a == nil || a.Config == nil {
		return false
	}
	return nodeID == a.Config.Get().NodeID
}

func collectAutomationEvents(stream <-chan pluginapi.OrchestrationEvent) (text, nodeID string, err error) {
	var b strings.Builder
	for evt := range stream {
		if evt.NodeID != "" {
			nodeID = evt.NodeID
		}
		if evt.Error != "" {
			err = fmt.Errorf("%s", evt.Error)
		}
		if evt.Content != "" && evt.Done {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(evt.Content)
		}
	}
	return strings.TrimSpace(b.String()), nodeID, err
}
