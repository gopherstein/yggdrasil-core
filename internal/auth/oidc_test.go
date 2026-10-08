package auth_test

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/store"
)

// fakeProvider is an OpenID Connect provider that signs its ID tokens with
// an RSA or an EC key, and lets a test change what it says.
type fakeProvider struct {
	srv      *httptest.Server
	rsaKey   *rsa.PrivateKey
	ecKey    *ecdsa.PrivateKey
	alg      string
	claims   func(nonce string) map[string]any
	verifier string
	nonce    string
}

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()
	rk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	ek, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	p := &fakeProvider{rsaKey: rk, ecKey: ek, alg: "RS256"}
	mux := http.NewServeMux()
	p.srv = httptest.NewServer(mux)
	t.Cleanup(p.srv.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": p.srv.URL, "authorization_endpoint": p.srv.URL + "/authorize",
			"token_endpoint": p.srv.URL + "/token", "jwks_uri": p.srv.URL + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]any{
			{"kty": "RSA", "kid": "r1", "use": "sig", "n": b64(rk.N.Bytes()), "e": b64(big.NewInt(int64(rk.E)).Bytes())},
			{"kty": "EC", "kid": "e1", "crv": "P-256", "x": b64(ek.X.FillBytes(make([]byte, 32))), "y": b64(ek.Y.FillBytes(make([]byte, 32)))},
		}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		id, secret, ok := r.BasicAuth()
		sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
		if !ok || id != "toskar" || secret != "s3cret" || r.PostForm.Get("code") != "good-code" || b64(sum[:]) != p.verifier {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": p.sign(p.claims(p.nonce))})
	})
	p.claims = func(nonce string) map[string]any {
		return map[string]any{
			"iss": p.srv.URL, "aud": "toskar", "sub": "u-123", "nonce": nonce,
			"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
			"email": "sam@example.com", "email_verified": true, "name": "Sam", "groups": []string{"family"},
		}
	}
	return p
}

func (p *fakeProvider) sign(claims map[string]any) string {
	kid := "r1"
	if p.alg == "ES256" {
		kid = "e1"
	}
	h, _ := json.Marshal(map[string]string{"alg": p.alg, "kid": kid, "typ": "JWT"})
	c, _ := json.Marshal(claims)
	signed := b64(h) + "." + b64(c)
	sum := sha256.Sum256([]byte(signed))
	var sig []byte
	if p.alg == "ES256" {
		r, s, _ := ecdsa.Sign(rand.Reader, p.ecKey, sum[:])
		sig = append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
	} else {
		sig, _ = rsa.SignPKCS1v15(rand.Reader, p.rsaKey, crypto.SHA256, sum[:])
	}
	return signed + "." + b64(sig)
}

// signIn goes through Start as a browser would, and Finish with code.
func (p *fakeProvider) signIn(t *testing.T, o *auth.OIDC, code string) (auth.OIDCIdentity, string, error) {
	t.Helper()
	to, err := o.Start(context.Background(), "https://toskar.example.com/api/v1/oidc/callback", "/chat")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(to)
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("redirect_uri") != "https://toskar.example.com/api/v1/oidc/callback" || !strings.Contains(q.Get("scope"), "openid") {
		t.Fatalf("authorization request %s", to)
	}
	p.verifier, p.nonce = q.Get("code_challenge"), q.Get("nonce")
	return o.Finish(context.Background(), q.Get("state"), code)
}

