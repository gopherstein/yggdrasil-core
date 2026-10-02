package connectors

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const standupICS = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:standup-1\r\nDTSTART:20261005T150000Z\r\nDTEND:20261005T160000Z\r\n" +
	"SUMMARY:Standup\r\nDESCRIPTION:Daily team check-in\r\nATTENDEE;CN=Sam:mailto:sam@example.org\r\n" +
	"BEGIN:VALARM\r\nACTION:DISPLAY\r\nDESCRIPTION:Reminder\r\nTRIGGER:-PT10M\r\nEND:VALARM\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"

const otherICS = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nUID:focus-1\r\nDTSTART:20261005T100000Z\r\nDTEND:20261005T110000Z\r\n" +
	"SUMMARY:Focus time\r\nTRANSP:TRANSPARENT\r\nEND:VEVENT\r\nBEGIN:VEVENT\r\nUID:trip-1\r\nDTSTART;VALUE=DATE:20261006\r\nDTEND;VALUE=DATE:20261007\r\n" +
	"SUMMARY:Trip to Sitka\r\nLOCATION:Sitka\\, AK\r\nEND:VEVENT\r\nBEGIN:VEVENT\r\nUID:yoga-1\r\nDTSTART;TZID=America/Juneau:20261005T070000\r\nDURATION:PT45M\r\n" +
	"SUMMARY:Yoga\r\nRRULE:FREQ=WEEKLY\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"

// fakeCalDAV is a server with a principal, a calendar home, an event
// calendar ("Work"), and a task list.
type fakeCalDAV struct {
	mu      sync.Mutex
	puts    map[string]string
	putHdr  map[string]string
	deletes []string
	delHdr  string
	reports []string
	stale   bool
}

func (f *fakeCalDAV) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, pass, _ := r.BasicAuth()
	if user != "me" || pass != "app-pass" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	defer f.mu.Unlock()
	ms := func(inner string) {
		w.WriteHeader(207)
		fmt.Fprintf(w, `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav">%s</d:multistatus>`, inner)
	}
	resp := func(href, props string) string {
		return `<d:response><d:href>` + href + `</d:href><d:propstat><d:prop>` + props + `</d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`
	}
	switch {
	case r.Method == "PROPFIND" && r.URL.Path == "/":
		ms(resp("/", `<d:resourcetype><d:collection/></d:resourcetype><d:current-user-principal><d:href>/principals/me/</d:href></d:current-user-principal>`))
	case r.Method == "PROPFIND" && r.URL.Path == "/principals/me/":
		ms(resp("/principals/me/", `<c:calendar-home-set><d:href>/calendars/me/</d:href></c:calendar-home-set>`))
	case r.Method == "PROPFIND" && r.URL.Path == "/calendars/me/":
		ms(resp("/calendars/me/", `<d:resourcetype><d:collection/></d:resourcetype>`) +
			resp("/calendars/me/work/", `<d:resourcetype><d:collection/><c:calendar/></d:resourcetype><d:displayname>Work</d:displayname><c:supported-calendar-component-set><c:comp name="VEVENT"/></c:supported-calendar-component-set>`) +
			resp("/calendars/me/tasks/", `<d:resourcetype><d:collection/><c:calendar/></d:resourcetype><d:displayname>Tasks</d:displayname><c:supported-calendar-component-set><c:comp name="VTODO"/></c:supported-calendar-component-set>`))
	case r.Method == "REPORT" && r.URL.Path == "/calendars/me/work/":
		f.reports = append(f.reports, string(body))
		if strings.Contains(string(body), "prop-filter") {
			if strings.Contains(string(body), ">yoga-1<") {
				ms(resp("/calendars/me/work/other.ics", `<d:getetag>"e2"</d:getetag><c:calendar-data>`+xmlText(otherICS)+`</c:calendar-data>`))
				return
			}
			ms(resp("/calendars/me/work/standup.ics", `<d:getetag>"e1"</d:getetag><c:calendar-data>`+xmlText(standupICS)+`</c:calendar-data>`))
			return
		}
		ms(resp("/calendars/me/work/standup.ics", `<d:getetag>"e1"</d:getetag><c:calendar-data>`+xmlText(standupICS)+`</c:calendar-data>`) +
			resp("/calendars/me/work/other.ics", `<d:getetag>"e2"</d:getetag><c:calendar-data>`+xmlText(otherICS)+`</c:calendar-data>`))
	case r.Method == http.MethodPut:
		if f.stale {
			w.WriteHeader(http.StatusPreconditionFailed)
			return
		}
		f.puts[r.URL.Path] = string(body)
		f.putHdr = map[string]string{"If-None-Match": r.Header.Get("If-None-Match"), "If-Match": r.Header.Get("If-Match"), "Content-Type": r.Header.Get("Content-Type")}
		w.WriteHeader(http.StatusCreated)
	case r.Method == http.MethodDelete:
		f.deletes = append(f.deletes, r.URL.Path)
		f.delHdr = r.Header.Get("If-Match")
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func xmlText(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(s)
}

func calendarSetup(t *testing.T) (*fakeCalDAV, Calendar, Credential, *http.Client) {
	t.Helper()
	old := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = old })
	f := &fakeCalDAV{puts: map[string]string{}}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	now := time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)
	return f, Calendar{Now: func() time.Time { return now }}, Credential{"url": srv.URL, "username": "me", "password": "app-pass"}, srv.Client()
}

