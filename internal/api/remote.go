package api

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yeixio/toskar-core/internal/auth"
)

// Access from anywhere (#456, docs/remote-access.md): a second listener for
// traffic from outside the home network. It serves the routes a phone uses
// and nothing else: only TLS, only phone keys, never this computer's or the
// local network's trust, no sign-in, no pairing, and no web pages.

type remoteKey struct{}

// remoteRequest reports a request that came in on the remote listener,
// with the device key it was let in with.
func remoteRequest(r *http.Request) bool { return remoteToken(r) != "" }

func remoteToken(r *http.Request) string {
	v, _ := r.Context().Value(remoteKey{}).(string)
	return v
}

// remoteFailures counts wrong keys on the remote listener by address: after
// twenty in ten minutes, that address waits.
var remoteFailures = auth.NewLimiter(20, 10*time.Minute)

// RemoteStatus is what the remote listener is doing.
type RemoteStatus struct {
	// Listening is the address it's listening on, or "".
	Listening string `json:"listening,omitempty"`
	// Error is why it isn't, such as a port another program has.
	Error string `json:"error,omitempty"`
}

type remoteListener struct {
	mu     sync.Mutex
	server *http.Server
	// relayed takes devices' connections from the relay tunnel (#456).
	relayed *mergedListener
	status  RemoteStatus
}

// remoteHandler marks every request as remote, keeps it to the API, and
// turns away an address that sent too many wrong keys.
func (s *Server) remoteHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			http.NotFound(w, r)
			return
		}
		ip := remoteIP(r)
		if remoteFailures.Blocked(ip) {
			writeErr(w, http.StatusTooManyRequests, "REMOTE_THROTTLED", "too many wrong keys from this address; try again later", nil)
			return
		}
		// Only a paired device's key gets any further, checked before
		// anything else reads the request, so every wrong one counts.
		token, err := auth.BearerToken(r)
		if err != nil || token == "" || s.deps.VerifyAPIKey == nil {
			remoteFailures.Fail(ip)
			writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "a paired device's key is required", nil)
			return
		}
		rec, err := s.deps.VerifyAPIKey(r.Context(), token)
		if err != nil {
			remoteFailures.Fail(ip)
			writeErr(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid api key", nil)
			return
		}
		if rec.Kind != auth.KindDevice {
			writeErr(w, http.StatusForbidden, "REMOTE_DEVICES_ONLY", "from outside the home network, only paired devices can connect", nil)
			return
		}
		ctx := context.WithValue(r.Context(), remoteKey{}, token)
		s.router.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ServeRemote starts the remote listener on addr, with the API's
// certificate, replacing one already running. It returns once it's
// listening, or with why it couldn't.
func (s *Server) ServeRemote(addr string) error {
	s.StopRemote()
	s.listenMu.Lock()
	cfg := s.tlsConfig
	s.listenMu.Unlock()
	if cfg == nil {
		err := errors.New("the API has no certificate")
		s.remote.set(RemoteStatus{Error: err.Error()})
		return err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		s.remote.set(RemoteStatus{Error: err.Error()})
		return err
	}
	srv := &http.Server{
		Handler:           s.remoteHandler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	merged := newMergedListener(ln)
	s.remote.mu.Lock()
	s.remote.server = srv
	s.remote.relayed = merged
	s.remote.status = RemoteStatus{Listening: ln.Addr().String()}
	s.remote.mu.Unlock()
	go func() {
		if err := srv.Serve(tls.NewListener(merged, cfg)); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.remote.set(RemoteStatus{Error: err.Error()})
		}
	}()
	return nil
}

// StopRemote closes the remote listener and its connections at once.
func (s *Server) StopRemote() {
	s.remote.mu.Lock()
	srv := s.remote.server
	s.remote.server, s.remote.relayed = nil, nil
	s.remote.status = RemoteStatus{}
	s.remote.mu.Unlock()
	if srv != nil {
		_ = srv.Close()
	}
}

// ServeRelayed answers a device's connection that came through the relay
// as the remote listener answers its own: TLS with the API's certificate,
// paired devices' keys only. It reports false when the listener is off.
func (s *Server) ServeRelayed(conn net.Conn) bool {
	s.remote.mu.Lock()
	merged := s.remote.relayed
	s.remote.mu.Unlock()
	return merged != nil && merged.push(conn)
}

// TLSCertificate is the API's certificate, which signs the address record
// devices find this computer by.
func (s *Server) TLSCertificate() (tls.Certificate, bool) {
	s.listenMu.Lock()
	defer s.listenMu.Unlock()
	if s.tlsConfig == nil || len(s.tlsConfig.Certificates) == 0 {
		return tls.Certificate{}, false
	}
	return s.tlsConfig.Certificates[0], true
}

// mergedListener accepts from a TCP listener and from connections pushed
// to it, so relayed devices get the same server as direct ones.
type mergedListener struct {
	net.Listener
	conns chan net.Conn
	errs  chan error
	done  chan struct{}
	once  sync.Once
}

func newMergedListener(ln net.Listener) *mergedListener {
	m := &mergedListener{Listener: ln, conns: make(chan net.Conn), errs: make(chan error, 1), done: make(chan struct{})}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				m.errs <- err
				return
			}
			if !m.push(c) {
				c.Close()
				return
			}
		}
	}()
	return m
}

