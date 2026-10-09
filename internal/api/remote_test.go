package api

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/config"
)

// The remote listener serves paired devices' keys on phone routes, over
// TLS, and nothing else: no "this computer", no other keys, no pages, and
// an address that keeps sending wrong keys waits (#456).
func TestRemoteListener(t *testing.T) {
	dir := t.TempDir()
	mgr, err := config.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := auth.APICertificate(auth.NewSecretStore(dir))
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(Dependencies{
		Config: mgr,
		VerifyAPIKey: func(_ context.Context, secret string) (auth.APIKeyRecord, error) {
			switch secret {
			case "phone-key":
				return auth.APIKeyRecord{ID: "d1", PersonID: auth.OwnerID, Kind: auth.KindDevice}, nil
			case "api-key":
				return auth.APIKeyRecord{ID: "k1", PersonID: auth.OwnerID}, nil
			}
			return auth.APIKeyRecord{}, errors.New("invalid")
		},
	})
	srv.SetTLS(&tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}, APITLS{Enabled: true})
	if err := srv.ServeRemote("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.StopRemote)
	t.Cleanup(func() { remoteFailures.Forget("127.0.0.1") })
	base := "https://" + srv.RemoteListening().Listening
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	call := func(method, path, key string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest(method, base+path, strings.NewReader("{}"))
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var b strings.Builder
		buf := make([]byte, 2048)
		n, _ := res.Body.Read(buf)
		b.Write(buf[:n])
		return res.StatusCode, b.String()
	}

	if code, _ := call(http.MethodGet, "/api/v1/health", "phone-key"); code != http.StatusOK {
		t.Fatalf("a phone's key: %d", code)
	}
	if code, body := call(http.MethodGet, "/api/v1/api-keys", "phone-key"); code != http.StatusForbidden || !strings.Contains(body, "DEVICE_NOT_ALLOWED") {
		t.Fatalf("a phone's key off its routes: %d %s", code, body)
	}
	if code, body := call(http.MethodGet, "/api/v1/health", "api-key"); code != http.StatusForbidden || !strings.Contains(body, "REMOTE_DEVICES_ONLY") {
		t.Fatalf("an API Access key: %d %s", code, body)
	}
	// From this very computer, with no key: still nobody.
	if code, _ := call(http.MethodGet, "/api/v1/settings", ""); code != http.StatusUnauthorized {
		t.Fatalf("no key: %d", code)
	}
	if code, _ := call(http.MethodPost, "/api/v1/devices/pair", ""); code != http.StatusUnauthorized {
		t.Fatalf("pairing from outside: %d", code)
	}
	if code, _ := call(http.MethodGet, "/", "phone-key"); code != http.StatusNotFound {
		t.Fatalf("pages: %d", code)
	}
	// Wrong keys: twenty, then the address waits, even with a good key.
	remoteFailures.Forget("127.0.0.1")
	for i := 0; i < 20; i++ {
		call(http.MethodGet, "/api/v1/health", "guess")
	}
	if code, body := call(http.MethodGet, "/api/v1/health", "phone-key"); code != http.StatusTooManyRequests || !strings.Contains(body, "REMOTE_THROTTLED") {
		t.Fatalf("after wrong keys: %d %s", code, body)
	}
	remoteFailures.Forget("127.0.0.1")

	// Off closes it.
	addr := srv.RemoteListening().Listening
	srv.StopRemote()
	if srv.RemoteListening().Listening != "" {
		t.Fatal("still listening")
	}
	if _, err := client.Get("https://" + addr + "/api/v1/health"); err == nil {
		t.Fatal("answered after it was turned off")
	}
}

// Without a certificate there's nothing to listen with.
func TestRemoteListenerNeedsTLS(t *testing.T) {
	srv := NewServer(Dependencies{})
	if err := srv.ServeRemote("127.0.0.1:0"); err == nil || srv.RemoteListening().Error == "" {
		t.Fatalf("listened without a certificate: %v", err)
	}
}

