package gjallarhorn

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDigestNext(t *testing.T) {
	g := Digest{At: "18:00", TimeZone: "America/Juneau"} // UTC-8 in October
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC) // 04:00 in Juneau
	if got := g.Next(now); !got.Equal(time.Date(2026, 10, 3, 2, 0, 0, 0, time.UTC)) {
		t.Fatalf("next = %v", got)
	}
	if got := g.Next(time.Date(2026, 10, 3, 2, 0, 0, 0, time.UTC)); !got.Equal(time.Date(2026, 10, 4, 2, 0, 0, 0, time.UTC)) {
		t.Fatalf("at the time itself, next = %v", got)
	}
	for _, bad := range []Digest{{At: "25:00", TimeZone: "UTC"}, {At: "08:00", TimeZone: "Mars/Base"}} {
		if bad.Validate() == nil {
			t.Errorf("%+v validated", bad)
		}
	}
}

func TestDigestGathersNotices(t *testing.T) {
	var mu sync.Mutex
	var posts []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		mu.Lock()
		posts = append(posts, body)
		mu.Unlock()
	}))
	defer srv.Close()
	hub, eg, clock := deliveryHub(t)
	ctx := context.Background()
	d, _, err := hub.CreateDestination(ctx, DestinationInput{Kind: KindWebhook, Name: ptr("Hook"), Webhook: &WebhookConfig{URL: srv.URL},
		Digest: &Digest{At: "18:00"}})
	if err != nil {
		t.Fatal(err)
	}
	if d.Digest == nil || d.Digest.TimeZone != "UTC" {
		t.Fatalf("digest = %+v", d.Digest)
	}

	a, _ := hub.Notify(ctx, Request{Title: "Report ready", Body: "The weekly report is done.\nMore lines.", Severity: SeverityInfo})
	b, _ := hub.Notify(ctx, Request{Title: "Model installed", Severity: SeverityWarning})
	failed, _ := hub.Notify(ctx, Request{Title: "Automation failed", Severity: SeverityError})
	hub.DeliverDue(ctx)
	if dl := deliveryOf(t, hub, a.ID); dl.Status != DeliveryDigest || dl.NextAttemptAt == nil || dl.NextAttemptAt.Hour() != 18 {
		t.Fatalf("info notice: %+v", dl)
	}
	// Errors don't wait.
	if dl := deliveryOf(t, hub, failed.ID); dl.Status != DeliveryDelivered {
		t.Fatalf("error notice: %+v", dl)
	}
	if len(posts) != 1 {
		t.Fatalf("before the digest, %d posts", len(posts))
	}

	*clock = time.Date(2026, 10, 2, 18, 0, 30, 0, time.UTC)
	hub.DeliverDue(ctx)
	if len(posts) != 2 {
		t.Fatalf("after the digest time, %d posts", len(posts))
	}
	raw, _ := json.Marshal(posts[1])
	digest := string(raw)
	for _, want := range []string{"2 notifications", "Report ready: The weekly report is done.", "Model installed"} {
		if !strings.Contains(digest, want) {
			t.Fatalf("digest lacks %q: %s", want, digest)
		}
	}
	if strings.Contains(digest, "More lines") || strings.Contains(digest, "Automation failed") {
		t.Fatalf("digest has what it shouldn't: %s", digest)
	}
	for _, id := range []string{a.ID, b.ID} {
		if dl := deliveryOf(t, hub, id); dl.Status != DeliveryDelivered {
			t.Fatalf("%s after the digest: %+v", id, dl)
		}
	}
	// One message left this computer for the digest.
	n := 0
	for _, add := range eg.adds {
		if strings.Contains(add, "2 notifications") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("egress: %v", eg.adds)
	}
	// Nothing waiting: nothing sent the next day.
	*clock = clock.Add(24 * time.Hour)
	hub.DeliverDue(ctx)
	if len(posts) != 2 {
		t.Fatalf("empty digest sent: %d posts", len(posts))
	}

	// Turning the digest off sends each notice again.
	if _, err := hub.UpdateDestination(ctx, d.ID, DestinationInput{Digest: &Digest{}}); err != nil {
		t.Fatal(err)
	}
	c, _ := hub.Notify(ctx, Request{Title: "Back to each notice", Severity: SeverityInfo})
	hub.DeliverDue(ctx)
	if dl := deliveryOf(t, hub, c.ID); dl.Status != DeliveryDelivered {
		t.Fatalf("after turning the digest off: %+v", dl)
	}
}

func TestDigestWaitsForQuietHours(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer srv.Close()
	hub, _, clock := deliveryHub(t)
	ctx := context.Background()
	if _, err := hub.SetQuietHours(ctx, QuietHours{Enabled: true, Start: "22:00", End: "07:00", TimeZone: "UTC", Allow: "errors"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := hub.CreateDestination(ctx, DestinationInput{Kind: KindWebhook, Name: ptr("Hook"), Webhook: &WebhookConfig{URL: srv.URL},
		Digest: &Digest{At: "23:00", TimeZone: "UTC"}}); err != nil {
		t.Fatal(err)
	}
	n, _ := hub.Notify(ctx, Request{Title: "Report ready", Severity: SeverityInfo})
	*clock = time.Date(2026, 10, 2, 23, 0, 30, 0, time.UTC)
	hub.DeliverDue(ctx)
	if calls != 0 {
		t.Fatal("a digest went out in quiet hours")
	}
	if dl := deliveryOf(t, hub, n.ID); dl.NextAttemptAt == nil || dl.NextAttemptAt.Hour() != 7 {
		t.Fatalf("held digest: %+v", dl)
	}
	*clock = time.Date(2026, 10, 3, 7, 0, 30, 0, time.UTC)
	hub.DeliverDue(ctx)
	if calls != 1 {
		t.Fatalf("after quiet hours, %d calls", calls)
	}
}
