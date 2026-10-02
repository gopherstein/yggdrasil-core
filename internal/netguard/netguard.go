// Package netguard keeps reads of the public web off this computer and the
// local network. A page the assistant opens can ask it to open another
// address, so an address such as the daemon's own API, a router's admin
// page, or a cloud metadata service must be refused wherever it comes from:
// the address given, a redirect, or a name that resolves somewhere private.
//
// The check runs when a connection is made, against the addresses actually
// dialed, so a redirect or a name that changes its answer (DNS rebinding)
// cannot get past it.
package netguard

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// ErrPrivate is an address on this computer or the local network.
var ErrPrivate = errors.New("this address is on your own network or computer, which web reads cannot open")

// reserved are ranges that are not on the public internet but that netip
// does not name: carrier-grade NAT, "this network", IETF protocol
// assignments, benchmarking, and the reserved class E.
var reserved = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"),
}

// nat64 is the well-known NAT64 prefix; an address in it reaches the IPv4
// address in its last four bytes.
var nat64 = netip.MustParsePrefix("64:ff9b::/96")

// Private reports an address that is not on the public internet: loopback,
// private (RFC 1918, fc00::/7), link-local, carrier-grade NAT, unspecified,
// multicast, or reserved.
func Private(a netip.Addr) bool {
	a = a.Unmap()
	if !a.IsValid() || a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() ||
		a.IsInterfaceLocalMulticast() || a.IsMulticast() || a.IsUnspecified() {
		return true
	}
	if a.Is6() && nat64.Contains(a) {
		b := a.As16()
		return Private(netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]}))
	}
	for _, p := range reserved {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// Resolver looks a host's addresses up. *net.Resolver is one.
type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

// Guard dials only public addresses. The zero value uses the system's
// resolver and dialer.
type Guard struct {
	// Resolver looks hosts up; nil uses the system's.
	Resolver Resolver
	// Dial connects to an address that has already been checked, given as
	// ip:port. Nil uses a net.Dialer whose Control checks the address again
	// just before connecting. Tests replace it to reach a local server
	// standing in for a public one.
	Dial func(ctx context.Context, network, address string) (net.Conn, error)
}

// Lookup returns a host's addresses, or ErrPrivate when any of them is
// private. A host that is an address is checked as it is.
func (g *Guard) Lookup(ctx context.Context, host string) ([]netip.Addr, error) {
	host = strings.ToLower(strings.TrimSuffix(strings.Trim(host, "[]"), "."))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return nil, ErrPrivate
	}
	if a, err := netip.ParseAddr(host); err == nil {
		if Private(a) {
			return nil, ErrPrivate
		}
		return []netip.Addr{a}, nil
	}
	var r Resolver = net.DefaultResolver
	if g.Resolver != nil {
		r = g.Resolver
	}
	addrs, err := r.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, errors.New("the site's address could not be found")
	}
	if len(addrs) == 0 {
		return nil, errors.New("the site's address could not be found")
	}
	for _, a := range addrs {
		if Private(a) {
			return nil, ErrPrivate
		}
	}
	return addrs, nil
}

// CheckURL allows an http or https address whose host is public.
func (g *Guard) CheckURL(ctx context.Context, raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return errors.New("url must be http or https")
	}
	_, err = g.Lookup(ctx, u.Hostname())
	return err
}

// DialContext resolves address, refuses it when any of its addresses is
// private, and connects to one of the addresses it checked, so the name is
// not looked up a second time.
func (g *Guard) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addrs, err := g.Lookup(ctx, host)
	if err != nil {
		return nil, err
	}
	dial := g.Dial
	if dial == nil {
		dial = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second, Control: Control}).DialContext
	}
	var first error
	for _, a := range addrs {
		conn, err := dial(ctx, network, net.JoinHostPort(a.Unmap().String(), port))
		if err == nil {
			return conn, nil
		}
		if first == nil {
			first = err
		}
	}
	return nil, first
}

// Control is a net.Dialer Control that refuses a private address at the
// moment of connecting.
func Control(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return ErrPrivate
	}
	if Private(ap.Addr()) {
		return ErrPrivate
	}
	return nil
}

// Client returns an HTTP client that connects only to public addresses,
// including on every redirect. It does not use a proxy from the
// environment, since a proxy would make the request on its behalf where
// the check cannot see.
func (g *Guard) Client(timeout time.Duration) *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.Proxy = nil
	tr.DialContext = g.DialContext
	return &http.Client{Timeout: timeout, Transport: tr}
}
