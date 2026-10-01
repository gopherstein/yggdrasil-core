package tools

import (
	"fmt"
	"strings"

	"github.com/yeixio/yggdrasil-core/pkg/contracts"
)

// UnattendedPolicy is the policy a scheduled run may use for one tool.
// The profile remains the ceiling: a grant cannot turn on a tool the profile does not already allow.
// ask and allow-for-session stay a human decision, so they do not run while the UI is closed.
// Write tools stay off as well. Unattended work is limited to read-only tools.
func UnattendedPolicy(profile contracts.AIProfile, granted []string, toolID string) (string, error) {
	toolID = strings.TrimSpace(toolID)
	if toolID == "" {
		return "", fmt.Errorf("tool id is required")
	}
	if !grantIncludes(granted, toolID) {
		return "", fmt.Errorf("tool %q is not enabled for this automation", toolID)
	}
	def, ok := Lookup(toolID)
	if !ok {
		return "", fmt.Errorf("tool %q is not allowed for unattended execution", toolID)
	}
	policy := strings.ToLower(strings.TrimSpace(PolicyForProfile(profile, toolID)))
	if policy != PolicyAllow {
		return "", fmt.Errorf("tool %q is not allowed for unattended execution", toolID)
	}
	if def.Risk != "read" {
		return "", fmt.Errorf("tool %q is not allowed for unattended execution", toolID)
	}
	return PolicyAllow, nil
}

// ForUnattended returns a profile whose tool list matches UnattendedPolicy.
// The orchestrator advertises only those tools. The stored profile is left unchanged.
func ForUnattended(profile contracts.AIProfile, granted []string, disabled map[string]struct{}) contracts.AIProfile {
	out := profile
	out.Tools = make([]contracts.ToolPolicy, len(profile.Tools))
	for i, tool := range profile.Tools {
		policy := PolicyDeny
		if _, off := disabled[tool.ToolID]; !off {
			if allowed, err := UnattendedPolicy(profile, granted, tool.ToolID); err == nil {
				policy = allowed
			}
		}
		out.Tools[i] = contracts.ToolPolicy{ToolID: tool.ToolID, Policy: policy}
	}
	return out
}

func grantIncludes(granted []string, toolID string) bool {
	if len(granted) == 0 {
		return true
	}
	for _, id := range granted {
		if strings.TrimSpace(id) == toolID {
			return true
		}
	}
	return false
}
