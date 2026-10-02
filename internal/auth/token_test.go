package auth

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestAuthTokenRoundTrip(t *testing.T) {
	dir := t.TempDir()
	secrets := NewSecretStore(dir)
	id, err := LoadOrCreateIdentity(secrets, "node-a")
	if err != nil {
		t.Fatal(err)
	}
	token := id.AuthToken("node-b")
	claims, err := ParseAndVerifyAuthToken(token, id.CertPEM, "node-b")
	if err != nil {
		t.Fatal(err)
	}
	if claims.NodeID != "node-a" || claims.Audience != "node-b" || claims.Nonce == "" {
		t.Fatalf("claims=%+v", claims)
	}
	peeked, err := PeekAuthTokenNodeID(token)
	if err != nil || peeked != "node-a" {
		t.Fatalf("peek=%q err=%v", peeked, err)
	}
	if id.AuthToken("node-b") == token {
		t.Fatal("two tokens should not be equal")
	}
}

func TestAuthTokenRejectsWrongPeer(t *testing.T) {
	dir := t.TempDir()
	a, err := LoadOrCreateIdentity(NewSecretStore(dir+"/a"), "node-a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoadOrCreateIdentity(NewSecretStore(dir+"/b"), "node-b")
	if err != nil {
		t.Fatal(err)
	}
	token := a.AuthToken("node-c")
	if _, err := ParseAndVerifyAuthToken(token, b.CertPEM, "node-c"); err == nil {
		t.Fatal("expected signature failure against wrong peer cert")
	}
}

// A token made for one computer is refused by another.
func TestAuthTokenRejectsWrongAudience(t *testing.T) {
	id, err := LoadOrCreateIdentity(NewSecretStore(t.TempDir()), "node-a")
	if err != nil {
		t.Fatal(err)
	}
	token := id.AuthToken("node-b")
	if _, err := ParseAndVerifyAuthToken(token, id.CertPEM, "node-c"); err == nil {
		t.Fatal("token for node-b accepted by node-c")
	}
	if _, err := ParseAndVerifyAuthToken(id.AuthToken(""), id.CertPEM, ""); err == nil {
		t.Fatal("token without an audience accepted")
	}
}

// Changing the audience after signing breaks the signature.
func TestAuthTokenRejectsRewrittenAudience(t *testing.T) {
	id, err := LoadOrCreateIdentity(NewSecretStore(t.TempDir()), "node-a")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(id.AuthToken("node-b"))
	parts := strings.Split(string(raw), "|")
	parts[2] = "node-c"
	forged := base64.RawURLEncoding.EncodeToString([]byte(strings.Join(parts, "|")))
	if _, err := ParseAndVerifyAuthToken(forged, id.CertPEM, "node-c"); err == nil {
		t.Fatal("rewritten audience accepted")
	}
}

func TestAuthTokenRejectsExpired(t *testing.T) {
	dir := t.TempDir()
	id, err := LoadOrCreateIdentity(NewSecretStore(dir), "node-a")
	if err != nil {
		t.Fatal(err)
	}
	exp := time.Now().UTC().Add(-time.Minute).Unix()
	nonce := "00112233445566778899aabbccddeeff"
	sig := id.Sign(authTokenMessage(id.NodeID, "node-b", exp, nonce))
	raw := fmt.Sprintf("v2|%s|node-b|%d|%s|%x", id.NodeID, exp, nonce, sig)
	token := base64.RawURLEncoding.EncodeToString([]byte(raw))
	if _, err := ParseAndVerifyAuthToken(token, id.CertPEM, "node-b"); err == nil {
		t.Fatal("expected expired token error")
	}
}

// Version 1 tokens, without an audience or nonce, are refused.
func TestAuthTokenRejectsLegacyFormat(t *testing.T) {
	id, err := LoadOrCreateIdentity(NewSecretStore(t.TempDir()), "node-a")
	if err != nil {
		t.Fatal(err)
	}
	exp := time.Now().UTC().Add(time.Minute).Unix()
	sig := id.Sign([]byte(fmt.Sprintf("%s|%d", id.NodeID, exp)))
	token := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%s|%d|%x", id.NodeID, exp, sig)))
	if _, err := ParseAndVerifyAuthToken(token, id.CertPEM, "node-b"); err == nil {
		t.Fatal("legacy token accepted")
	}
}

func TestAuthTokenRejectsUnsignedGarbage(t *testing.T) {
	if _, err := ParseAndVerifyAuthToken("not-a-token", []byte("x"), "node-b"); err == nil {
		t.Fatal("expected error")
	}
}

func TestReplayGuardAcceptsANonceOnce(t *testing.T) {
	id, err := LoadOrCreateIdentity(NewSecretStore(t.TempDir()), "node-a")
	if err != nil {
		t.Fatal(err)
	}
	g := NewReplayGuard()
	claims, err := ParseAndVerifyAuthToken(id.AuthToken("node-b"), id.CertPEM, "node-b")
	if err != nil {
		t.Fatal(err)
	}
	if !g.Use(claims) {
		t.Fatal("first use refused")
	}
	if g.Use(claims) {
		t.Fatal("replayed token accepted")
	}
}
