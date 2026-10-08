package auth

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// The API certificate is ECDSA, which browsers and phones accept, and is
// kept, so its fingerprint stays the same for a phone that saved it (#213).
func TestAPICertificateIsKept(t *testing.T) {
	dir := t.TempDir()
	first, err := APICertificate(NewSecretStore(dir))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := first.Leaf.PublicKey.(*ecdsa.PublicKey); !ok {
		t.Fatalf("key %T", first.Leaf.PublicKey)
	}
	if !contains(first.Leaf.DNSNames, "localhost") {
		t.Fatalf("names %v", first.Leaf.DNSNames)
	}
	again, err := APICertificate(NewSecretStore(dir))
	if err != nil || CertFingerprint(again) != CertFingerprint(first) {
		t.Fatalf("a second load changed the certificate: %v", err)
	}
	sum := sha256.Sum256(first.Certificate[0])
	if fp := CertFingerprint(first); fp != "sha256:"+hex.EncodeToString(sum[:]) {
		t.Fatalf("fingerprint %s", fp)
	}
	if st, err := os.Stat(filepath.Join(dir, "secrets", "api-tls.key")); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("key file: %v %v", st, err)
	}
	if got := ShortFingerprint("sha256:e863b5098ee6"); got != "E863-B509" {
		t.Fatalf("short %q", got)
	}

	// The person's own certificate, as files.
	own, err := LoadCertificate(filepath.Join(dir, "secrets", "api-tls.crt"), filepath.Join(dir, "secrets", "api-tls.key"))
	if err != nil || CertFingerprint(own) != CertFingerprint(first) {
		t.Fatalf("own: %v", err)
	}
	if _, err := LoadCertificate(filepath.Join(dir, "missing.crt"), filepath.Join(dir, "missing.key")); err == nil {
		t.Fatal("a missing certificate loaded")
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
