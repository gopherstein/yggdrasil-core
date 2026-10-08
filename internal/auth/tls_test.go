package auth

import (
	"context"
	"testing"
)

// A paired computer's pin is the key it paired with, and once it speaks
// TLS that is remembered until it is paired again (#175).
func TestLinkPinAndTLSMarker(t *testing.T) {
	a, b := newPairingPeer(t, "a"), newPairingPeer(t, "b")
	ctx := context.Background()
	if err := a.pm.Trust(ctx, "b", "B", "127.0.0.1:7332", b.id.CertPEM); err != nil {
		t.Fatal(err)
	}
	pin, seen, err := a.pm.Link(ctx, "b")
	if err != nil || pin != b.id.Fingerprint() || seen {
		t.Fatalf("link = %q %v %v", pin, seen, err)
	}
	if err := a.pm.MarkTLS(ctx, "b"); err != nil {
		t.Fatal(err)
	}
	if _, seen, _ := a.pm.Link(ctx, "b"); !seen {
		t.Fatal("TLS wasn't remembered")
	}
	// Paired again: it starts over.
	if err := a.pm.Trust(ctx, "b", "B", "127.0.0.1:7332", b.id.CertPEM); err != nil {
		t.Fatal(err)
	}
	if _, seen, _ := a.pm.Link(ctx, "b"); seen {
		t.Fatal("TLS marker survived pairing again")
	}
	if _, _, err := a.pm.Link(ctx, "nobody"); err == nil {
		t.Fatal("a computer that isn't paired has a link")
	}
}

// The TLS certificate carries the identity key, so its fingerprint is the
// one pairing stores.
func TestTLSCertificateCarriesTheIdentityKey(t *testing.T) {
	a := newPairingPeer(t, "a")
	cert, err := a.id.TLSCertificate()
	if err != nil {
		t.Fatal(err)
	}
	pub, err := PeerKey(cert.Certificate)
	if err != nil || KeyFingerprint(pub) != a.id.Fingerprint() {
		t.Fatalf("key %v %v", pub, err)
	}
	if err := PinnedTLSConfig(a.id.Fingerprint()).VerifyPeerCertificate(cert.Certificate, nil); err != nil {
		t.Fatalf("own pin refused: %v", err)
	}
	if err := PinnedTLSConfig("sha256:00").VerifyPeerCertificate(cert.Certificate, nil); err != ErrPinMismatch {
		t.Fatalf("wrong pin: %v", err)
	}
}
