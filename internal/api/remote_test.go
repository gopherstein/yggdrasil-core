package api

import (
	"context"
	"crypto/tls"
	"errors"
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
