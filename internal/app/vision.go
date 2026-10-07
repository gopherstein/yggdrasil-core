package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yeixio/toskar-core/internal/artifacts"
	"github.com/yeixio/toskar-core/internal/speech"
	"github.com/yeixio/toskar-core/pkg/contracts"
	"github.com/yeixio/toskar-core/pkg/pluginapi"
)

// Pictures in chat (#191): a picture attached to a message is shown to a
// model that can see, one with its projector, and a message with pictures
// goes to such a model when the chosen one reads text only. A video is shown
// as frames sampled through it.

// maxTurnImages is how many pictures, counting video frames, one message
// shows the model. Each costs hundreds of tokens of the window.
const maxTurnImages = 8

// Videos: how many one message shows, and frames from each.
const (
	maxTurnVideos  = 2
	framesPerVideo = 6
)

// videoExts are the videos whose frames can be read.
var videoExts = map[string]bool{".mp4": true, ".mov": true, ".m4v": true, ".webm": true, ".mkv": true}

// watchable reports a video a vision model can be shown frames of.
func watchable(art artifacts.Artifact) bool {
	return videoExts[strings.ToLower(filepath.Ext(art.Name))]
}

// visual reports a picture or a video.
func visual(art artifacts.Artifact) bool { return seeable(art) || watchable(art) }

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
	pictures, videos := 0, 0
	keep := func(art artifacts.Artifact) {
		switch {
		case seeable(art) && pictures < maxTurnImages:
			pictures++
			out = append(out, art)
		case watchable(art) && videos < maxTurnVideos && a.canWatch():
			videos++
			out = append(out, art)
		}
	}
	for _, art := range attached {
		keep(art)
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
			if err == nil && art.ConversationID == conversationID {
				keep(art)
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

// attachesPictures reports a message with a picture attached.
func (a *App) attachesPictures(ctx context.Context) bool {
	if a.Artifacts == nil {
		return false
	}
	for _, id := range artifacts.AttachmentsFrom(ctx) {
		if art, err := a.Artifacts.Get(ctx, id); err == nil && (seeable(art) || (watchable(art) && a.canWatch())) {
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

// canWatch reports that video frames can be read here: with the speech
// environment's PyAV, which a sandboxed copy may not include.
func (a *App) canWatch() bool {
	if a.Speech == nil {
		return false
	}
	ok, _ := a.Speech.Available()
	return ok
}

// look reads the message's pictures and video frames once, into images, and
// notes what each video's frames are for the attachment block.
func (e *chatExecEnv) look(ctx context.Context) {
	e.lookOnce.Do(func() {
		if e.app == nil || e.app.Artifacts == nil || len(e.pictures) == 0 {
			return
		}
		e.watched = map[string]string{}
		budget, videos := maxTurnImages, 0
		for _, art := range e.pictures {
			if watchable(art) {
				videos++
				continue
			}
			if budget == 0 {
				continue
			}
			_, data, err := e.app.Artifacts.Read(ctx, art.ID)
			if err != nil {
				continue
			}
			mime := imageMIME[strings.ToLower(filepath.Ext(art.Name))]
			e.images = append(e.images, "data:"+mime+";base64,"+base64.StdEncoding.EncodeToString(data))
			budget--
		}
		for _, art := range e.pictures {
			if !watchable(art) {
				continue
			}
			n := min(framesPerVideo, budget/max(videos, 1))
			videos--
			if n < 1 {
				continue
			}
			clip, err := e.app.watch(ctx, art, n)
			if err != nil {
				e.watched[art.ID] = "Its frames could not be read: " + err.Error() + "."
				continue
			}
			var at []string
			for _, f := range clip.Frames {
				e.images = append(e.images, "data:image/jpeg;base64,"+base64.StdEncoding.EncodeToString(f.JPEG))
				at = append(at, clock(f.Time))
			}
			budget -= len(clip.Frames)
			note := fmt.Sprintf("It is %s long. You are shown %d frames from it with this message, in order, at %s; look at them to answer.",
				clock(clip.Duration), len(clip.Frames), strings.Join(at, ", "))
			if clip.HasAudio {
				note += fmt.Sprintf(" To know what is said in it, call %s with {\"file\": %q}.", AudioToolID, art.Name)
			}
			e.watched[art.ID] = note
			if e.trace != nil {
				e.trace.watched(art)
			}
		}
	})
}

// clock is seconds as m:ss.
func clock(seconds float64) string {
	s := int(seconds + 0.5)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// frameCache keeps a few videos' frames, so a follow-up about the same clip
// doesn't read it again.
type frameCache struct {
	mu    sync.Mutex
	order []string
	clips map[string]speech.Clip
}

const framesCached = 4

// watch samples n frames from a video, or returns the ones read before.
func (a *App) watch(ctx context.Context, art artifacts.Artifact, n int) (speech.Clip, error) {
	key := fmt.Sprintf("%s/%d", art.ID, n)
	c := &a.frames
	c.mu.Lock()
	if clip, ok := c.clips[key]; ok {
		c.mu.Unlock()
		return clip, nil
	}
	c.mu.Unlock()
	_, data, err := a.Artifacts.Read(ctx, art.ID)
	if err != nil {
		return speech.Clip{}, err
	}
	clip, err := a.Speech.Frames(ctx, art.Name, data, n)
	if err != nil {
		return speech.Clip{}, err
	}
	if len(clip.Frames) == 0 {
		return speech.Clip{}, fmt.Errorf("no frames could be read")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.clips == nil {
		c.clips = map[string]speech.Clip{}
	}
	if _, ok := c.clips[key]; !ok {
		c.order = append(c.order, key)
	}
	c.clips[key] = clip
	for len(c.order) > framesCached {
		delete(c.clips, c.order[0])
		c.order = c.order[1:]
	}
	return clip, nil
}
