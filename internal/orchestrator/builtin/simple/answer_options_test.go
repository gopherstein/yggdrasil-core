package simple

import (
	"context"
	"testing"

	"github.com/yeixio/toskar-core/pkg/contracts"
	"github.com/yeixio/toskar-core/pkg/pluginapi"
)

// An API caller's temperature and max_tokens (#70) go to the calls that
// write the answer; Deliberate's drafts keep their own settings.
func TestAnswerOptionsReachTheAnswer(t *testing.T) {
	env := newRoleEnv(map[string]string{"assistant": "Final answer: $26", "drafter:2": "Final answer: $26", "drafter:3": "Final answer: $26"})
	want := pluginapi.GenerateOptions{Temperature: pluginapi.GreedyTemperature, MaxTokens: 64}
	ctx := pluginapi.WithAnswerOptions(context.Background(), want)
	events, err := New().Run(ctx, contracts.Task{Prompt: muffins}, contracts.AIProfile{
		Roles:         []contracts.ModelRole{{Role: "assistant", ModelID: "m"}},
		Orchestration: always,
	}, env)
	if err != nil {
		t.Fatal(err)
	}
	for range events {
	}
	if got := env.opts["assistant"]; got != want {
		t.Errorf("the answer's options = %+v, want %+v", got, want)
	}
	if got := env.opts["drafter:2"]; got.Temperature != 0.6 {
		t.Errorf("draft 2's options = %+v, want its own temperature", got)
	}
}

func TestSamplingTemperature(t *testing.T) {
	for in, want := range map[float64]struct {
		t  float64
		ok bool
	}{0: {0, false}, 0.7: {0.7, true}, pluginapi.GreedyTemperature: {0, true}} {
		if got, ok := pluginapi.SamplingTemperature(in); got != want.t || ok != want.ok {
			t.Errorf("SamplingTemperature(%v) = %v, %v", in, got, ok)
		}
	}
}
