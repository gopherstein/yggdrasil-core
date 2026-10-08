package app

import (
	"context"
	"fmt"
	"time"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/automations"
	"github.com/yeixio/toskar-core/internal/store/repositories"
)

// systemAutomations is the scheduler's view of automations (#206): it finds
// what's due across everyone, then each runs as its own person.
type systemAutomations struct {
	*repositories.AutomationRepo
}

func (s systemAutomations) Get(ctx context.Context, id string) (automations.Automation, error) {
	return s.AutomationRepo.Get(auth.WithSystem(ctx), id)
}

func (s systemAutomations) Due(ctx context.Context, now time.Time) ([]automations.Automation, error) {
	return s.AutomationRepo.Due(auth.WithSystem(ctx), now)
}

func (s systemAutomations) RefreshNextRuns(ctx context.Context, now time.Time) error {
	return s.AutomationRepo.RefreshNextRuns(auth.WithSystem(ctx), now)
}

func (s systemAutomations) Checked(ctx context.Context, id string, state []byte, checkedAt, now time.Time) error {
	return s.AutomationRepo.Checked(auth.WithSystem(ctx), id, state, checkedAt, now)
}

func (s systemAutomations) Followers(ctx context.Context, id string) ([]automations.Automation, error) {
	return s.AutomationRepo.Followers(auth.WithSystem(ctx), id)
}

// asAutomationPerson is ctx acting for the person whose automation it is.
// A disabled person's automations don't run.
func (a *App) asAutomationPerson(ctx context.Context, automation automations.Automation) (context.Context, error) {
	id := automation.PersonID
	if id == "" {
		id = auth.OwnerID
	}
	person := auth.Person{ID: id, Role: auth.RoleOwner}
	if a.People != nil {
		p, err := a.People.Active(ctx, id)
		if err != nil {
			return ctx, fmt.Errorf("this automation's person can no longer use Toskar")
		}
		person = p
	}
	return auth.AsPerson(ctx, person), nil
}
