package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

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