func calTool(t *testing.T, k Calendar, id string, c *http.Client, cred Credential, args map[string]any) (map[string]any, error) {
	t.Helper()
	for _, tool := range k.Tools() {
		if tool.Def.ID == id {
			return tool.Run(context.Background(), c, cred, args)
		}
	}
	t.Fatalf("no tool %s", id)
	return nil, nil
}

func TestCalendarDiscoveryAndSearch(t *testing.T) {
	f, k, cred, c := calendarSetup(t)
	if account, err := k.Check(context.Background(), c, cred); err != nil || account != "me (Work)" {
		t.Fatalf("check %q %v", account, err)
	}
	bad := Credential{"url": cred["url"], "username": "me", "password": "wrong"}
	if _, err := k.Check(context.Background(), c, bad); err == nil || !strings.Contains(err.Error(), "refused the sign-in") {
		t.Fatalf("bad password: %v", err)
	}
	res, err := calTool(t, k, "calendar.search", c, cred, map[string]any{"from": "2026-10-05", "to": "2026-10-06"})
	if err != nil {
		t.Fatal(err)
	}
	evs := res["events"].([]map[string]any)
	var titles []string
	for _, e := range evs {
		titles = append(titles, e["title"].(string))
	}
	// Yoga is 07:00 in Juneau, 15:00 UTC.
	if strings.Join(titles, "|") != "Focus time|Standup|Yoga|Trip to Sitka" {
		t.Fatalf("events %q", titles)
	}
	standup := evs[1]
	if standup["start"] != "2026-10-05T15:00:00Z" || standup["description"] != "Daily team check-in" || standup["uid"] != "standup-1" {
		t.Fatalf("standup %v", standup)
	}
	trip := evs[3]
	if trip["all_day"] != true || trip["start"] != "2026-10-06" || trip["end"] != "2026-10-06" || trip["location"] != "Sitka, AK" {
		t.Fatalf("trip %v", trip)
	}
	if evs[2]["repeats"] != true || evs[2]["end"] != "2026-10-05T15:45:00Z" || evs[0]["free"] != true {
		t.Fatalf("yoga %v focus %v", evs[2], evs[0])
	}
	if !strings.Contains(f.reports[0], `<c:expand start="20261005T000000Z" end="20261007T000000Z"/>`) {
		t.Fatalf("report %s", f.reports[0])
	}
	res, _ = calTool(t, k, "calendar.search", c, cred, map[string]any{"from": "2026-10-05", "query": "sitka"})
	if n := len(res["events"].([]map[string]any)); n != 1 {
		t.Fatalf("query matched %d", n)
	}
	if _, err := calTool(t, k, "calendar.search", c, cred, map[string]any{"calendar": "Home"}); err == nil || !strings.Contains(err.Error(), "the calendars are Work") {
		t.Fatalf("unknown calendar: %v", err)
	}
}

// Free time is working hours minus busy events, ignoring events marked
// free and time already past.
func TestCalendarAvailability(t *testing.T) {
	_, k, cred, c := calendarSetup(t)
	res, err := calTool(t, k, "calendar.availability", c, cred, map[string]any{"from": "2026-10-05"})
	if err != nil {
		t.Fatal(err)
	}
	busy := res["busy"].([]map[string]string)
	free := res["free"].([]map[string]string)
	if len(busy) != 1 || busy[0]["start"] != "2026-10-05T15:00:00Z" {
		t.Fatalf("busy %v", busy)
	}
	if len(free) != 2 || free[0]["start"] != "2026-10-05T09:00:00Z" || free[0]["end"] != "2026-10-05T15:00:00Z" ||
		free[1]["start"] != "2026-10-05T16:00:00Z" || free[1]["end"] != "2026-10-05T17:00:00Z" {
		t.Fatalf("free %v", free)
	}
}

