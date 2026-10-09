package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/config"
	"github.com/yeixio/toskar-core/internal/portmap"
	"github.com/yeixio/toskar-core/internal/relayclient"
	"github.com/yeixio/toskar-core/pkg/contracts"
)

// A forwarded address is a host or IP, with a port when it differs; the
// port is the API's own neither (#456).
func TestRemoteAccessSettings(t *testing.T) {
	for in, ok := range map[string]bool{
		"home.example.com":       true,
		"203.0.113.7:7333":       true,
		"[2001:db8::1]:7333":     true,
		"2001:db8::1":            true,
		"":                       true,
		"https://home.example":   false,
		"home.example.com/path":  false,
		"home.example.com:99999": false,
		"bad_host.example":       false,
		"user@home.example":      false,
	} {
		if _, err := cleanRemoteAddress(in); (err == nil) != ok {
			t.Errorf("%q: %v", in, err)
		}
	}

	t.Setenv("TOSKAR_DISCOVERY_ENABLED", "false")
	a, err := New(Options{DataDir: t.TempDir(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.DB.Close() })
	ctx := context.Background()
	if err := a.applySettingsPatch(ctx, map[string]any{"remote_access_port": float64(7331)}); !errors.Is(err, errRemotePort) {
		t.Fatalf("the API's port: %v", err)
	}
	if err := a.applySettingsPatch(ctx, map[string]any{"remote_access_address": "https://x"}); !errors.Is(err, errRemoteAddress) {
		t.Fatalf("a URL: %v", err)
	}
	if code, _ := contracts.ErrorCode(errRemotePort); code != "REMOTE_PORT_INVALID" {
		t.Fatalf("code: %q", code)
	}
	if err := a.applySettingsPatch(ctx, map[string]any{"remote_access_enabled": true, "remote_access_port": float64(17333), "remote_access_address": " home.example.com "}); err != nil {
		t.Fatal(err)
	}
	view, err := a.settingsView(ctx)
	if err != nil || !view.RemoteAccessEnabled || view.RemoteAccessPort != 17333 || view.RemoteAccessAddress != "home.example.com" {
		t.Fatalf("view: %+v %v", view, err)
	}
	if err := a.applySettingsPatch(ctx, map[string]any{"remote_access_port": float64(0)}); err != nil {
		t.Fatal(err)
	}
	if view, _ := a.settingsView(ctx); view.RemoteAccessPort != 7333 {
		t.Fatalf("default port: %d", view.RemoteAccessPort)
	}
}

// How the internet reaches the listener, in a word (#456).
func TestReachOf(t *testing.T) {
	on := config.Config{RemoteAccess: config.RemoteAccess{Enabled: true}}
	upnp := func(addr string) portmap.Mapping {
		return portmap.Mapping{Method: portmap.MethodUPnP, External: netip.MustParseAddrPort(addr)}
	}
	manual := on
	manual.RemoteAccess.Address = "home.example.com"
	for name, c := range map[string]struct {
		cfg       config.Config
		listening bool
		m         portmap.Mapping
		ipv6      bool
		relayed   bool
		reach     string
		reason    string
	}{
		"off":                {config.Config{}, false, portmap.Mapping{}, false, false, "none", "off"},
		"not listening":      {on, false, portmap.Mapping{}, false, true, "none", "not_listening"},
		"manual":             {manual, true, portmap.Mapping{}, false, true, "manual", ""},
		"direct":             {on, true, upnp("203.0.113.9:7333"), false, true, "direct", ""},
		"ipv6":               {on, true, portmap.Mapping{}, true, true, "ipv6", ""},
		"relay":              {on, true, portmap.Mapping{}, false, true, "relay", ""},
		"relay, carrier NAT": {on, true, upnp("100.70.1.2:7333"), false, true, "relay", ""},
		"carrier NAT":        {on, true, upnp("100.70.1.2:7333"), false, false, "none", "carrier_nat"},
		"no port":            {on, true, portmap.Mapping{}, false, false, "none", "no_port"},
	} {
		if reach, reason := reachOf(c.cfg, c.listening, c.m, c.ipv6, c.relayed); reach != c.reach || reason != c.reason {
			t.Errorf("%s: %s %s", name, reach, reason)
		}
	}
	if !mapsPort(on) || mapsPort(manual) {
		t.Fatal("maps a port")
	}
	noMap := on
	noMap.RemoteAccess.NoPortMapping = true
	if mapsPort(noMap) {
		t.Fatal("mapped with mapping off")
	}
}

// The relay's name and enrollment secret: checked, the secret kept with
// the secrets and never shown, and a change clearing the old token (#456).
func TestRelaySettings(t *testing.T) {
	for in, want := range map[string]string{
		"":                           "",
		" Relay.Example.com ":        "relay.example.com",
		"https://relay.example.com/": "relay.example.com",
		"relay.localhost:7472":       "relay.localhost:7472",
	} {
		if got, err := cleanRelayName(in); err != nil || got != want {
			t.Errorf("%q: %q %v", in, got, err)
		}
	}
	for _, bad := range []string{"relay", "203.0.113.7", "relay.example.com/x", "relay.example.com:0", "user@relay.example.com", "bad_name.example"} {
		if _, err := cleanRelayName(bad); !errors.Is(err, errRelayName) {
			t.Errorf("%q: %v", bad, err)
		}
	}

	t.Setenv("TOSKAR_DISCOVERY_ENABLED", "false")
	t.Setenv("TOSKAR_RELAY_URL", "")
	dir := t.TempDir()
	a, err := New(Options{DataDir: dir, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.DB.Close() })
	ctx := context.Background()
	secrets := auth.NewSecretStore(dir)
	if got := relayName(a.Config.Get()); got != relayclient.DefaultRelay {
		t.Fatalf("default relay: %s", got)
	}
	if err := a.applySettingsPatch(ctx, map[string]any{"remote_access_relay_secret": "short"}); !errors.Is(err, errRelaySecret) {
		t.Fatalf("short secret: %v", err)
	}
	if err := a.applySettingsPatch(ctx, map[string]any{"remote_access_relay": "nope"}); !errors.Is(err, errRelayName) {
		t.Fatalf("bad name: %v", err)
	}
	_ = secrets.Write(relayclient.TokenName, "rt1.old")
	secret := "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG"
	if err := a.applySettingsPatch(ctx, map[string]any{"remote_access_relay": "relay.example.com", "remote_access_relay_secret": " " + secret + "\n"}); err != nil {
		t.Fatal(err)
	}
	if kept, _ := secrets.Read(relaySecretName); kept != secret {
		t.Fatalf("kept secret: %q", kept)
	}
	if got := enrollSecret(a.Config.Get(), secrets); got != secret {
		t.Fatalf("enroll secret for the organization's relay: %q", got)
	}
	if _, err := secrets.Read(relayclient.TokenName); err == nil {
		t.Fatal("the old relay's token survived")
	}
	view, err := a.settingsView(ctx)
	if err != nil || view.RemoteAccessRelay != "relay.example.com" || !view.RemoteAccessRelayEnrolled {
		t.Fatalf("view: %+v %v", view, err)
	}
	if got := relayName(a.Config.Get()); got != "relay.example.com" {
		t.Fatalf("relay: %s", got)
	}
	if err := a.applySettingsPatch(ctx, map[string]any{"remote_access_relay": "", "remote_access_relay_secret": ""}); err != nil {
		t.Fatal(err)
	}
	if view, _ := a.settingsView(ctx); view.RemoteAccessRelay != "" || view.RemoteAccessRelayEnrolled {
		t.Fatalf("cleared: %+v", view)
	}
	// Toskar's relay never gets an enrollment secret, even one left set.
	_ = secrets.Write(relaySecretName, secret)
	if got := enrollSecret(a.Config.Get(), secrets); got != "" {
		t.Fatalf("enroll secret sent to Toskar's relay: %q", got)
	}
	t.Setenv("TOSKAR_RELAY_URL", "https://relay.localhost:7472/")
	if got := relayName(a.Config.Get()); got != "relay.localhost:7472" {
		t.Fatalf("from the environment: %s", got)
	}
}
