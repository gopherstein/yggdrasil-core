package app

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"strings"

	"github.com/yeixio/toskar-core/internal/artifacts"
	"github.com/yeixio/toskar-core/pkg/contracts"
	"github.com/yeixio/toskar-core/pkg/pluginapi"
)

// Pictures in chat (#191): a picture attached to a message is shown to a
// model that can see, one with its projector, and a message with pictures
// goes to such a model when the chosen one reads text only.

// maxTurnImages is how many pictures one message shows the model. Each costs
// hundreds of tokens of the window.
const maxTurnImages = 4

// imageMIME are the picture types llama.cpp reads, by extension.
var imageMIME = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".bmp":  "image/bmp",
}

// seeable reports a file a vision model can be shown.
func seeable(art artifacts.Artifact) bool {
	_, ok := imageMIME[strings.ToLower(filepath.Ext(art.Name))]
	return ok
}

// seesImages reports a model that can see pictures on this computer.
func (a *App) seesImages(modelID string) bool {
	return a.Models != nil && modelID != "" && a.Models.SeesImages(modelID)
}

// turnPictures are the pictures a message is about: the ones attached to it,
// or, when it has none, the ones attached to the message before it, so a
// follow-up ("and the one on the left?") still sees them.
func (a *App) turnPictures(ctx context.Context, conversationID string, attached []artifacts.Artifact) []artifacts.Artifact {
	var out []artifacts.Artifact
	for _, art := range attached {
		if seeable(art) && len(out) < maxTurnImages {
			out = append(out, art)
		}
	}
	if len(attached) > 0 || conversationID == "" || a.Conversations == nil || a.Artifacts == nil {
		return out
	}
	msgs, err := a.Conversations.ListMessages(ctx, conversationID)
	if err != nil {
		return nil
	}
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != "user" {
			continue
		}
		if msgs[i].Meta == nil {
			return nil
		}
		for _, f := range msgs[i].Meta.Files {
			art, err := a.Artifacts.Get(ctx, f.ID)
			if err == nil && art.ConversationID == conversationID && seeable(art) && len(out) < maxTurnImages {
				out = append(out, art)
			}
		}
		return out
	}
	return nil
}

// seeingModel is the installed model that can see pictures to answer with:
// the largest that fits this computer's memory, or else the smallest.
func (a *App) seeingModel(ctx context.Context) (contracts.Model, bool) {
	var fit, small *contracts.Model
	total := a.memoryTotal(ctx)
	models := a.installedModels(ctx)
	for i := range models {
		m := &models[i]
		if !m.Installed || !a.seesImages(m.ID) || a.recentlyFailed(m.ID) {
			continue
		}
		if total == 0 || m.MemoryNeeded == 0 || float64(m.MemoryNeeded) <= float64(total)*0.6 {
			if fit == nil || m.MemoryNeeded > fit.MemoryNeeded {
				fit = m
			}
		}
		if small == nil || m.MemoryNeeded < small.MemoryNeeded {
			small = m
		}
	}
	switch {
	case fit != nil:
		return *fit, true
	case small != nil:
		return *small, true
	}
	return contracts.Model{}, false
}

// imageURLs reads pictures as data URLs for ChatMessage.Images, with the
// pictures it could read.
func (a *App) imageURLs(ctx context.Context, pictures []artifacts.Artifact) ([]string, []artifacts.Artifact) {
	var urls []string
	var read []artifacts.Artifact
	for _, art := range pictures {
		_, data, err := a.Artifacts.Read(ctx, art.ID)
		if err != nil {
			continue
		}
		mime := imageMIME[strings.ToLower(filepath.Ext(art.Name))]
		urls = append(urls, "data:"+mime+";base64,"+base64.StdEncoding.EncodeToString(data))
		read = append(read, art)
	}
	return urls, read
}

// attachesPictures reports a message with a picture attached.
func (a *App) attachesPictures(ctx context.Context) bool {
	if a.Artifacts == nil {
		return false
	}
	for _, id := range artifacts.AttachmentsFrom(ctx) {
		if art, err := a.Artifacts.Get(ctx, id); err == nil && seeable(art) {
			return true
		}
	}
	return false
}

// withoutImages is messages with their pictures left out.
func withoutImages(messages []pluginapi.ChatMessage) []pluginapi.ChatMessage {
	var out []pluginapi.ChatMessage
	for i, m := range messages {
		if len(m.Images) == 0 {
			continue
		}
		if out == nil {
			out = append([]pluginapi.ChatMessage(nil), messages...)
		}
		out[i].Images = nil
	}
	if out == nil {
		return messages
	}
	return out
}
