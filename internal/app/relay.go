package app

import (
	"context"
	"crypto/x509"
	"errors"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/config"
	"github.com/yeixio/toskar-core/internal/portmap"
	"github.com/yeixio/toskar-core/internal/relayclient"
	"github.com/yeixio/toskar-core/internal/rendezvous"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

// The relay (#456, docs/remote-access.md): when no direct path reaches this
// computer, paired devices come through a relay, Toskar's or the
// organization's own, over a tunnel this computer keeps open.

// relaySecretName is the enrollment secret's name in the secret store.
const relaySecretName = "relay-enroll-secret"

var (
	errRelayName   = contracts.NewError("RELAY_INVALID", nil, errors.New("the relay is a host name, with a port when it isn't 443, such as relay.example.com"))
	errRelaySecret = contracts.NewError("RELAY_SECRET_INVALID", nil, errors.New("that isn't an enrollment secret; copy it from the relay's enroll.secret file"))
)

// relayName is the relay this computer uses: its setting, then
// TOSKAR_RELAY_URL (for a developer's relay), then Toskar's.
func relayName(cfg config.Config) string {
	if cfg.RemoteAccess.Relay != "" {
		return cfg.RemoteAccess.Relay
	}
	if env := strings.TrimSpace(os.Getenv("TOSKAR_RELAY_URL")); env != "" {
		env = strings.TrimPrefix(strings.TrimPrefix(env, "https://"), "http://")
		return strings.TrimSuffix(env, "/")
	}
	return relayclient.DefaultRelay
}

// cleanRelayName checks a relay's name; "" is Toskar's.
func cleanRelayName(in string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(in))
	name = strings.TrimSuffix(strings.TrimPrefix(name, "https://"), "/")
	if name == "" {
		return "", nil
	}
	if strings.ContainsAny(name, " /?#@[]") || len(name) > 260 {
		return "", errRelayName
	}
	host := name
	if h, port, err := net.SplitHostPort(name); err == nil {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", errRelayName
		}
		host = h
	}
	if net.ParseIP(host) != nil || !validHostName(host) || !strings.Contains(host, ".") {
		return "", errRelayName
	}
	return name, nil
}

// cleanRelaySecret checks an enrollment secret as pasted.
func cleanRelaySecret(in string) (string, error) {
	secret := strings.TrimSpace(in)
	if len(secret) < 22 || len(secret) > 200 || strings.ContainsAny(secret, " \t\r\n") {
		return "", errRelaySecret
	}
	return secret, nil
}

// applyRelay starts the relay client while access from anywhere is on and
// its listener is up, or stops it. A change of relay or secret restarts it.
func (a *App) applyRelay() {
	a.relayMu.Lock()
	defer a.relayMu.Unlock()
	if a.relayStop != nil {
		a.relayStop()
		a.relayStop, a.relay = nil, nil
	}
	if a.API == nil || a.Config == nil {
		return
	}
	cfg := a.Config.Get()
	if !cfg.RemoteAccess.Enabled || a.API.RemoteListening().Listening == "" {
		return
	}
	secrets := auth.NewSecretStore(cfg.DataDir)
	routeSecret, err := rendezvous.RouteSecret(secrets)
	if err != nil {
		a.Logger.Warn("relay off: no route secret", "error", err)
		return
	}
	c := &relayclient.Client{
		Relay:        relayName(cfg),
		Secret:       routeSecret,
		EnrollSecret: enrollSecret(cfg, secrets),
		Store:        secrets,
		Certificate:  a.API.TLSCertificate,
		Addresses:    a.directAddresses,
		Deliver:      a.API.ServeRelayed,
		RootCAs:      relayRoots(a),
		Log:          a.Logger,
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.relay, a.relayStop = c, cancel
	go c.Run(ctx)
}

// enrollSecret is the enrollment secret to use: only with an
// organization's own relay. Toskar's relay never gets it; its tokens come
// with a subscription.
func enrollSecret(cfg config.Config, secrets *auth.SecretStore) string {
	if relayName(cfg) == relayclient.DefaultRelay {
		return ""
	}
	secret, _ := secrets.Read(relaySecretName)
	return strings.TrimSpace(secret)
}

// relayEnrolled reports an enrollment secret kept for the relay.
func (a *App) relayEnrolled(cfg config.Config) bool {
	secret, err := auth.NewSecretStore(cfg.DataDir).Read(relaySecretName)
	return err == nil && strings.TrimSpace(secret) != ""
}

// stopRelay stops the relay client.
func (a *App) stopRelay() {
	a.relayMu.Lock()
	defer a.relayMu.Unlock()
	if a.relayStop != nil {
		a.relayStop()
		a.relayStop, a.relay = nil, nil
	}
}

// relayStatus is the relay client's state, or nil when it isn't running.
func (a *App) relayStatus() (string, *relayclient.Status) {
	a.relayMu.Lock()
	defer a.relayMu.Unlock()
	if a.relay == nil {
		return "", nil
	}
	st := a.relay.Status()
	return a.relay.Relay, &st
}

// relayRoots trusts the system's certificates, plus TOSKAR_RELAY_CA (a PEM
// file) for a relay whose certificate an organization's own CA issued.
func relayRoots(a *App) *x509.CertPool {
	path := strings.TrimSpace(os.Getenv("TOSKAR_RELAY_CA"))
	if path == "" {
		return nil
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	pem, err := os.ReadFile(path)
	if err != nil || !pool.AppendCertsFromPEM(pem) {
		a.Logger.Warn("TOSKAR_RELAY_CA not used", "path", path, "error", err)
		return nil
	}
	return pool
}

// directAddresses are the ways in that skip the relay, best first: a
// forwarded address, the router's port, then IPv6.
func (a *App) directAddresses() []string {
	cfg := a.Config.Get()
	port := strconv.Itoa(cfg.RemotePort())
	var out []string
	if addr := cfg.RemoteAccess.Address; addr != "" {
		if _, _, err := net.SplitHostPort(addr); err != nil {
			addr = net.JoinHostPort(strings.Trim(addr, "[]"), port)
		}
		out = append(out, addr)
	}
	if m, _ := a.portKeeper.Status(); m.Method != "" && portmap.Public(m.External.Addr()) {
		out = append(out, m.External.String())
	}
	for _, ip := range portmap.PublicIPv6() {
		out = append(out, net.JoinHostPort(ip.String(), port))
	}
	return out
}
