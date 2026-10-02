package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func fakeDaemon(t *testing.T, routes map[string]func(w http.ResponseWriter, r *http.Request)) daemonClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, ok := routes[r.Method+" "+r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return daemonClient{base: srv.URL, client: srv.Client()}
}

func reply(status int, v any) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(v)
	}
}

func apiErr(code, msg string) map[string]any {
	return map[string]any{"error": map[string]any{"code": code, "message": msg}}
}

var joinArgs = []string{"--server", "10.0.0.5:7332", "--token", "ygj_aaaaaaaa_" + strings.Repeat("b", 32), "--fingerprint", "sha256:" + strings.Repeat("c", 64)}

func TestJoinCommand(t *testing.T) {
	var got map[string]string
	c := fakeDaemon(t, map[string]func(http.ResponseWriter, *http.Request){
		"POST /api/v1/network/join": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&got)
			reply(200, map[string]any{"status": "joined", "network_id": "net-1",
				"server": map[string]any{"id": "s", "name": "studio", "address": "10.0.0.5:7332"},
				"node":   map[string]any{"id": "n1", "name": "gpu-box"}})(w, r)
		},
	})
	var out bytes.Buffer
	if err := joinCommand(joinArgs, c, &out); err != nil {
		t.Fatal(err)
	}
	if got["server"] != "10.0.0.5:7332" || got["fingerprint"] != joinArgs[5] || got["name"] != "" {
		t.Fatalf("sent %v", got)
	}
	if !strings.Contains(out.String(), "✓ Connected to studio (10.0.0.5:7332)") || !strings.Contains(out.String(), "gpu-box") {
		t.Fatalf("output:\n%s", out.String())
	}
	out.Reset()
	if err := joinCommand(append(joinArgs, "--output", "json"), c, &out); err != nil || !strings.Contains(out.String(), `"name": "studio"`) {
		t.Fatalf("json = %v\n%s", err, out.String())
	}
	if err := joinCommand(append(joinArgs, "--name", "gpu box 2"), c, &bytes.Buffer{}); err != nil || got["name"] != "gpu box 2" {
		t.Fatalf("--name = %v, sent %v", err, got)
	}
	// Missing a flag is a usage error.
	var e *exitError
	if err := joinCommand(joinArgs[:4], c, &out); !errors.As(err, &e) || e.code != 2 {
		t.Fatalf("missing fingerprint err = %v", err)
	}
}

