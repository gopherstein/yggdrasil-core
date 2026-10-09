// Package rendezvous holds the computer's side of access from anywhere's
// introductions (#456, docs/remote-access.md): the route secret paired
// devices get, the route ID and record key derived from it, and the signed,
// encrypted address record the rendezvous stores without being able to read.
package rendezvous

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yeixio/toskar-core/internal/auth"
)

// secretName is the route secret's file in the secret store.
const secretName = "route-secret"

// SecretSize is the route secret's length in bytes.
const SecretSize = 32

// RouteSecret is this computer's route secret, made the first time. Paired
// devices get it; the route ID and the record key come from it, so someone
// who knows only the certificate's fingerprint can't find the computer.
func RouteSecret(secrets *auth.SecretStore) ([]byte, error) {
	if raw, err := secrets.Read(secretName); err == nil {
		if b, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw)); err == nil && len(b) == SecretSize {
			return b, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	b := make([]byte, SecretSize)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	if err := secrets.Write(secretName, base64.RawURLEncoding.EncodeToString(b)); err != nil {
		return nil, err
	}
	return b, nil
}

// EncodeSecret is the route secret as paired devices get it.
func EncodeSecret(secret []byte) string { return base64.RawURLEncoding.EncodeToString(secret) }

var lowerBase32 = base32.NewEncoding("abcdefghijklmnopqrstuvwxyz234567").WithPadding(base32.NoPadding)

// RouteID is the name the computer is registered and reached under: 20
// bytes from the secret, as 32 lowercase base32 characters, one DNS label.
func RouteID(secret []byte) string {
	b, _ := hkdf.Key(sha256.New, secret, nil, "toskar route id", 20)
	return lowerBase32.EncodeToString(b)
}

func recordKey(secret []byte) []byte {
	b, _ := hkdf.Key(sha256.New, secret, nil, "toskar route key", 32)
	return b
}

// signed is what the certificate's key signs.
type signed struct {
	V         int      `json:"v"`
	Route     string   `json:"route"`
	At        int64    `json:"at"`
	Addresses []string `json:"addresses"`
}

// record is what the record key seals.
type record struct {
	// Payload is the signed part, as signed.
	Payload []byte `json:"payload"`
	// Cert is the API certificate, whose SHA-256 is the devices' pin.
	Cert []byte `json:"cert"`
	// Alg is ecdsa-sha256, rsa-pkcs1-sha256, or ed25519.
	Alg string `json:"alg"`
	Sig []byte `json:"sig"`
}

// Seal makes the address record: the addresses and the time, signed with
// the API certificate's key and sealed with the record key, so only a paired
// device can read it, and only this computer can have written it.
func Seal(secret []byte, cert tls.Certificate, addresses []string, now time.Time) ([]byte, error) {
	if len(cert.Certificate) == 0 {
		return nil, errors.New("no certificate")
	}
	route := RouteID(secret)
	payload, err := json.Marshal(signed{V: 1, Route: route, At: now.Unix(), Addresses: addresses})
	if err != nil {
		return nil, err
	}
	alg, sig, err := sign(cert.PrivateKey, payload)
	if err != nil {
		return nil, err
	}
	plain, err := json.Marshal(record{Payload: payload, Cert: cert.Certificate[0], Alg: alg, Sig: sig})
	if err != nil {
		return nil, err
	}
	gcm, err := aead(secret)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plain, []byte(route)), nil
}

func sign(key crypto.PrivateKey, payload []byte) (string, []byte, error) {
	sum := sha256.Sum256(payload)
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		sig, err := ecdsa.SignASN1(rand.Reader, k, sum[:])
		return "ecdsa-sha256", sig, err
	case *rsa.PrivateKey:
		sig, err := rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, sum[:])
		return "rsa-pkcs1-sha256", sig, err
	case ed25519.PrivateKey:
		return "ed25519", ed25519.Sign(k, payload), nil
	}
	return "", nil, fmt.Errorf("can't sign with a %T key", key)
}

func aead(secret []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(recordKey(secret))
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Errors from opening a record.
var (
	ErrRecordSealed = errors.New("the record doesn't open with this route secret")
	ErrRecordPin    = errors.New("the record isn't from the pinned computer")
	ErrRecordStale  = errors.New("the record is too old")
)

// Opened is what an address record says.
type Opened struct {
	Addresses []string
	At        time.Time
}

// Open reads an address record as a paired device does: it opens it with
// the route secret, checks that the certificate is the pinned one
// ("sha256:<hex>" of the certificate) and the signature holds, and that it
// isn't older than maxAge. The apps do the same; this is the reference.
func Open(blob, secret []byte, pin string, now time.Time, maxAge time.Duration) (Opened, error) {
	gcm, err := aead(secret)
	if err != nil {
		return Opened{}, err
	}
	route := RouteID(secret)
	if len(blob) < gcm.NonceSize() {
		return Opened{}, ErrRecordSealed
	}
	plain, err := gcm.Open(nil, blob[:gcm.NonceSize()], blob[gcm.NonceSize():], []byte(route))
	if err != nil {
		return Opened{}, ErrRecordSealed
	}
	var rec record
	if err := json.Unmarshal(plain, &rec); err != nil {
		return Opened{}, ErrRecordSealed
	}
	sum := sha256.Sum256(rec.Cert)
	if !strings.EqualFold(pin, "sha256:"+hex.EncodeToString(sum[:])) {
		return Opened{}, ErrRecordPin
	}
	cert, err := x509.ParseCertificate(rec.Cert)
	if err != nil {
		return Opened{}, ErrRecordPin
	}
	if !verify(cert.PublicKey, rec.Alg, rec.Payload, rec.Sig) {
		return Opened{}, ErrRecordPin
	}
	var body signed
	if err := json.Unmarshal(rec.Payload, &body); err != nil || body.V != 1 || body.Route != route {
		return Opened{}, ErrRecordPin
	}
	at := time.Unix(body.At, 0)
	if now.Sub(at) > maxAge || at.Sub(now) > 5*time.Minute {
		return Opened{}, ErrRecordStale
	}
	return Opened{Addresses: body.Addresses, At: at}, nil
}

func verify(pub any, alg string, payload, sig []byte) bool {
	sum := sha256.Sum256(payload)
	switch k := pub.(type) {
	case *ecdsa.PublicKey:
		return alg == "ecdsa-sha256" && ecdsa.VerifyASN1(k, sum[:], sig)
	case *rsa.PublicKey:
		return alg == "rsa-pkcs1-sha256" && rsa.VerifyPKCS1v15(k, crypto.SHA256, sum[:], sig) == nil
	case ed25519.PublicKey:
		return alg == "ed25519" && ed25519.Verify(k, payload, sig)
	}
	return false
}
