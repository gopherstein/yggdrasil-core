package netguard

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPrivate(t *testing.T) {
	for addr, want := range map[string]bool{
		"127.0.0.1":            true,
		"::1":                  true,
		"10.1.2.3":             true,
		"172.16.0.1":           true,
		"192.168.1.1":          true,
		"169.254.169.254":      true,
		"100.64.0.1":           true,
		"0.0.0.0":              true,
		"::":                   true,
		"224.0.0.1":            true,
		"ff02::1":              true,
		"fe80::1":              true,
		"fd00::1":              true,
		"::ffff:127.0.0.1":     true,
		"64:ff9b::a9fe:a9fe":   true,
		"8.8.8.8":              false,
		"203.0.113.10":         false,
		"2606:4700::1111":      false,
		"64:ff9b::808:808":     false,
		"::ffff:93.184.216.34": false,
	} {
		if got := Private(netip.MustParseAddr(addr)); got != want {
			t.Errorf("Private(%s) = %v, want %v", addr, got, want)
		}
	}
}

// fakeResolver answers from a map.
type fakeResolver map[string][]string

func (f fakeResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	answers, ok := f[host]
	if !ok {
		return nil, errors.New("no such host")
	}
	var addrs []netip.Addr
	for _, a := range answers {
		addrs = append(addrs, netip.MustParseAddr(a))
	}
	return addrs, nil
}

// standIn is a guard whose public.example resolves to a public address
// and whose dialer connects that address to server, standing in for a
// site on the internet.
func standIn(server *httptest.Server, resolver fakeResolver) (*Guard, *atomic.Int32) {
	var dials atomic.Int32
	target := server.Listener.Addr().String()
	return &Guard{
		Resolver: resolver,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dials.Add(1)
			if !strings.HasPrefix(address, "203.0.113.10:") {
				return nil, errors.New("unexpected dial to " + address)
			}
			var d net.Dialer
			return d.DialContext(ctx, network, target)
		},
	}, &dials
}

func TestLookupRefusesLocalNames(t *testing.T) {
	g := &Guard{Resolver: fakeResolver{"mixed.example": {"203.0.113.10", "192.168.1.1"}}}
	for _, host := range []string{"localhost", "api.localhost", "LOCALHOST.", "127.0.0.1", "[::1]", "169.254.169.254", "mixed.example"} {
		if _, err := g.Lookup(context.Background(), host); !errors.Is(err, ErrPrivate) {
			t.Errorf("Lookup(%q) = %v, want ErrPrivate", host, err)
		}
	}
}

func TestClientRefusesLoopbackServer(t *testing.T) {
	var hit atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hit.Store(true) }))
	defer server.Close()
	_, err := (&Guard{}).Client(5 * time.Second).Get(server.URL)
	if !errors.Is(err, ErrPrivate) {
		t.Fatalf("err = %v, want ErrPrivate", err)
	}
	if hit.Load() {
		t.Fatal("the local server was reached")
	}
}

func TestClientOpensPublicAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "hello") }))
	defer server.Close()
	g, _ := standIn(server, fakeResolver{"public.example": {"203.0.113.10"}})
	res, err := g.Client(5 * time.Second).Get("http://public.example/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if string(body) != "hello" {
		t.Fatalf("body = %q", body)
	}
}

func TestClientRefusesRedirectToLoopback(t *testing.T) {
	var hit atomic.Bool
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hit.Store(true) }))
	defer local.Close()
	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, local.URL+"/api/v1/secrets", http.StatusFound)
	}))
	defer public.Close()
	g, _ := standIn(public, fakeResolver{"public.example": {"203.0.113.10"}})
	_, err := g.Client(5 * time.Second).Get("http://public.example/")
	if !errors.Is(err, ErrPrivate) {
		t.Fatalf("err = %v, want ErrPrivate", err)
	}
	if hit.Load() {
		t.Fatal("the redirect reached the local server")
	}
}

func TestClientRefusesNameWithPrivateAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	g, dials := standIn(server, fakeResolver{"rebind.example": {"203.0.113.10", "127.0.0.1"}})
	_, err := g.Client(5 * time.Second).Get("http://rebind.example/")
	if !errors.Is(err, ErrPrivate) {
		t.Fatalf("err = %v, want ErrPrivate", err)
	}
	if dials.Load() != 0 {
		t.Fatal("dialed despite a private answer")
	}
}

func TestControl(t *testing.T) {
	if err := Control("tcp", "127.0.0.1:7331", nil); !errors.Is(err, ErrPrivate) {
		t.Fatalf("loopback: %v", err)
	}
	if err := Control("tcp", "[fe80::1]:80", nil); !errors.Is(err, ErrPrivate) {
		t.Fatalf("link-local: %v", err)
	}
	if err := Control("tcp", "203.0.113.10:443", nil); err != nil {
		t.Fatalf("public: %v", err)
	}
}

func TestCheckURL(t *testing.T) {
	g := &Guard{Resolver: fakeResolver{"public.example": {"203.0.113.10"}}}
	if err := g.CheckURL(context.Background(), "https://public.example/page"); err != nil {
		t.Fatal(err)
	}
	if err := g.CheckURL(context.Background(), "http://127.0.0.1:7331/api/v1/health"); !errors.Is(err, ErrPrivate) {
		t.Fatalf("loopback: %v", err)
	}
	if err := g.CheckURL(context.Background(), "file:///etc/passwd"); err == nil || errors.Is(err, ErrPrivate) {
		t.Fatalf("file: %v", err)
	}
}