func (m *mergedListener) push(c net.Conn) bool {
	select {
	case m.conns <- c:
		return true
	case <-m.done:
		return false
	}
}

func (m *mergedListener) Accept() (net.Conn, error) {
	select {
	case c := <-m.conns:
		return c, nil
	case err := <-m.errs:
		return nil, err
	case <-m.done:
		return nil, net.ErrClosed
	}
}

func (m *mergedListener) Close() error {
	m.once.Do(func() { close(m.done) })
	return m.Listener.Close()
}

// RemoteListening is what the remote listener is doing.
func (s *Server) RemoteListening() RemoteStatus {
	s.remote.mu.Lock()
	defer s.remote.mu.Unlock()
	return s.remote.status
}

func (l *remoteListener) set(st RemoteStatus) {
	l.mu.Lock()
	l.status = st
	l.mu.Unlock()
}

// handleRemoteAccess is access from anywhere's setting and what its listener
// is doing (#456), for Admins.
func (s *Server) handleRemoteAccess(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{"enabled": false}
	if s.deps.Config != nil {
		cfg := s.deps.Config.Get()
		out["enabled"] = cfg.RemoteAccess.Enabled
		out["port"] = cfg.RemotePort()
		if cfg.RemoteAccess.Address != "" {
			out["address"] = cfg.RemoteAccess.Address
		}
	}
	if s.deps.RemoteReach != nil {
		reach := s.deps.RemoteReach()
		out["port_mapping"] = reach.PortMapping
		if reach.Mapped != "" {
			out["mapped"], out["mapped_by"] = reach.Mapped, reach.Method
		}
		if reach.MapError != "" {
			out["map_error"] = reach.MapError
		}
		if len(reach.IPv6) > 0 {
			out["ipv6"] = reach.IPv6
		}
		out["reachable"] = reach.Reachable
		if reach.Reason != "" {
			out["reason"] = reach.Reason
		}
		if reach.Relay != "" {
			relay := map[string]any{"name": reach.Relay, "state": reach.RelayState}
			if reach.RelayError != "" {
				relay["error"] = reach.RelayError
			}
			out["relay"] = relay
		}
	}
	st := s.RemoteListening()
	if st.Listening != "" {
		out["listening"] = st.Listening
	}
	if st.Error != "" {
		out["error"] = st.Error
	}
	writeJSON(w, http.StatusOK, out)
}

// RemoteReach is how the internet reaches the remote listener (#456): the
// router port it opened, its IPv6 addresses, and in one word, how.
type RemoteReach struct {
	// PortMapping is whether Toskar asks the router to open a port.
	PortMapping bool
	// Mapped is the router's outside address and port, and Method how it
	// was opened: pcp, nat-pmp, or upnp.
	Mapped, Method string
	MapError       string
	IPv6           []string
	// Reachable is direct (a router port at a public address), ipv6,
	// manual (a port forwarded by hand), relay, or none, with Reason: off,
	// not_listening, carrier_nat, or no_port.
	Reachable, Reason string
	// Relay is the relay used, and RelayState how it's doing: no_token,
	// connecting, connected, or error, with RelayError its code.
	Relay, RelayState, RelayError string
}

// handleRouteSecret gives a paired device the route secret, on the home
// network only, never through the remote listener: devices paired before
// access from anywhere get it here the first time they connect at home.
func (s *Server) handleRouteSecret(w http.ResponseWriter, r *http.Request) {
	if remoteRequest(r) || !auth.FromLocalNetwork(r) {
		writeErr(w, http.StatusForbidden, "ROUTE_NOT_LOCAL", "the route secret is given only on the home network", nil)
		return
	}
	if s.deps.RouteSecret == nil {
		writeErr(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Access from anywhere isn't available.", nil)
		return
	}
	secret, route, err := s.deps.RouteSecret()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "ROUTE_FAILED", err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"route_secret": secret, "route_id": route})
}
