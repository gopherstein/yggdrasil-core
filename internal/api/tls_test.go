package api

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/yeixio/toskar-core/internal/auth"
)

// The API answers HTTPS and plain HTTP on one port, also after it moves,
// and says which certificate it serves (#213).
func TestAPISpeaksHTTPS(t *testing.T) {
	cert, err := auth.APICertificate(auth.NewSecretStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	fp := auth.CertFingerprint(cert)
	srv.SetTLS(&tls.Config{Certificates: []tls.Certificate{cert}}, APITLS{Enabled: true, Fingerprint: fp, Short: auth.ShortFingerprint(fp)})
	go func() { _ = srv.ListenAndServe("127.0.0.1:0") }()
	deadline := time.Now().Add(3 * time.Second)
	for srv.Addr() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	t.Cleanup(func() { _ = srv.Shutdown(t.Context()) })

	// A client that checks by fingerprint, as a phone that saved it does.
	pinned := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // checked by fingerprint below
		VerifyConnection: func(cs tls.ConnectionState) error {
			if got := auth.CertFingerprint(tls.Certificate{Certificate: [][]byte{cs.PeerCertificates[0].Raw}}); got != fp {
				t.Errorf("served %s, want %s", got, fp)
			}
			return nil
		},
	}}}
	check := func(addr string) {
		t.Helper()
		for _, url := range []string{"http://" + addr + "/api/v1/health", "https://" + addr + "/api/v1/health"} {
			resp, err := pinned.Get(url)
			if err != nil {
				t.Fatalf("%s: %v", url, err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("%s: %d", url, resp.StatusCode)
			}
		}
	}
	check(srv.Addr())
	if err := srv.Rebind("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	check(srv.Addr())

	resp, err := pinned.Get("https://" + srv.Addr() + "/api/v1/tls")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var info APITLS
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil || !info.Enabled || info.Fingerprint != fp || info.Short == "" {
		t.Fatalf("tls = %+v %v", info, err)
	}
}
