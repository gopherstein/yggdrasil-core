// Package browser lets the assistant use web pages in an isolated browser
// (Gungnir §26): open a page, read it, click, type, download, and take a
// screenshot. Each chat gets its own headless browser with a fresh profile,
// never the person's own, and every request is checked so a page cannot
// reach this computer or the local network.
package browser

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ErrPrivate is a request for this computer or the local network.
var ErrPrivate = errors.New("this address is on your own network or computer, which the browser does not open")

// Guard decides which hosts the browser may reach.
type Guard struct {
	// Allow, when set, lets a host through regardless; tests use it for
	// their local servers.
	Allow func(host string) bool
	// Resolver looks hosts up; nil uses the system's.
	Resolver *net.Resolver

	mu    sync.Mutex
	cache map[string]guardEntry
}

type guardEntry struct {
	err error
	at  time.Time
}

// private reports an address that is not on the public internet.
func private(a netip.Addr) bool {
	a = a.Unmap()
	if a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() ||
		a.IsInterfaceLocalMulticast() || a.IsMulticast() || a.IsUnspecified() {
		return true
	}
	// Carrier-grade NAT and other shared or reserved ranges.
	for _, p := range []netip.Prefix{netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("0.0.0.0/8"),
		netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("240.0.0.0/4")} {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// CheckURL allows an http or https address on the public internet.
func (g *Guard) CheckURL(ctx context.Context, raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return errors.New("the address must start with http:// or https://")
	}
	return g.CheckHost(ctx, u.Hostname())
}

// CheckHost allows a host whose every address is public. Answers are kept
// for a minute.
func (g *Guard) CheckHost(ctx context.Context, host string) error {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if g.Allow != nil && g.Allow(host) {
		return nil
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return ErrPrivate
	}
	g.mu.Lock()
	if e, ok := g.cache[host]; ok && time.Since(e.at) < time.Minute {
		g.mu.Unlock()
		return e.err
	}
	g.mu.Unlock()
	err := g.lookup(ctx, host)
	g.mu.Lock()
	if g.cache == nil {
		g.cache = map[string]guardEntry{}
	}
	g.cache[host] = guardEntry{err: err, at: time.Now()}
	g.mu.Unlock()
	return err
}

func (g *Guard) lookup(ctx context.Context, host string) error {
	if a, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		if private(a) {
			return ErrPrivate
		}
		return nil
	}
	r := g.Resolver
	if r == nil {
		r = net.DefaultResolver
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	addrs, err := r.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return errors.New("the site's address could not be found")
	}
	for _, a := range addrs {
		if private(a) {
			return ErrPrivate
		}
	}
	return nil
}
