package automations

import (
	"fmt"
	"time"
)

// Kind is the v1 schedule shape. Price checks, stock checks, and one-shot research
// are all one of these, not separate product features.
type Kind string

const (
	KindOnce     Kind = "once"
	KindDaily    Kind = "daily"
	KindWeekly   Kind = "weekly"
	KindInterval Kind = "interval"
)

// Schedule is the time rule for an automation.
// Clock fields are civil time in TimeZone. At and Anchor are absolute instants.
type Schedule struct {
	Kind         Kind       `json:"kind"`
	TimeZone     string     `json:"time_zone"`
	At           *time.Time `json:"at,omitempty"`
	Hour         int        `json:"hour,omitempty"`
	Minute       int        `json:"minute,omitempty"`
	Weekday      *int       `json:"weekday,omitempty"`
	EverySeconds int        `json:"every_seconds,omitempty"`
	Anchor       *time.Time `json:"anchor,omitempty"`
}

// Validate checks that the schedule can produce a next run.
func (s Schedule) Validate() error {
	switch s.Kind {
	case KindOnce, KindDaily, KindWeekly, KindInterval:
	case "":
		return fmt.Errorf("schedule kind is required")
	default:
		return fmt.Errorf("unsupported schedule kind %q", s.Kind)
	}
	if s.TimeZone == "" {
		return fmt.Errorf("time zone is required")
	}
	if _, err := time.LoadLocation(s.TimeZone); err != nil {
		return fmt.Errorf("time zone %q: %w", s.TimeZone, err)
	}
	switch s.Kind {
	case KindOnce:
		if s.At == nil || s.At.IsZero() {
			return fmt.Errorf("one-time schedule requires at")
		}
	case KindDaily, KindWeekly:
		if s.Hour < 0 || s.Hour > 23 {
			return fmt.Errorf("hour must be 0-23")
		}
		if s.Minute < 0 || s.Minute > 59 {
			return fmt.Errorf("minute must be 0-59")
		}
		if s.Kind == KindWeekly {
			if s.Weekday == nil {
				return fmt.Errorf("weekly schedule requires weekday")
			}
			if *s.Weekday < 0 || *s.Weekday > 6 {
				return fmt.Errorf("weekday must be 0-6")
			}
		}
	case KindInterval:
		if s.EverySeconds < 1 {
			return fmt.Errorf("interval must be at least 1 second")
		}
		const maxSeconds = int64(1<<63-1) / int64(time.Second)
		if int64(s.EverySeconds) > maxSeconds {
			return fmt.Errorf("interval is too long")
		}
	}
	return nil
}

// NextRun chooses the single occurrence the daemon should execute.
//
// createdAt ignores recurring slots from before the automation existed.
// lastOccurrence is when the previous occurrence finished. The zero time means it has never run.
// When several slots fall after that bound and at or before now, NextRun returns the latest
// one so a restart runs the most recent missed occurrence once.
// The boolean is false when a one-time schedule has already run.
func (s Schedule) NextRun(createdAt, lastOccurrence, now time.Time) (time.Time, bool, error) {
	if err := s.Validate(); err != nil {
		return time.Time{}, false, err
	}
	createdAt = wall(createdAt)
	lastOccurrence = wall(lastOccurrence)
	now = wall(now)

	switch s.Kind {
	case KindOnce:
		if !lastOccurrence.IsZero() {
			return time.Time{}, false, nil
		}
		return wall(*s.At), true, nil
	case KindInterval:
		every := time.Duration(s.EverySeconds) * time.Second
		start := createdAt
		if s.Anchor != nil && !s.Anchor.IsZero() {
			start = wall(*s.Anchor)
		}
		if start.IsZero() {
			return time.Time{}, false, fmt.Errorf("interval schedule requires a created time or anchor")
		}
		return nextOnGrid(start, every, later(createdAt, lastOccurrence), now), true, nil
	default:
		loc, err := time.LoadLocation(s.TimeZone)
		if err != nil {
			return time.Time{}, false, fmt.Errorf("time zone %q: %w", s.TimeZone, err)
		}
		bound := later(createdAt, lastOccurrence)
		latest := latestCalendar(s, loc, now)
		if latest.After(bound) && !latest.After(now) {
			return wall(latest), true, nil
		}
		after := now
		if bound.After(after) {
			after = bound
		}
		return wall(nextCalendar(s, loc, after)), true, nil
	}
}

func wall(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	return t.UTC().Truncate(time.Second)
}

func later(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// nextOnGrid walks start, start+every, start+2every, ...
// It returns the latest point in (notBefore, now], or the next point after now when nothing is due.
func nextOnGrid(start time.Time, every time.Duration, notBefore, now time.Time) time.Time {
	first := firstAfter(start, notBefore, every)
	if first.After(now) || now.Before(start) {
		return first
	}
	elapsed := now.Sub(start)
	latest := now.Add(-(elapsed % every))
	if latest.After(notBefore) {
		return latest
	}
	return first
}

func firstAfter(start, after time.Time, every time.Duration) time.Time {
	if start.After(after) {
		return start
	}
	elapsed := after.Sub(start)
	rem := elapsed % every
	if rem == 0 {
		return after.Add(every)
	}
	return after.Add(every - rem)
}

func latestCalendar(s Schedule, loc *time.Location, now time.Time) time.Time {
	if s.Kind == KindWeekly {
		return nearestWeekday(s, loc, now, -1, false)
	}
	return nearestDay(s, loc, now, -1, false)
}

func nextCalendar(s Schedule, loc *time.Location, after time.Time) time.Time {
	if s.Kind == KindWeekly {
		return nearestWeekday(s, loc, after, 1, true)
	}
	return nearestDay(s, loc, after, 1, true)
}

// nearestDay walks civil days from now. forward is 1 or -1.
// strict means the slot must be after now; otherwise it must be at or before now.
func nearestDay(s Schedule, loc *time.Location, now time.Time, forward int, strict bool) time.Time {
	local := now.In(loc)
	for i := 0; i <= 3; i++ {
		slot := civilOnDay(local, i*forward, s.Hour, s.Minute, loc)
		if strict && slot.After(now) {
			return slot
		}
		if !strict && !slot.After(now) {
			return slot
		}
	}
	return civilOnDay(local, forward, s.Hour, s.Minute, loc)
}

func nearestWeekday(s Schedule, loc *time.Location, now time.Time, forward int, strict bool) time.Time {
	local := now.In(loc)
	want := time.Weekday(*s.Weekday)
	step := 1
	if forward < 0 {
		step = -1
	}
	for i := 0; i <= 10; i++ {
		slot := civilOnDay(local, i*step, s.Hour, s.Minute, loc)
		if slot.Weekday() != want {
			continue
		}
		if strict && slot.After(now) {
			return slot
		}
		if !strict && !slot.After(now) {
			return slot
		}
	}
	return civilOnDay(local, 7*step, s.Hour, s.Minute, loc)
}

// civilOnDay returns hour:minute on the civil day offset from local, using noon as the
// day anchor so a DST transition does not shift the calendar date.
func civilOnDay(local time.Time, dayOffset, hour, minute int, loc *time.Location) time.Time {
	base := time.Date(local.Year(), local.Month(), local.Day(), 12, 0, 0, 0, loc).AddDate(0, 0, dayOffset)
	return time.Date(base.Year(), base.Month(), base.Day(), hour, minute, 0, 0, loc)
}
