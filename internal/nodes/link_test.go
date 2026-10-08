package nodes

import (
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/mixtls"
)

func identity(t *testing.T, id string) *auth.NodeIdentity {
	t.Helper()
	ident, err := auth.LoadOrCreateIdentity(auth.NewSecretStore(t.TempDir()), id)
	if err != nil {
		t.Fatal(err)
	}
	return ident
}

// serveMixed serves h on a port that speaks TLS with ident's certificate
// and plain HTTP, as Bifrost does (#175).
func serveMixed(t *testing.T, ident *auth.NodeIdentity, h http.Handler) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := ident.ServerTLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = srv.Serve(mixtls.NewListener(ln, cfg)) }()
	t.Cleanup(func() { _ = srv.Close() })
	return "http://" + ln.Addr().String()
}

// how answers whether the request came over TLS.
var how = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	if r.TLS != nil {
		_, _ = io.WriteString(w, "tls "+string(body))
		return
	}
	_, _ = io.WriteString(w, "plain "+string(body))
})

func get(t *testing.T, c *http.Client, url string) (string, error) {
	t.Helper()
	resp, err := c.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b), nil
}

func TestBifrostSpeaksTLSAndPlainOnOnePort(t *testing.T) {
	peer := identity(t, "peer")
	base := serveMixed(t, peer, how)

	// A computer that hasn't updated still gets an answer.
	if got, err := get(t, http.DefaultClient, base+"/x"); err != nil || got != "plain " {
		t.Fatalf("plain: %q %v", got, err)
	}

	// An updated one speaks TLS, checked against the key it paired with.
	var marked atomic.Int32
	c := LinkClient("peer-a", Link{Pin: peer.Fingerprint(), MarkTLS: func() { marked.Add(1) }}, 5*time.Second)
	if got, err := get(t, c, base+"/x"); err != nil || got != "tls " {
		t.Fatalf("tls: %q %v", got, err)
	}
	if _, _ = get(t, c, base+"/x"); marked.Load() != 1 || Transport("peer-a") != "tls" {
		t.Fatalf("marked %d times, transport %q", marked.Load(), Transport("peer-a"))
	}

	// Another computer at its address, with another key, is refused, and
	// not retried in plain HTTP.
	other := identity(t, "other")
	c = LinkClient("peer-b", Link{Pin: other.Fingerprint()}, 5*time.Second)
	if _, err := get(t, c, base+"/x"); err == nil || !errors.Is(err, auth.ErrPinMismatch) {
		t.Fatalf("wrong key: %v", err)
	}
}

func TestOlderComputerIsReachedPlainly(t *testing.T) {
	old := httptest.NewServer(how)
	defer old.Close()

	// Never spoke TLS: plain HTTP, with the body sent again.
	c := LinkClient("old-a", Link{Pin: "sha256:00"}, 5*time.Second)
	resp, err := c.Post(old.URL+"/x", "text/plain", strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(b) != "plain hello" || Transport("old-a") != "plain" {
		t.Fatalf("got %q, transport %q", b, Transport("old-a"))
	}

	// Spoke TLS before: an answer without it is refused.
	c = LinkClient("old-b", Link{Pin: "sha256:00", TLSSeen: true}, 5*time.Second)
	if _, err := get(t, c, old.URL+"/x"); !errors.Is(err, ErrPlainAfterTLS) {
		t.Fatalf("downgrade: %v", err)
	}
}

// The internal server and client speak TLS end to end: a paired computer's
// health, checked by its key.
func TestInternalServerOverTLS(t *testing.T) {
	peer := identity(t, "peer")
	srv := NewInternalServer(InternalDeps{Identity: peer, Logger: discardLogger()})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	go func() { _ = srv.ListenAndServe(addr) }()
	t.Cleanup(func() { _ = srv.Shutdown(t.Context()) })
	deadline := time.Now().Add(5 * time.Second)
	for {
		if c, err := net.Dial("tcp", addr); err == nil {
			_ = c.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("server didn't start")
		}
		time.Sleep(20 * time.Millisecond)
	}
	c := NewClient("http://"+addr, nil, "peer-health").Secure(Link{Pin: peer.Fingerprint()})
	if err := c.Health(t.Context()); err != nil || Transport("peer-health") != "tls" {
		t.Fatalf("health: %v, transport %q", err, Transport("peer-health"))
	}
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
