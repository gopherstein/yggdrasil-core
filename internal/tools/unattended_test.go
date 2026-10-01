package tools

import (
	"strings"
	"testing"

	"github.com/yeixio/yggdrasil-core/pkg/contracts"
)

func TestUnattendedPolicy(t *testing.T) {
	profile := contracts.AIProfile{
		Tools: []contracts.ToolPolicy{
			{ToolID: "internet.search", Policy: PolicyAllow},
			{ToolID: "filesystem.read", Policy: PolicyAllow},
			{ToolID: "filesystem.write", Policy: PolicyAllow},
			{ToolID: "terminal", Policy: PolicyAsk},
			{ToolID: "git.push", Policy: PolicyAllowForSession},
			{ToolID: "git.commit", Policy: PolicyDeny},
		},
	}

	policy, err := UnattendedPolicy(profile, nil, "internet.search")
	if err != nil || policy != PolicyAllow {
		t.Fatalf("search policy=%q err=%v", policy, err)
	}
	if _, err := UnattendedPolicy(profile, nil, "filesystem.write"); err == nil {
		t.Fatal("expected a write tool to stay off while unattended")
	}
	if _, err := UnattendedPolicy(profile, nil, "terminal"); err == nil {
		t.Fatal("expected ask to stay a prompt")
	}
	if _, err := UnattendedPolicy(profile, nil, "git.push"); err == nil {
		t.Fatal("expected session approval to stay a prompt")
	}
	if _, err := UnattendedPolicy(profile, nil, "git.commit"); err == nil {
		t.Fatal("expected deny to stay denied")
	}
	if _, err := UnattendedPolicy(profile, []string{"filesystem.write"}, "internet.search"); err == nil {
		t.Fatal("expected a tool outside the automation grant to be rejected")
	}
	if _, err := UnattendedPolicy(profile, []string{"filesystem.write"}, "filesystem.write"); err == nil {
		t.Fatal("expected a grant to be unable to enable a write tool")
	}
	if _, err := UnattendedPolicy(profile, []string{"git.commit"}, "git.commit"); err == nil {
		t.Fatal("expected a grant to be unable to enable a denied tool")
	}
	policy, err = UnattendedPolicy(profile, []string{"filesystem.read"}, "filesystem.read")
	if err != nil || policy != PolicyAllow {
		t.Fatalf("granted read policy=%q err=%v", policy, err)
	}
	if _, err := UnattendedPolicy(profile, nil, "not.a.tool"); err == nil {
		t.Fatal("expected an unknown tool to be rejected")
	}
}

func TestForUnattendedHidesWriteAndPromptTools(t *testing.T) {
	profile := contracts.AIProfile{
		Tools: []contracts.ToolPolicy{
			{ToolID: "internet.search", Policy: PolicyAllow},
			{ToolID: "filesystem.write", Policy: PolicyAllow},
			{ToolID: "terminal", Policy: PolicyAsk},
		},
	}
	narrowed := ForUnattended(profile, nil, map[string]struct{}{"internet.open": {}})
	prompt := PromptFor(narrowed)
	if !strings.Contains(prompt, "internet.search") {
		t.Fatalf("prompt = %s", prompt)
	}
	if strings.Contains(prompt, "filesystem.write") || strings.Contains(prompt, "terminal") {
		t.Fatalf("prompt advertised a tool the run cannot use: %s", prompt)
	}
	if profile.Tools[1].Policy != PolicyAllow {
		t.Fatal("narrowing changed the stored profile")
	}
}
