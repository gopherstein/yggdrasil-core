package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yeixio/yggdrasil-core/internal/events"
	"github.com/yeixio/yggdrasil-core/internal/gjallarhorn"
)

// Model crashes notify only when they repeat (Gjallarhorn §23, §29): one
// crash is handled by answering on another model.
const (
	crashWindow   = 30 * time.Minute
	crashRepeat   = 2
	crashCooldown = time.Hour
)

// healthNotices turns computer and model health into notifications on
// changes only (§23): a computer going offline and coming back, and a model
// that keeps crashing, instead of the same state again and again.
type healthNotices struct {
	mu       sync.Mutex
	now      func() time.Time
	offline  map[string]bool
	crashes  map[string][]time.Time
	notified map[string]time.Time
}

func newHealthNotices() *healthNotices {
	return &healthNotices{now: time.Now, offline: map[string]bool{}, crashes: map[string][]time.Time{}, notified: map[string]time.Time{}}
}

// notice returns the notification for a health event, if it is a change
// worth one. peerAlerts is the "Notify when a paired computer goes offline"
// setting.
func (h *healthNotices) notice(evt events.Event, name func(modelID string) string, peerAlerts bool) (gjallarhorn.Request, bool) {
	str := func(k string) string { v, _ := evt.Payload[k].(string); return v }
	h.mu.Lock()
	defer h.mu.Unlock()
	switch evt.Type {
	case events.NodeOffline:
		id := str("node_id")
		if id == "" || !peerAlerts || h.offline[id] {
			return gjallarhorn.Request{}, false
		}
		h.offline[id] = true
		return gjallarhorn.Request{SourceType: "node", SourceID: id, Category: gjallarhorn.CategoryHealth, Severity: gjallarhorn.SeverityWarning,
			Title: computerName(str("name")) + " went offline", Body: "Work that needs it runs on another computer, or waits until it is back.",
			Link: "/nodes", DedupeKey: "node.offline:" + id}, true
	case events.NodeOnline:
		id := str("node_id")
		if !h.offline[id] {
			// Only a computer that was announced offline is announced back.
			return gjallarhorn.Request{}, false
		}
		delete(h.offline, id)
		return gjallarhorn.Request{SourceType: "node", SourceID: id, Category: gjallarhorn.CategoryHealth, Severity: gjallarhorn.SeveritySuccess,
			Title: computerName(str("name")) + " is back online", Body: "It can run work again.",
			Link: "/nodes", DedupeKey: "node.online:" + id}, true
	case events.ModelHealthFailed:
		model := str("model_id")
		if model == "" {
			return gjallarhorn.Request{}, false
		}
		now := h.now()
		recent := h.crashes[model][:0]
		for _, t := range h.crashes[model] {
			if now.Sub(t) < crashWindow {
				recent = append(recent, t)
			}
		}
		recent = append(recent, now)
		h.crashes[model] = recent
		if len(recent) < crashRepeat || now.Sub(h.notified[model]) < crashCooldown {
			return gjallarhorn.Request{}, false
		}
		h.notified[model] = now
		body := fmt.Sprintf("It stopped %d times in the last %d minutes. Chats answered on another model.", len(recent), int(crashWindow.Minutes()))
		if memory, _ := evt.Payload["likely_memory_pressure"].(bool); memory {
			body += " It is probably running out of memory: close other apps, or choose a smaller model."
		}
		return gjallarhorn.Request{SourceType: "model", SourceID: model, Category: gjallarhorn.CategoryHealth, Severity: gjallarhorn.SeverityError,
			Title: name(model) + " keeps crashing", Body: body, Link: "/models", DedupeKey: "model.crashing:" + model}, true
	}
	return gjallarhorn.Request{}, false
}

func computerName(name string) string {
	if name == "" {
		return "A paired computer"
	}
	return name
}

// healthNotice is the notification for a health event, if any.
func (a *App) healthNotice(ctx context.Context, evt events.Event) (gjallarhorn.Request, bool) {
	if a.health == nil {
		return gjallarhorn.Request{}, false
	}
	peerAlerts := true
	if evt.Type == events.NodeOffline && a.Settings != nil {
		peerAlerts, _ = a.Settings.GetBool(ctx, "notify_peer_offline", true)
	}
	return a.health.notice(evt, a.modelName, peerAlerts)
}
