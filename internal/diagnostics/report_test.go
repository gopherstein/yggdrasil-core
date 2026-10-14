package diagnostics

import (
	"strings"
	"testing"
)

func TestScrub(t *testing.T) {
	cases := []struct{ in, want string }{
		{"sent to mike@example.com ok", "sent to <email> ok"},
		{"key ygg_abc123DEF rejected", "key <key> rejected"},
		{"Authorization: Bearer abc.def.ghi", "Authorization: Bearer <hidden>"},
		{`password="hunter2"`, `password="<hidden>"`},
		{"token aGVsbG8gd29ybGQgdGhpcyBpcyBhIHNlY3JldCB0b2tlbg9 here", "token <hidden> here"},
		{"/Users/mike/Library/Application Support/Toskar", "/Users/<you>/Library/Application Support/Toskar"},
		{`C:\Users\Mike\AppData`, `C:\Users\<you>\AppData`},
		{"bound on Mikes-MacBook.local and MIKES-MACBOOK", "bound on <computer>.local and <computer>"},
		{"router 192.168.1.1, outside 8.8.8.8, cgnat 100.72.1.1", "router 192.168.1.1, outside <public IPv4>, cgnat <carrier-grade NAT address>"},
		{"ipv6 2001:db8::1 and fe80::1 and ::1", "ipv6 <public IPv6> and fe80::1 and ::1"},
		{"API key invalid, token expired", "API key invalid, token expired"},
		// Kept: times, model IDs, and UUIDs that match a report to a log.
		{"at 18:16:04 loaded qwen2.5-7b-q4", "at 18:16:04 loaded qwen2.5-7b-q4"},
		{"instance 211c1b31-3d9a-427a-bbe9-26a2f4a61135", "instance 211c1b31-3d9a-427a-bbe9-26a2f4a61135"},
	}
	for _, c := range cases {
		if got := Scrub(c.in, "Mikes-MacBook.local"); got != c.want {
			t.Errorf("Scrub(%q)\n got %q\nwant %q", c.in, got, c.want)
		}
	}
}

func TestReportLog(t *testing.T) {
	log := strings.Join([]string{
		`{"time":"t1","level":"INFO","msg":"api listening","addr":"[::]:7331"}`,
		`{"time":"t2","level":"WARN","msg":"model load slow","model":"qwen","prompt":"my secret plans"}`,
		`not json`,
		`{"time":"t3","level":"ERROR","msg":"chat failed","error":"open /Users/mike/x: denied","status":500}`,
		`{"time":"t4","level":"ERROR","msg":"third"}`,
	}, "\n")
	got := ReportLog(log, "", 2)
	want := []string{
		`t3 ERROR chat failed error="open /Users/<you>/x: denied" status=500`,
		`t4 ERROR third`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got %q", got)
	}
	all := strings.Join(ReportLog(log, "", 40), "\n")
	if strings.Contains(all, "secret plans") || strings.Contains(all, "api listening") {
		t.Fatalf("kept what it shouldn't: %q", all)
	}
}
