package app

import (
	"context"
	"fmt"
	"time"

	"github.com/yeixio/yggdrasil-core/internal/events"
	"github.com/yeixio/yggdrasil-core/internal/locale"
	modelhealth "github.com/yeixio/yggdrasil-core/internal/models/health"
	"github.com/yeixio/yggdrasil-core/internal/share"
)

// EventWorkWaiting reports work queued behind higher-priority work (§60).
const EventWorkWaiting = "work.waiting"

// enterWork admits work to this computer by priority (spec §60). Work that
// has to wait publishes why. Without a gate, everything runs at once.
func (a *App) enterWork(ctx context.Context, class share.Class, label string, waiting func(reason string)) (*share.Work, error) {
	if a.Share == nil {
		return nil, nil
	}
	return a.Share.Enter(ctx, class, label, func(reason string) {
		if a.Logger != nil {
			a.Logger.Info("work waiting", "class", class.String(), "label", label, "reason", reason)
		}
		if a.Bus != nil {
			a.Bus.Publish(events.New(EventWorkWaiting, map[string]any{"class": class.String(), "label": label, "reason": reason}))
		}
		if waiting != nil {
			waiting(reason)
		}
	})
}

// trainingNow describes a training run holding this computer, such as
// `Training "Tire shop" is using this computer (about 12 minutes left)`.
func (a *App) trainingNow() (string, bool) {
	if a.Share == nil {
		return "", false
	}
	w, ok := a.Share.Running(share.Training)
	if !ok {
		return "", false
	}
	what := "Training"
	if w.Label() != "" {
		what = fmt.Sprintf("Training %q", w.Label())
	}
	return what + " is using this computer" + aboutLeft(w), true
}

// trainingStep says, in the App language lang, that a training run holding
// this computer may slow the answer, and about how long it has left.
func (a *App) trainingStep(lang string) (string, bool) {
	if a.Share == nil {
		return "", false
	}
	w, ok := a.Share.Running(share.Training)
	if !ok {
		return "", false
	}
	msg := locale.Message{Body: []locale.Text{locale.Key("chat:steps.training", nil)}}
	if w.Label() != "" {
		msg.Body[0] = locale.Key("chat:steps.trainingNamed", map[string]any{"name": w.Label()})
	}
	if m, ok := minutesLeft(w); ok {
		switch {
		case m < 1:
			msg.Body = append(msg.Body, locale.Key("chat:steps.leftUnderMinute", nil))
		case m < 120:
			msg.Body = append(msg.Body, locale.Key("chat:steps.leftMinutes", map[string]any{"count": m}))
		default:
			msg.Body = append(msg.Body, locale.Key("chat:steps.leftHours", map[string]any{"count": (m + 30) / 60}))
		}
	}
	_, body := msg.Render(lang)
	return body, true
}

// minutesLeft is a work's time left, rounded to minutes.
func minutesLeft(w *share.Work) (int, bool) {
	d, ok := w.Remaining()
	return int((d + 30*time.Second) / time.Minute), ok
}

func aboutLeft(w *share.Work) string {
	m, ok := minutesLeft(w)
	if !ok {
		return ""
	}
	switch {
	case m < 1:
		return " (less than a minute left)"
	case m == 1:
		return " (about 1 minute left)"
	case m < 120:
		return fmt.Sprintf(" (about %d minutes left)", m)
	default:
		return fmt.Sprintf(" (about %d hours left)", (m+30)/60)
	}
}

// explainWhileTraining rewrites an out-of-memory failure that happened while
// training holds this computer, so the person knows why and when to retry.
func (a *App) explainWhileTraining(errText string) string {
	busy, ok := a.trainingNow()
	if !ok || !modelhealth.OutOfMemory(errText) {
		return errText
	}
	return modelhealth.WithMessage(errText, busy+", so there was not enough memory for this model. Try again when training finishes, choose a smaller model, or cancel training on the Train page.")
}
