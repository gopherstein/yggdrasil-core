package gjallarhorn

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/events"
	"github.com/yeixio/toskar-core/internal/store"
)

type fakeChannel struct {
	name string
	err  error
	got  []Notification
}

func (c *fakeChannel) Name() string { return c.name }
func (c *fakeChannel) Deliver(_ context.Context, n Notification) error {
	c.got = append(c.got, n)
	return c.err
}

func newHub(t *testing.T, channels ...Channel) (*Hub, *events.Bus) {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	bus := events.NewBus(16)
	return NewHub(db.SQL, bus, channels...), bus
}

func TestNotifyStoresAnnouncesAndDelivers(t *testing.T) {
	desktop := &fakeChannel{name: "desktop"}
	broken := &fakeChannel{name: "email", err: errors.New("smtp down")}
	quiet := &fakeChannel{name: "push", err: ErrSuppressed}
	hub, bus := newHub(t, desktop, broken, quiet)
	subID, sub := bus.Subscribe()
	defer bus.Unsubscribe(subID)
	ctx := context.Background()

	n, err := hub.Notify(ctx, Request{SourceType: "automation", SourceID: "a1", Category: CategoryAutomation, Severity: SeveritySuccess,
		Title: "Daily report", Body: "Ready.", Link: "/automations?id=a1", Channels: []string{"desktop", "email", "push"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(desktop.got) != 1 || desktop.got[0].Title != "Daily report" {
		t.Fatal("desktop delivery")
	}
	select {
	case evt := <-sub:
		if evt.Type != EventCreated || evt.Payload["title"] != "Daily report" {
			t.Fatalf("event = %+v", evt)
		}
	case <-time.After(time.Second):
		t.Fatal("no event")
	}
	// A failed delivery does not erase the notification.
	got, err := hub.Get(ctx, n.ID)
	if err != nil || got.Link != "/automations?id=a1" || len(got.Deliveries) != 3 {
		t.Fatalf("stored = %+v %v", got, err)
	}
	status := map[string]string{}
	for _, d := range got.Deliveries {
		status[d.Channel] = d.Status
	}
	if status["desktop"] != DeliveryDelivered || status["email"] != DeliveryFailed || status["push"] != DeliverySuppressed {
		t.Fatalf("deliveries = %+v", got.Deliveries)
	}
}

func TestDedupeReadAndDismiss(t *testing.T) {
	hub, _ := newHub(t)
	ctx := context.Background()
	a, _ := hub.Notify(ctx, Request{Title: "Node offline", DedupeKey: "node:n1", Body: "first"})
	_ = hub.MarkRead(ctx, []string{a.ID})
	b, _ := hub.Notify(ctx, Request{Title: "Node offline", DedupeKey: "node:n1", Body: "again"})
	if a.ID != b.ID || b.ReadAt != nil || b.Body != "again" {
		t.Fatalf("a repeat folds into the first and is unread again: %+v", b)
	}
	_, _ = hub.Notify(ctx, Request{Title: "Download finished"})
	list, unread, err := hub.List(ctx, false, 0)
	if err != nil || len(list) != 2 || unread != 2 || list[0].Title != "Download finished" {
		t.Fatalf("list = %+v unread=%d %v", list, unread, err)
	}
	if err := hub.MarkRead(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if _, unread, _ := hub.List(ctx, false, 0); unread != 0 {
		t.Fatal("all read")
	}
	if err := hub.Dismiss(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if list, _, _ := hub.List(ctx, false, 0); len(list) != 1 {
		t.Fatal("dismissed notifications leave the center")
	}
	if _, err := hub.Notify(ctx, Request{}); err == nil {
		t.Fatal("a title is required")
	}
}

// Each person sees their own notices, Admins and the Owner also see the
// install's, and another person's own never leave the app (#206).
func TestNotificationsBelongToTheirPerson(t *testing.T) {
	desktop := &fakeChannel{name: "desktop"}
	hub, _ := newHub(t, desktop)
	owner := context.Background()
	as := func(id string, role auth.Role) context.Context {
		return auth.WithPrincipal(owner, auth.Principal{Person: auth.Person{ID: id, Role: role}, Via: auth.ViaSession})
	}
	sam, ada := as("sam", auth.RoleMember), as("ada", auth.RoleAdmin)

	ownerRun, _ := hub.Notify(owner, Request{Category: CategoryAutomation, Title: "Owner's report", Channels: []string{"desktop"}})
	samRun, _ := hub.Notify(auth.AsPerson(owner, auth.Person{ID: "sam"}), Request{Category: CategoryAutomation, Title: "Sam's report", Channels: []string{"desktop"}})
	_, _ = hub.Notify(owner, Request{Category: CategoryModel, Title: "Model ready", Channels: []string{"desktop"}})

	titles := func(ctx context.Context) []string {
		list, _, err := hub.List(ctx, false, 50)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, n := range list {
			out = append(out, n.Title)
		}
		return out
	}
	if got := titles(sam); len(got) != 1 || got[0] != "Sam's report" {
		t.Fatalf("sam sees %v", got)
	}
	if got := titles(ada); len(got) != 1 || got[0] != "Model ready" {
		t.Fatalf("ada sees %v", got)
	}
	if got := titles(owner); len(got) != 2 {
		t.Fatalf("the owner sees %v", got)
	}
	if len(desktop.got) != 2 || len(samRun.Deliveries) != 0 {
		t.Fatalf("delivered %d to the desktop, sam's deliveries %v", len(desktop.got), samRun.Deliveries)
	}

	if _, err := hub.GetVisible(sam, ownerRun.ID); err == nil {
		t.Fatal("sam read the owner's notice")
	}
	if err := hub.Dismiss(ada, ownerRun.ID); err == nil {
		t.Fatal("ada dismissed the owner's notice")
	}
	if err := hub.MarkRead(sam, nil); err != nil {
		t.Fatal(err)
	}
	if _, unread, _ := hub.List(owner, true, 50); unread != 2 {
		t.Fatalf("sam's mark-all-read reached others: %d unread", unread)
	}
}
