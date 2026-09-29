package app

import (
	"context"

	"github.com/yeixio/yggdrasil-core/internal/automations"
)

type boolSettings interface {
	GetBool(ctx context.Context, key string, def bool) (bool, error)
}

// automationNotifier delivers a scheduled-task notice when the user still wants them.
// The daemon posts it, so the notice can appear while the desktop UI is closed.
type automationNotifier struct {
	settings boolSettings
	send     automations.Notifier
}

func (n automationNotifier) Notify(ctx context.Context, notice automations.Notice) error {
	if n.settings != nil {
		ok, err := n.settings.GetBool(ctx, "notify_task_finish", true)
		if err != nil {
			return err
		}
		if !ok {
			return automations.ErrNotifyDisabled
		}
	}
	if n.send == nil {
		return automations.ErrNotifyDisabled
	}
	return n.send.Notify(ctx, notice)
}
