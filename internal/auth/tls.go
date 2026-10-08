package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// Bifrost over TLS (#175): each computer serves HTTPS with a certificate
// made from its ed25519 identity key, and a paired computer is checked by
// that key's fingerprint, the one stored when they paired, instead of by a
// certificate authority.

// TLSCertificate is a self-signed certificate for this computer's identity
// key. It is made at start: what a peer checks is the key, which doesn't
// change, not the certificate.
func (id *NodeIdentity) TLSCertificate() (tls.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	if err != nil {
		return tls.Certificate{}, err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "toskar-node-" + id.NodeID},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, id.PublicKey, id.PrivateKey)
	if err != nil {
		return tls.Certificate{}, err
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: id.PrivateKey, Leaf: leaf}, nil
}

// ServerTLSConfig serves this computer's identity certificate.
func (id *NodeIdentity) ServerTLSConfig() (*tls.Config, error) {
	cert, err := id.TLSCertificate()
	if err != nil {
		return nil, err
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}, nil
}

// ErrPinMismatch is a computer whose key isn't the one stored when it was
// paired: another computer answering at its address.
var ErrPinMismatch = errors.New("the computer at this address doesn't have the key expected for it, so it may not be the computer you meant")

// PeerKey is the ed25519 key of a TLS peer's certificate.
func PeerKey(rawCerts [][]byte) (ed25519.PublicKey, error) {
	if len(rawCerts) == 0 {
		return nil, errors.New("no certificate")
	}
	cert, err := x509.ParseCertificate(rawCerts[0])
	if err != nil {
		return nil, err
	}
	pub, ok := cert.PublicKey.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not a Toskar certificate: %T key", cert.PublicKey)
	}
	return pub, nil
}

// PinnedTLSConfig accepts only a computer whose key has fingerprint pin
// (KeyFingerprint form). With pin empty, any Toskar computer is accepted:
// the connection is encrypted but not checked, for the steps before
// pairing, which check each other's signatures themselves.
func PinnedTLSConfig(pin string) *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS13,
		// Checked below by key, not by a certificate authority or name.
		InsecureSkipVerify: true, //nolint:gosec // pinned by key in VerifyPeerCertificate
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			pub, err := PeerKey(rawCerts)
			if err != nil {
				return err
			}
			if pin != "" && KeyFingerprint(pub) != pin {
				return ErrPinMismatch
			}
			return nil
		},
	}
}
