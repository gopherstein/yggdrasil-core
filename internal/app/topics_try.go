package app

import (
	"context"
	"errors"
	"strings"

	"github.com/yeixio/toskar-core/internal/orchestrator/builtin/simple"
	"github.com/yeixio/toskar-core/internal/profiles"
	"github.com/yeixio/toskar-core/internal/tools"
	"github.com/yeixio/toskar-core/pkg/contracts"
	"github.com/yeixio/toskar-core/pkg/pluginapi"
)

// TopicTrial is what a profile's topic controls do with one message, for
// the profile editor's Try it panel (#345).
type TopicTrial struct {
	// Label is what the check found: on_topic, small_talk, off_topic, or
	// "" when it gave nothing usable.
	Label string `json:"label"`
	// Held is an off-topic message an Enforce profile answers with the set
	// reply, without the full answer.
	Held bool `json:"held"`
	// Replaced is an answer that went off topic and was replaced with the
	// set reply.
	Replaced bool   `json:"replaced"`
	Reply    string `json:"reply"`
}

// ErrTrialMessage is an empty or too long message to try.
var ErrTrialMessage = errors.New("type a message of up to 2000 characters to try")

// TryTopics runs one message past a profile's topic controls, or draft
// ones in their place, as a chat would: the check is always run, so Guide
// shows its label too, and Enforce holds or replaces as it would. The
// answer is a quick one, from the topic rules alone, without tools,
// knowledge, memories, or the person's style, and nothing is saved.
func (a *App) TryTopics(ctx context.Context, profileID string, draft *contracts.TopicPolicy, message string) (TopicTrial, error) {
	message = strings.TrimSpace(message)
	if message == "" || len([]rune(message)) > 2000 {
		return TopicTrial{}, ErrTrialMessage
	}
	profile, err := a.Profiles.Get(ctx, profileID)
	if err != nil {
		return TopicTrial{}, err
	}
	if draft != nil {
		if err := profiles.ValidateTopics(draft); err != nil {
			return TopicTrial{}, err
		}
		normalized := profiles.Normalize(profiles.Profile{Name: profile.Name, Topics: draft})
		profile.Topics = normalized.Topics
	}
	if profile.Topics == nil {
		return TopicTrial{}, errors.New("this profile has no topic controls to try")
	}
	env := &chatExecEnv{app: a, ctx: ctx, profile: profile, turnPrompt: message, trace: &turnTrace{lang: a.appLanguage(ctx)}}
	role := simple.PickRole(profile.Roles)
	v := env.topicCheck(ctx, role, "each message a person sends", "The person's latest message", message)
	out := TopicTrial{Label: v.label}
	if enforcing(profile.Topics) && v.label == topicOff {
		out.Held, out.Reply = true, env.offTopicReply(ctx, v)
		return out, nil
	}
	system := topicBlock(profile.Topics) + "\n\n" +
		a.replyLanguage(ctx, "", message, "").Instruction() + "\n\n" +
		"Answer briefly, in plain text."
	answer, err := collectText(ctx, env, role, []pluginapi.ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: message},
	})
	if err != nil {
		return TopicTrial{}, err
	}
	answer = strings.TrimSpace(tools.VisibleText(answer))
	out.Reply = answer
	if checked := env.CheckAnswer(ctx, role, message, answer); checked != answer {
		out.Replaced, out.Reply = true, checked
	}
	return out, nil
}