// The route secret comes only on the home network, never through the
// remote listener, and with pairing (#456).
func TestRouteSecret(t *testing.T) {
	dir := t.TempDir()
	mgr, err := config.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Update(func(c *config.Config) { c.APIHost, c.LANAPIEnabled = "0.0.0.0", true }); err != nil {
		t.Fatal(err)
	}
	cert, err := auth.APICertificate(auth.NewSecretStore(dir))
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(Dependencies{
		Config: mgr,
		VerifyAPIKey: func(_ context.Context, secret string) (auth.APIKeyRecord, error) {
			if secret == "phone-key" {
				return auth.APIKeyRecord{ID: "d1", PersonID: auth.OwnerID, Kind: auth.KindDevice}, nil
			}
			return auth.APIKeyRecord{}, errors.New("invalid")
		},
		RouteSecret: func() (string, string, error) { return "c2VjcmV0", "routeid", nil },
	})
	srv.SetTLS(&tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}, APITLS{Enabled: true})

	// On the home network, with a phone's key.
	r := httptest.NewRequest(http.MethodGet, "/api/v1/remote-access/route", nil)
	r.RemoteAddr, r.Host = "192.168.1.20:50000", "192.168.1.5:7331"
	r.Header.Set("Authorization", "Bearer phone-key")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, r)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"route_secret":"c2VjcmV0"`) {
		t.Fatalf("home network: %d %s", rec.Code, rec.Body)
	}

	// Through the remote listener: refused, though the key is good.
	if err := srv.ServeRemote("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.StopRemote)
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	req, _ := http.NewRequest(http.MethodGet, "https://"+srv.RemoteListening().Listening+"/api/v1/remote-access/route", nil)
	req.Header.Set("Authorization", "Bearer phone-key")
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("remote: %d", res.StatusCode)
	}
}

// pipeConn is a device's connection as the relay client hands it over:
// its address is the device's, not the relay's.
type pipeConn struct {
	net.Conn
	remote net.Addr
}

func (c pipeConn) RemoteAddr() net.Addr { return c.remote }

// A device through the relay meets the remote listener as a direct one
// does: TLS with the API's certificate, paired devices' keys only, wrong
// keys counted against the device's address (#456).
func TestServeRelayed(t *testing.T) {
	dir := t.TempDir()
	mgr, err := config.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := auth.APICertificate(auth.NewSecretStore(dir))
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(Dependencies{
		Config: mgr,
		VerifyAPIKey: func(_ context.Context, secret string) (auth.APIKeyRecord, error) {
			if secret == "phone-key" {
				return auth.APIKeyRecord{ID: "d1", PersonID: auth.OwnerID, Kind: auth.KindDevice}, nil
			}
			return auth.APIKeyRecord{}, errors.New("invalid")
		},
	})
	if srv.ServeRelayed(nil) {
		t.Fatal("took a connection with the listener off")
	}
	srv.SetTLS(&tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}, APITLS{Enabled: true})
	if err := srv.ServeRemote("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.StopRemote)
	device := "203.0.113.50"
	t.Cleanup(func() { remoteFailures.Forget(device) })
	pin := sha256.Sum256(cert.Certificate[0])
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			a, b := net.Pipe()
			if !srv.ServeRelayed(pipeConn{Conn: b, remote: &net.TCPAddr{IP: net.ParseIP(device)}}) {
				return nil, errors.New("not taken")
			}
			return a, nil
		},
		// The device trusts the pinned certificate, as through the relay.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true, VerifyPeerCertificate: func(raw [][]byte, _ [][]*x509.Certificate) error {
			if sha256.Sum256(raw[0]) != pin {
				return errors.New("not the pinned certificate")
			}
			return nil
		}},
	}}
	call := func(key string) (int, string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, "https://relayed.invalid/api/v1/health", nil)
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var body struct {
			Error struct{ Code string }
		}
		_ = json.NewDecoder(res.Body).Decode(&body)
		return res.StatusCode, body.Error.Code
	}
	if status, _ := call("phone-key"); status != http.StatusOK {
		t.Fatalf("phone key: %d", status)
	}
	if status, code := call(""); status != http.StatusUnauthorized {
		t.Fatalf("no key: %d %s", status, code)
	}
	for range 20 {
		call("wrong")
	}
	if status, code := call("phone-key"); status != http.StatusTooManyRequests || code != "REMOTE_THROTTLED" {
		t.Fatalf("after wrong keys from the device's address: %d %s", status, code)
	}
	srv.StopRemote()
	if srv.ServeRelayed(pipeConn{remote: &net.TCPAddr{}}) {
		t.Fatal("took a connection after stopping")
	}
}