func TestJoinFailures(t *testing.T) {
	cases := map[string]struct {
		code, msg, want string
	}{
		"wrong server": {"JOIN_WRONG_SERVER", "the computer at that address isn't the one the join command was made on", "Join stopped: the computer at that address"},
		"unreachable":  {"JOIN_UNREACHABLE", "could not reach Yggdrasil at 10.0.0.5:7332", "firewall allows TCP port 7332"},
		"expired":      {"JOIN_TOKEN_EXPIRED", "this join token has expired", "yggctl join-token create"},
	}
	for name, tc := range cases {
		c := fakeDaemon(t, map[string]func(http.ResponseWriter, *http.Request){
			"POST /api/v1/network/join": reply(400, apiErr(tc.code, tc.msg)),
		})
		err := joinCommand(joinArgs, c, &bytes.Buffer{})
		var e *exitError
		if !errors.As(err, &e) || e.code != 1 || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	// No daemon at all says so.
	down := daemonClient{base: "http://127.0.0.1:1", client: &http.Client{Timeout: time.Second}}
	if err := joinCommand(joinArgs, down, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "isn't running on this computer") {
		t.Fatalf("no daemon err = %v", err)
	}
}

func TestJoinTokenCommands(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	var ttl map[string]int
	c := fakeDaemon(t, map[string]func(http.ResponseWriter, *http.Request){
		"POST /api/v1/join-tokens": func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&ttl)
			reply(201, map[string]any{"id": "abcd1234", "status": "active", "expires_at": now.Add(30 * time.Minute),
				"command":         "yggctl join --server 10.0.0.5:7332 --token ygj_x --fingerprint sha256:y",
				"install_command": "curl -fsSL https://example/install.sh | sh -s -- join --server 10.0.0.5:7332"})(w, r)
		},
		"GET /api/v1/join-tokens": reply(200, []map[string]any{
			{"id": "abcd1234", "status": "active", "created_at": now.Add(-2 * time.Minute), "expires_at": now.Add(13 * time.Minute)},
			{"id": "efgh5678", "status": "used", "used_by": "gpu-box", "created_at": now.Add(-3 * time.Hour), "expires_at": now.Add(-2 * time.Hour)},
		}),
		"DELETE /api/v1/join-tokens/abcd1234": reply(200, map[string]any{"id": "abcd1234", "status": "revoked"}),
		"DELETE /api/v1/join-tokens/efgh5678": reply(409, apiErr("JOIN_TOKEN_NOT_ACTIVE", "That join token can't be revoked")),
	})
	clock := func() time.Time { return now }
	var out bytes.Buffer
	if err := joinTokenCommand([]string{"create", "--ttl", "30m"}, c, &out, clock); err != nil {
		t.Fatal(err)
	}
	if ttl["ttl_minutes"] != 30 || !strings.Contains(out.String(), "  yggctl join --server 10.0.0.5:7332") || !strings.Contains(out.String(), "expires in 30m and can be used once") ||
		!strings.Contains(out.String(), "this installs it and joins (Linux, macOS):\n\n  curl -fsSL https://example/install.sh") {
		t.Fatalf("create sent %v, printed:\n%s", ttl, out.String())
	}
	out.Reset()
	if err := joinTokenCommand([]string{"list"}, c, &out, clock); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ID", "abcd1234", "2m ago", "in 13m", "used by gpu-box", "3h ago"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("list missing %q:\n%s", want, out.String())
		}
	}
	out.Reset()
	if err := joinTokenCommand([]string{"revoke", "abcd1234"}, c, &out, clock); err != nil || !strings.Contains(out.String(), "Revoked join token abcd1234") {
		t.Fatalf("revoke = %v, %q", err, out.String())
	}
	if err := joinTokenCommand([]string{"revoke", "efgh5678"}, c, &out, clock); err == nil || !strings.Contains(err.Error(), "can't be revoked") {
		t.Fatalf("revoke used err = %v", err)
	}
	var e *exitError
	if err := joinTokenCommand([]string{"create", "--ttl", "48h"}, c, &out, clock); !errors.As(err, &e) || e.code != 2 {
		t.Fatalf("48h err = %v", err)
	}
	out.Reset()
	if err := joinTokenCommand([]string{"list", "--output", "json"}, c, &out, clock); err != nil || !json.Valid(out.Bytes()) {
		t.Fatalf("json list = %v, %q", err, out.String())
	}
}

func TestNetworkAndLeave(t *testing.T) {
	c := fakeDaemon(t, map[string]func(http.ResponseWriter, *http.Request){
		"GET /api/v1/network": reply(200, map[string]any{"network_id": "net-1", "reachable": true,
			"node":  map[string]any{"id": "n1", "name": "gpu-box", "address": "10.0.0.22:7332", "fingerprint": "sha256:abc"},
			"peers": []map[string]any{{"id": "s", "name": "studio", "address": "10.0.0.5:7332", "status": "online"}}}),
		"POST /api/v1/network/leave": reply(200, map[string]any{"left": []string{"studio"}, "unreachable": []string{"laptop"}}),
	})
	var out bytes.Buffer
	if err := networkCommand([]string{"status"}, c, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"gpu-box", "sha256:abc", "net-1", "studio", "online"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("network missing %q:\n%s", want, out.String())
		}
	}
	out.Reset()
	if err := leaveCommand(nil, c, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Told: studio") || !strings.Contains(out.String(), "Couldn't reach laptop") {
		t.Fatalf("leave:\n%s", out.String())
	}
}

func TestJoinWaitsForTheDaemon(t *testing.T) {
	// Nothing listens yet; a daemon appears after a second.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/network/join" {
			reply(200, map[string]any{"status": "joined", "server": map[string]any{"name": "studio"}, "node": map[string]any{"name": "gpu-box"}})(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	})}
	go func() {
		time.Sleep(time.Second)
		if l, err := net.Listen("tcp", addr); err == nil {
			_ = srv.Serve(l)
		}
	}()
	t.Cleanup(func() { _ = srv.Close() })
	c := daemonClient{base: "http://" + addr, client: &http.Client{Timeout: time.Second}}
	if err := joinCommand(joinArgs, c, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "isn't running") {
		t.Fatalf("without --wait err = %v", err)
	}
	var out bytes.Buffer
	if err := joinCommand(append(joinArgs, "--wait", "10s"), c, &out); err != nil || !strings.Contains(out.String(), "Joined") {
		t.Fatalf("with --wait = %v\n%s", err, out.String())
	}
}
