package app

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/yeixio/yggdrasil-core/internal/automations"
	"github.com/yeixio/yggdrasil-core/internal/store"
	"github.com/yeixio/yggdrasil-core/internal/store/repositories"
)

func TestAutomationNotifierHonorsTaskSetting(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	settings := repositories.NewSettingsRepo(db.SQL)
	ctx := context.Background()
	sent := 0
	n := automationNotifier{
		settings: settings,
		send: noticeFunc(func(context.Context, automations.Notice) error {
			sent++
			return nil
		}),
	}
	if err := n.Notify(ctx, automations.Notice{Title: "Morning price", Body: "The laptop is $420."}); err != nil {
		t.Fatal(err)
	}
	if sent != 1 {
		t.Fatalf("sent = %d", sent)
	}
	if err := settings.SetBool(ctx, "notify_task_finish", false); err != nil {
		t.Fatal(err)
	}
	err = n.Notify(ctx, automations.Notice{Title: "Morning price", Body: "The laptop is $420."})
	if !errors.Is(err, automations.ErrNotifyDisabled) {
		t.Fatalf("disabled notify err = %v", err)
	}
	if sent != 1 {
		t.Fatalf("sent while notifications are off = %d", sent)
	}
}

type noticeFunc func(context.Context, automations.Notice) error

func (f noticeFunc) Notify(ctx context.Context, notice automations.Notice) error {
	return f(ctx, notice)
}
