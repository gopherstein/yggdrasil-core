package app

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/yeixio/toskar-core/internal/egress"
	"github.com/yeixio/toskar-core/internal/topiclog"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

// Off-topic attempts (#345): what an Enforce profile held, counted by day
// and by where it came from, with the messages, so an Admin sees who tries
// to take it off topic and when the topic is set too narrowly.

// TopicWhere is where attempts came from: a portal, an API key, a person
// in the app, or an automation.
type TopicWhere struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// TopicAttemptView is an attempt with where it came from.
type TopicAttemptView struct {
	topiclog.Attempt
	Where TopicWhere `json:"where"`
}

// TopicDay is a day's count of attempts.
type TopicDay struct {
	Day   string `json:"day"`
	Count int    `json:"count"`
}

// TopicWhereCount is how many attempts came from one place.
type TopicWhereCount struct {
	TopicWhere
	Count int `json:"count"`
}

// TopicActivity is a profile's off-topic attempts over some days.
type TopicActivity struct {
	Days    int               `json:"days"`
	Total   int               `json:"total"`
	ByDay   []TopicDay        `json:"by_day"`
	ByWhere []TopicWhereCount `json:"by_where"`
	// Attempts are the latest, newest first.
	Attempts []TopicAttemptView `json:"attempts"`
}

// Where kinds.
const (
	wherePortal     = "portal"
	whereKey        = "key"
	wherePerson     = "person"
	whereAutomation = "automation"
)

// maxShownAttempts is how many messages the activity lists.
const maxShownAttempts = 100

// TopicAttempts is a profile's off-topic attempts over the last days.
func (a *App) TopicAttempts(ctx context.Context, profileID string, days int) (TopicActivity, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	if _, err := a.Profiles.Get(ctx, profileID); err != nil {
		return TopicActivity{}, err
	}
	list, err := a.TopicLog.List(ctx, profileID, time.Now().AddDate(0, 0, -days), 0)
	if err != nil {
		return TopicActivity{}, err
	}
	out := TopicActivity{Days: days, Total: len(list), ByDay: []TopicDay{}, ByWhere: []TopicWhereCount{}, Attempts: []TopicAttemptView{}}
	names := a.whereNames(ctx)
	byDay := map[string]int{}
	byWhere := map[TopicWhere]int{}
	for i, at := range list {
		w := names.of(at)
		byDay[at.At.Local().Format("2006-01-02")]++
		byWhere[w]++
		if i < maxShownAttempts {
			out.Attempts = append(out.Attempts, TopicAttemptView{Attempt: at, Where: w})
		}
	}
	for day, n := range byDay {
		out.ByDay = append(out.ByDay, TopicDay{day, n})
	}
	sort.Slice(out.ByDay, func(i, j int) bool { return out.ByDay[i].Day < out.ByDay[j].Day })
	for w, n := range byWhere {
		out.ByWhere = append(out.ByWhere, TopicWhereCount{w, n})
	}
	sort.Slice(out.ByWhere, func(i, j int) bool {
		if out.ByWhere[i].Count != out.ByWhere[j].Count {
			return out.ByWhere[i].Count > out.ByWhere[j].Count
		}
		return out.ByWhere[i].Name < out.ByWhere[j].Name
	})
	return out, nil
}

// whereNamer names portals, keys, and people once per listing.
type whereNamer struct {
	portals, keys, people map[string]string
}

func (a *App) whereNames(ctx context.Context) whereNamer {
	n := whereNamer{portals: map[string]string{}, keys: map[string]string{}, people: map[string]string{}}
	if a.Portals != nil {
		if list, err := a.Portals.List(ctx); err == nil {
			for _, p := range list {
				n.portals[p.ID] = p.Name
			}
		}
	}
	if a.APIKeys != nil {
		if list, err := a.APIKeys.List(ctx); err == nil {
			for _, k := range list {
				n.keys[k.ID] = k.Name
			}
		}
	}
	if a.People != nil {
		if list, err := a.People.List(ctx); err == nil {
			for _, p := range list {
				n.people[p.ID] = p.Name
			}
		}
	}
	return n
}

func (n whereNamer) of(at topiclog.Attempt) TopicWhere {
	switch {
	case at.PortalID != "":
		return TopicWhere{Kind: wherePortal, ID: at.PortalID, Name: n.portals[at.PortalID]}
	case at.KeyID != "":
		return TopicWhere{Kind: whereKey, ID: at.KeyID, Name: n.keys[at.KeyID]}
	case at.Source == egress.SourceAutomation:
		return TopicWhere{Kind: whereAutomation}
	default:
		return TopicWhere{Kind: wherePerson, ID: at.PersonID, Name: n.people[at.PersonID]}
	}
}

// ErrNoAttempt is an attempt that isn't this profile's, or is gone.
var ErrNoAttempt = contracts.NewError("ATTEMPT_NOT_FOUND", nil, errors.New("that attempt is gone"))

// ErrExamplesFull is a profile with as many example questions as it can
// have.
var ErrExamplesFull = contracts.NewError("EXAMPLES_FULL", nil, errors.New("the profile has 20 example questions; remove one to add this"))

// MarkOnTopic adds an attempt's message to the profile's example
// questions, so the checks count it as on topic from now on, and forgets
// the attempts with that message.
func (a *App) MarkOnTopic(ctx context.Context, profileID, attemptID string) error {
	at, err := a.TopicLog.Get(ctx, attemptID)
	if err != nil || at.ProfileID != profileID {
		return ErrNoAttempt
	}
	profile, err := a.Profiles.Get(ctx, profileID)
	if err != nil {
		return err
	}
	if profile.Topics == nil {
		return errors.New("this profile has no topic controls")
	}
	example := strings.TrimSpace(at.Message)
	if r := []rune(example); len(r) > 200 {
		example = strings.TrimSpace(string(r[:200]))
	}
	if !slices.Contains(profile.Topics.Examples, example) {
		if len(profile.Topics.Examples) >= 20 {
			return ErrExamplesFull
		}
		topics := *profile.Topics
		topics.Examples = append(append([]string(nil), topics.Examples...), example)
		profile.Topics = &topics
		if err := a.Profiles.Update(ctx, profile); err != nil {
			return err
		}
	}
	return a.TopicLog.ForgetMessage(ctx, profileID, at.Message)
}
