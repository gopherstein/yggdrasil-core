package rendezvous

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"errors"
	"math/big"
	"slices"
	"testing"
	"time"

	"github.com/yeixio/toskar-core/internal/auth"
)

func pinOf(cert tls.Certificate) string {
	sum := sha256.Sum256(cert.Certificate[0])
	return "sha256:" + hex.EncodeToString(sum[:])
}

// The route secret is made once and kept; its route ID is one DNS label.
func TestRouteSecret(t *testing.T) {
	store := auth.NewSecretStore(t.TempDir())
	a, err := RouteSecret(store)
	if err != nil || len(a) != SecretSize {
		t.Fatalf("%v %v", a, err)
	}
	b, _ := RouteSecret(store)
	if !slices.Equal(a, b) {
		t.Fatal("a new secret each time")
	}
	id := RouteID(a)
	if len(id) != 32 || id != RouteID(b) {
		t.Fatalf("route id: %q", id)
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= '2' && c <= '7') {
			t.Fatalf("not a DNS label: %q", id)
		}
	}
}

// A record opens only with the route secret, only from the pinned
// computer, and only while it's fresh (#456).
func TestRecord(t *testing.T) {
	dir := t.TempDir()
	cert, err := auth.APICertificate(auth.NewSecretStore(dir))
	if err != nil {
		t.Fatal(err)
	}
	secret, _ := RouteSecret(auth.NewSecretStore(dir))
	now := time.Unix(1_800_000_000, 0)
	addrs := []string{"203.0.113.9:7333", "[2001:db8::1]:7333"}
	blob, err := Seal(secret, cert, addrs, now)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Open(blob, secret, pinOf(cert), now.Add(time.Minute), 15*time.Minute)
	if err != nil || !slices.Equal(got.Addresses, addrs) || !got.At.Equal(now) {
		t.Fatalf("open: %+v %v", got, err)
	}

	other := make([]byte, SecretSize)
	_, _ = rand.Read(other)
	if _, err := Open(blob, other, pinOf(cert), now, time.Hour); !errors.Is(err, ErrRecordSealed) {
		t.Fatalf("another secret: %v", err)
	}
	if _, err := Open(blob, secret, "sha256:"+hex.EncodeToString(make([]byte, 32)), now, time.Hour); !errors.Is(err, ErrRecordPin) {
		t.Fatalf("another pin: %v", err)
	}
	if _, err := Open(blob, secret, pinOf(cert), now.Add(time.Hour), 15*time.Minute); !errors.Is(err, ErrRecordStale) {
		t.Fatalf("old: %v", err)
	}
	tampered := slices.Clone(blob)
	tampered[len(tampered)-1] ^= 1
	if _, err := Open(tampered, secret, pinOf(cert), now, time.Hour); !errors.Is(err, ErrRecordSealed) {
		t.Fatalf("tampered: %v", err)
	}

	// A record signed by another key, with the pinned certificate inside,
	// fails the signature.
	impostor := otherCert(t)
	impostor.Certificate = cert.Certificate
	forged, err := Seal(secret, impostor, []string{"198.51.100.66:7333"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Open(forged, secret, pinOf(cert), now, time.Hour); !errors.Is(err, ErrRecordPin) {
		t.Fatalf("forged: %v", err)
	}
}

// A certificate of the person's own, with an Ed25519 key, signs too.
func TestRecordEd25519(t *testing.T) {
	cert := otherCert(t)
	secret := make([]byte, SecretSize)
	_, _ = rand.Read(secret)
	now := time.Now()
	blob, err := Seal(secret, cert, []string{"home.example.com:7333"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := Open(blob, secret, pinOf(cert), now, time.Minute); err != nil || got.Addresses[0] != "home.example.com:7333" {
		t.Fatalf("%+v %v", got, err)
	}
}

func otherCert(t *testing.T) tls.Certificate {
	t.Helper()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: "other"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, priv)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: priv}
}