func newOIDC(t *testing.T, p *fakeProvider, s auth.OIDCSettings) *auth.OIDC {
	t.Helper()
	s.Issuer, s.ClientID, s.ClientSecret = p.srv.URL, "toskar", "s3cret"
	o, err := auth.NewOIDC(s)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestOIDCSignIn(t *testing.T) {
	p := newFakeProvider(t)
	o := newOIDC(t, p, auth.OIDCSettings{MemberGroups: []string{"family"}, DefaultRole: "none"})
	for _, alg := range []string{"RS256", "ES256"} {
		p.alg = alg
		id, back, err := p.signIn(t, o, "good-code")
		if err != nil || id.Subject != "u-123" || id.Email != "sam@example.com" || !id.EmailVerified || back != "/chat" || len(id.Groups) != 1 {
			t.Fatalf("%s: %+v %q %v", alg, id, back, err)
		}
	}

	// A state is good once.
	to, _ := o.Start(context.Background(), "https://toskar.example.com/api/v1/oidc/callback", "/")
	u, _ := url.Parse(to)
	p.verifier, p.nonce = u.Query().Get("code_challenge"), u.Query().Get("nonce")
	if _, _, err := o.Finish(context.Background(), u.Query().Get("state"), "good-code"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := o.Finish(context.Background(), u.Query().Get("state"), "good-code"); err == nil {
		t.Fatal("a state was used twice")
	}
	if _, _, err := p.signIn(t, o, "stolen-code"); err == nil {
		t.Fatal("a code the provider refused signed someone in")
	}
}

func TestOIDCRefusesBadTokens(t *testing.T) {
	p := newFakeProvider(t)
	o := newOIDC(t, p, auth.OIDCSettings{})
	good := p.claims
	cases := map[string]func(map[string]any){
		"another audience": func(c map[string]any) { c["aud"] = "someone-else" },
		"another issuer":   func(c map[string]any) { c["iss"] = "https://evil.example.com" },
		"expired":          func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() },
		"another nonce":    func(c map[string]any) { c["nonce"] = "replayed" },
		"nobody":           func(c map[string]any) { delete(c, "sub") },
		"another client":   func(c map[string]any) { c["aud"] = []string{"toskar", "other"}; c["azp"] = "other" },
	}
	for name, change := range cases {
		p.claims = func(nonce string) map[string]any {
			c := good(nonce)
			change(c)
			return c
		}
		if _, _, err := p.signIn(t, o, "good-code"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	// Signed by a key the provider doesn't publish.
	p.claims = good
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	p.rsaKey = other
	if _, _, err := p.signIn(t, o, "good-code"); err == nil {
		t.Error("a forged signature was accepted")
	}
}

func TestOIDCPeople(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	people := auth.NewPeople(db.SQL)
	p := newFakeProvider(t)
	o := newOIDC(t, p, auth.OIDCSettings{AdminGroups: []string{"admins"}, DefaultRole: "visitor", OwnerEmail: "mike@example.com"})
	ctx := context.Background()

	sam, err := people.SignedInWith(ctx, o, auth.OIDCIdentity{Subject: "u-1", Email: "sam@example.com", Name: "Sam"})
	if err != nil || sam.Role != auth.RoleVisitor || sam.Name != "Sam" || sam.External != "sam@example.com" {
		t.Fatalf("sam: %+v %v", sam, err)
	}
	if again, _ := people.SignedInWith(ctx, o, auth.OIDCIdentity{Subject: "u-1", Email: "sam@new.example.com", Groups: []string{"admins"}}); again.ID != sam.ID || again.Role != auth.RoleAdmin {
		t.Fatalf("sam joined admins: %+v", again)
	}
	if owner, _ := people.SignedInWith(ctx, o, auth.OIDCIdentity{Subject: "u-9", Email: "Mike@example.com", EmailVerified: true}); owner.ID != auth.OwnerID {
		t.Fatalf("the owner: %+v", owner)
	}
	// An email the provider hasn't verified doesn't make anyone the Owner.
	if who, _ := people.SignedInWith(ctx, o, auth.OIDCIdentity{Subject: "u-8", Email: "mike@example.com"}); who.ID == auth.OwnerID {
		t.Fatal("an unverified email became the owner")
	}
	strict := newOIDC(t, p, auth.OIDCSettings{DefaultRole: "none"})
	if _, err := people.SignedInWith(ctx, strict, auth.OIDCIdentity{Subject: "u-2"}); !errors.Is(err, auth.ErrOIDCRefused) {
		t.Fatalf("no role: %v", err)
	}
}

func TestOIDCSettings(t *testing.T) {
	if o, err := auth.NewOIDC(auth.OIDCSettings{}); o != nil || err != nil {
		t.Fatalf("none: %v %v", o, err)
	}
	for name, s := range map[string]auth.OIDCSettings{
		"no client":    {Issuer: "https://id.example.com"},
		"not a url":    {Issuer: "id.example.com", ClientID: "x"},
		"bad role":     {Issuer: "https://id.example.com", ClientID: "x", DefaultRole: "owner"},
		"bad redirect": {Issuer: "https://id.example.com", ClientID: "x", RedirectURL: "/callback"},
	} {
		if _, err := auth.NewOIDC(s); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	o, _ := auth.NewOIDC(auth.OIDCSettings{Issuer: "https://id.example.com/", ClientID: "x"})
	if o.Label() != "id.example.com" || o.RedirectURL("http://192.168.1.20:7331") != "http://192.168.1.20:7331/api/v1/oidc/callback" {
		t.Fatalf("label %q, redirect %q", o.Label(), o.RedirectURL("http://192.168.1.20:7331"))
	}
}
