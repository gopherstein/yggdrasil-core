package automations

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNextRun(t *testing.T) {
	friday := int(time.Friday)
	anchor := mustTime(t, "2026-09-21T15:00:00Z")
	cases := []struct {
		name    string
		sched   Schedule
		created time.Time
		last    time.Time
		now     time.Time
		want    time.Time
		ok      bool
	}{
		{
			name:    "once in the future",
			sched:   Schedule{Kind: KindOnce, TimeZone: "UTC", At: timePtr(mustTime(t, "2026-09-22T16:00:00Z"))},
			created: mustTime(t, "2026-09-21T16:00:00Z"),
			now:     mustTime(t, "2026-09-21T16:00:00Z"),
			want:    mustTime(t, "2026-09-22T16:00:00Z"),
			ok:      true,
		},
		{
			name:    "once already due",
			sched:   Schedule{Kind: KindOnce, TimeZone: "UTC", At: timePtr(mustTime(t, "2026-09-21T15:00:00Z"))},
			created: mustTime(t, "2026-09-21T16:00:00Z"),
			now:     mustTime(t, "2026-09-21T16:00:00Z"),
			want:    mustTime(t, "2026-09-21T15:00:00Z"),
			ok:      true,
		},
		{
			name:    "once already ran",
			sched:   Schedule{Kind: KindOnce, TimeZone: "UTC", At: timePtr(mustTime(t, "2026-09-21T15:00:00Z"))},
			created: mustTime(t, "2026-09-21T14:00:00Z"),
			last:    mustTime(t, "2026-09-21T15:00:05Z"),
			now:     mustTime(t, "2026-09-21T16:00:00Z"),
			ok:      false,
		},
		{
			name: "daily waits for the next morning when today's slot already passed",
			sched: Schedule{
				Kind: KindDaily, TimeZone: "America/Los_Angeles", Hour: 8,
			},
			created: mustTime(t, "2026-09-21T16:00:00Z"),
			now:     mustTime(t, "2026-09-21T16:00:00Z"),
			want:    mustTime(t, "2026-09-22T15:00:00Z"),
			ok:      true,
		},
		{
			name: "daily catches the latest missed morning",
			sched: Schedule{
				Kind: KindDaily, TimeZone: "America/Los_Angeles", Hour: 8,
			},
			created: mustTime(t, "2026-09-21T14:00:00Z"),
			last:    mustTime(t, "2026-09-21T15:00:10Z"),
			now:     mustTime(t, "2026-09-24T17:00:00Z"),
			want:    mustTime(t, "2026-09-24T15:00:00Z"),
			ok:      true,
		},
		{
			name: "daily already finished today",
			sched: Schedule{
				Kind: KindDaily, TimeZone: "America/Los_Angeles", Hour: 8,
			},
			created: mustTime(t, "2026-09-21T14:00:00Z"),
			last:    mustTime(t, "2026-09-24T15:00:20Z"),
			now:     mustTime(t, "2026-09-24T17:00:00Z"),
			want:    mustTime(t, "2026-09-25T15:00:00Z"),
			ok:      true,
		},
		{
			name: "weekly friday is due once after a missed week",
			sched: Schedule{
				Kind: KindWeekly, TimeZone: "America/Los_Angeles", Hour: 8, Weekday: &friday,
			},
			created: mustTime(t, "2026-09-23T16:00:00Z"),
			now:     mustTime(t, "2026-09-26T16:00:00Z"),
			want:    mustTime(t, "2026-09-25T15:00:00Z"),
			ok:      true,
		},
		{
			name: "interval keeps the latest missed slot",
			sched: Schedule{
				Kind: KindInterval, TimeZone: "UTC", EverySeconds: 6 * 60 * 60, Anchor: &anchor,
			},
			created: mustTime(t, "2026-09-21T14:00:00Z"),
			now:     mustTime(t, "2026-09-21T21:30:00Z"),
			want:    mustTime(t, "2026-09-21T21:00:00Z"),
			ok:      true,
		},
		{
			name: "interval after the latest slot waits for the next one",
			sched: Schedule{
				Kind: KindInterval, TimeZone: "UTC", EverySeconds: 6 * 60 * 60, Anchor: &anchor,
			},
			created: mustTime(t, "2026-09-21T14:00:00Z"),
			last:    mustTime(t, "2026-09-21T21:00:05Z"),
			now:     mustTime(t, "2026-09-21T21:30:00Z"),
			want:    mustTime(t, "2026-09-22T03:00:00Z"),
			ok:      true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok, err := tc.sched.NextRun(tc.created, tc.last, tc.now)
			if err != nil {
				t.Fatalf("NextRun: %v", err)
			}
			if ok != tc.ok {
				t.Fatalf("ok = %v, want %v (got %s)", ok, tc.ok, got)
			}
			if !ok {
				return
			}
			if !got.Equal(tc.want) {
				t.Fatalf("next = %s, want %s", got.Format(time.RFC3339), tc.want.Format(time.RFC3339))
			}
		})
	}
}

func TestDailySpringForwardUsesCivilTime(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	// 02:30 does not exist on 2026-03-08. The civil constructor normalizes it.
	want := time.Date(2026, 3, 8, 2, 30, 0, 0, loc).UTC().Truncate(time.Second)
	sched := Schedule{Kind: KindDaily, TimeZone: "America/Los_Angeles", Hour: 2, Minute: 30}
	got, ok, err := sched.NextRun(
		mustTime(t, "2026-03-07T18:00:00Z"),
		time.Time{},
		mustTime(t, "2026-03-08T18:00:00Z"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || !got.Equal(want) {
		t.Fatalf("next = %s ok=%v, want civil instant %s", got.Format(time.RFC3339), ok, want.Format(time.RFC3339))
	}
}

func TestScheduleRejectsUnknownZone(t *testing.T) {
	err := (Schedule{Kind: KindDaily, TimeZone: "Not/AZone", Hour: 8}).Validate()
	if err == nil {
		t.Fatal("expected invalid time zone")
	}
}

func TestScheduleJSONKeepsMidnight(t *testing.T) {
	in := Schedule{Kind: KindDaily, TimeZone: "UTC"}
	body, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var got Schedule
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	next, ok, err := got.NextRun(
		mustTime(t, "2026-09-21T00:30:00Z"),
		time.Time{},
		mustTime(t, "2026-09-21T00:30:00Z"),
	)
	if err != nil || !ok {
		t.Fatalf("NextRun ok=%v err=%v", ok, err)
	}
	want := mustTime(t, "2026-09-22T00:00:00Z")
	if !next.Equal(want) {
		t.Fatalf("next = %s, want %s", next.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func timePtr(t time.Time) *time.Time { return &t }