func TestCalendarCreateUpdateCancel(t *testing.T) {
	f, k, cred, c := calendarSetup(t)
	res, err := calTool(t, k, "calendar.create", c, cred, map[string]any{"title": "Lunch, with Sam", "start": "2026-10-07T10:00", "duration_minutes": float64(30), "location": "Coppa"})
	if err != nil {
		t.Fatal(err)
	}
	var path, body string
	for p, b := range f.puts {
		path, body = p, b
	}
	if !strings.HasPrefix(path, "/calendars/me/work/") || !strings.HasSuffix(path, ".ics") || f.putHdr["If-None-Match"] != "*" || !strings.HasPrefix(f.putHdr["Content-Type"], "text/calendar") {
		t.Fatalf("put %s %v", path, f.putHdr)
	}
	for _, want := range []string{"DTSTART:20261007T100000Z", "DTEND:20261007T103000Z", `SUMMARY:Lunch\, with Sam`, "LOCATION:Coppa", "\r\nEND:VCALENDAR\r\n"} {
		if !strings.Contains(body, want) {
			t.Errorf("event lacks %q:\n%s", want, body)
		}
	}
	if res["event"].(map[string]any)["title"] != "Lunch, with Sam" {
		t.Fatalf("created %v", res)
	}

	// Moving the start keeps the length, and attendees and alarms stay.
	f.puts = map[string]string{}
	res, err = calTool(t, k, "calendar.update", c, cred, map[string]any{"uid": "standup-1", "start": "2026-10-05T17:00"})
	if err != nil {
		t.Fatal(err)
	}
	body = f.puts["/calendars/me/work/standup.ics"]
	if f.putHdr["If-Match"] != `"e1"` || !strings.Contains(body, "DTSTART:20261005T170000Z") || !strings.Contains(body, "DTEND:20261005T180000Z") ||
		!strings.Contains(body, "ATTENDEE;CN=Sam:mailto:sam@example.org") || !strings.Contains(body, "DESCRIPTION:Reminder") || !strings.Contains(body, "DESCRIPTION:Daily team check-in") ||
		strings.Count(body, "DTSTART") != 1 {
		t.Fatalf("update %v:\n%s", f.putHdr, body)
	}
	if res["event"].(map[string]any)["end"] != "2026-10-05T18:00:00Z" {
		t.Fatalf("updated %v", res)
	}
	if _, err := calTool(t, k, "calendar.update", c, cred, map[string]any{"uid": "yoga-1", "title": "Pilates"}); err == nil || !strings.Contains(err.Error(), "repeating") {
		t.Fatalf("repeating: %v", err)
	}
	f.stale = true
	if _, err := calTool(t, k, "calendar.update", c, cred, map[string]any{"uid": "standup-1", "title": "Sync"}); err == nil || !strings.Contains(err.Error(), "changed in the calendar") {
		t.Fatalf("stale: %v", err)
	}
	f.stale = false

	res, err = calTool(t, k, "calendar.cancel", c, cred, map[string]any{"uid": "standup-1"})
	if err != nil || res["cancelled"] != true || len(f.deletes) != 1 || f.deletes[0] != "/calendars/me/work/standup.ics" || f.delHdr != `"e1"` {
		t.Fatalf("cancel %v %v %v %q", res, err, f.deletes, f.delHdr)
	}
	if _, err := calTool(t, k, "calendar.update", c, cred, map[string]any{"uid": "standup-1"}); err == nil {
		t.Error("an update with no changes was accepted")
	}
}

func TestICS(t *testing.T) {
	long := event{UID: "u", Summary: strings.Repeat("ø", 60), Start: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), AllDay: true}
	data := long.ics(time.Now())
	for _, line := range strings.Split(data, "\r\n") {
		if len(line) > 75 {
			t.Errorf("line of %d octets", len(line))
		}
	}
	got := parseICS(data)
	if len(got) != 1 || got[0].Summary != long.Summary || !got[0].AllDay || got[0].End.Format("2006-01-02") != "2026-01-02" {
		t.Fatalf("round trip %+v", got)
	}
	if d := icsDuration("P1DT2H30M"); d != 26*time.Hour+30*time.Minute {
		t.Errorf("duration %v", d)
	}
	if _, err := davBase(Credential{"url": "http://caldav.example.com"}); err == nil {
		t.Error("plain http to a remote server was accepted")
	}
}
