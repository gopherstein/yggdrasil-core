package app

import (
	"context"
	"fmt"

	"github.com/yeixio/toskar-core/internal/artifacts"
	"github.com/yeixio/toskar-core/internal/events"
	"github.com/yeixio/toskar-core/internal/locale"
	"github.com/yeixio/toskar-core/internal/structured"
	"github.com/yeixio/toskar-core/pkg/contracts"
	"github.com/yeixio/toskar-core/pkg/pluginapi"
)

// A picture sent to a chat with no model that can see it used to be
// answered blind, with a notice to install one on the Models page. Now,
// when a model that sees fits this computer, Toskar offers to install it,
// and the app sends the question again with its pictures once it's ready
// (#529), as for images and clips (§29).

// visionOption is the model to offer for seeing pictures: the smallest
// catalog model with an image encoder that fits this computer, since the
// person is waiting for it.
func (a *App) visionOption(ctx context.Context) (contracts.Model, int64, bool) {
	if a.Models == nil {
		return contracts.Model{}, 0, false
	}
	list, err := a.Models.List(ctx)
	if err != nil {
		return contracts.Model{}, 0, false
	}
	total := a.memoryTotal(ctx)
	var best contracts.Model
	var bestSize int64
	for _, m := range list {
		entry, ok := a.Models.Catalog().Get(m.ID)
		if !ok || entry.Projector == nil || m.Installed || !m.Capabilities.Vision {
			continue
		}
		if total > 0 && m.MemoryNeeded > 0 && float64(m.MemoryNeeded) > float64(total)*0.6 {
			continue
		}
		size := int64(m.SizeBytes + entry.Projector.SizeBytes)
		if bestSize == 0 || size < bestSize {
			best, bestSize = m, size
		}
	}
	return best, bestSize, bestSize > 0
}

// offerVision answers a message sent with pictures no installed model can
// see with an offer to install one, when one fits; only chats in the app
// get it, as for setup offers. A follow-up about pictures sent before
// isn't offered it again.
func (a *App) offerVision(ctx context.Context, conversationID, message, lang string, attached, pictures []artifacts.Artifact) (<-chan pluginapi.ChatChunk, bool) {
	if conversationID == "" || len(structured.SchemaFrom(ctx)) > 0 || len(attached) == 0 || len(pictures) == 0 {
		return nil, false
	}
	m, size, ok := a.visionOption(ctx)
	if !ok {
		return nil, false
	}
	ids := make([]string, 0, len(pictures))
	for _, p := range pictures {
		ids = append(ids, p.ID)
	}
	label := locale.T(lang, "chat:setup.visionLabel", nil)
	offer := &contracts.SetupOffer{Ability: "vision", Label: label, Option: m.ID, Name: m.DisplayName,
		SizeBytes: size, Request: message, Attachments: ids}
	reply := locale.T(lang, "chat:setup.visionOffer", map[string]any{"name": m.DisplayName, "size": fmt.Sprintf("%.1f GB", float64(size)/1e9)})
	meta := &contracts.MessageMeta{Setup: offer,
		Steps: []contracts.ActivityStep{{Kind: "share", Text: locale.T(lang, "chat:setup.visionStep", nil)}}}
	// The question keeps its pictures, in the chat and for the retry.
	if save, _ := a.Settings.GetBool(ctx, "save_chat_history", true); save {
		var userMeta *contracts.MessageMeta
		if len(attached) > 0 {
			userMeta = &contracts.MessageMeta{Files: fileRefs(attached)}
		}
		_, _ = a.Conversations.AddMessageWithMeta(ctx, conversationID, "user", message, userMeta)
		_, _ = a.Conversations.AddMessageWithMeta(ctx, conversationID, "assistant", reply, meta)
	}
	a.publish(ctx, events.New(events.ChatToken, map[string]any{"conversation_id": conversationID, "content": reply}))
	a.publish(ctx, events.New(events.ChatComplete, map[string]any{"conversation_id": conversationID, "meta": meta}))
	ch := make(chan pluginapi.ChatChunk, 2)
	ch <- pluginapi.ChatChunk{Content: reply}
	ch <- pluginapi.ChatChunk{Done: true}
	close(ch)
	return ch, true
}
