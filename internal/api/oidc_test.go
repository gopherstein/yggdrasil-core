package api

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/config"
	"github.com/yeixio/toskar-core/internal/store"
)

// A browser signs in with the provider and comes back signed in; a
// callback link from another browser signs nobody in (#206).
func TestOIDCSignInFlow(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	var nonce, challenge string
	mux := http.NewServeMux()
	idp := httptest.NewServer(mux)
	t.Cleanup(idp.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": idp.URL, "authorization_endpoint": idp.URL + "/authorize",
			"token_endpoint": idp.URL + "/token", "jwks_uri": idp.URL + "/jwks", "token_endpoint_auth_methods_supported": []string{"client_secret_post"}})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]any{{"kty": "RSA", "kid": "k", "n": enc(key.N.Bytes()), "e": enc(big.NewInt(int64(key.E)).Bytes())}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
		if r.PostForm.Get("client_secret") != "s3cret" || r.PostForm.Get("code") != "the-code" || enc(sum[:]) != challenge {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		h, _ := json.Marshal(map[string]string{"alg": "RS256", "kid": "k"})
		c, _ := json.Marshal(map[string]any{"iss": idp.URL, "aud": "toskar", "sub": "u-7", "nonce": nonce, "exp": time.Now().Add(time.Hour).Unix(),
			"email": "sam@example.com", "email_verified": true, "name": "Sam", "groups": []string{"family"}})
		signed := enc(h) + "." + enc(c)
		digest := sha256.Sum256([]byte(signed))
		sig, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": signed + "." + enc(sig)})
	})

	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	mgr, err := config.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Update(func(c *config.Config) {
		c.APIHost = "0.0.0.0"
		c.OIDC = config.OIDC{Issuer: idp.URL, ClientID: "toskar", MemberGroups: []string{"family"}, DefaultRole: "none", Label: "Authentik"}
	}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TOSKAR_OIDC_CLIENT_SECRET", "s3cret")
	srv := NewServer(Dependencies{Config: mgr, People: auth.NewPeople(db.SQL), Sessions: auth.NewSessions(db.SQL), Invites: auth.NewInvites(db.SQL)})
	do := func(r *http.Request) *httptest.ResponseRecorder {
		r.RemoteAddr = "192.168.1.30:5000"
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, r)
		return rec
	}

	info := do(httptest.NewRequest(http.MethodGet, "/api/v1/oidc", nil))
	if !strings.Contains(info.Body.String(), `"label":"Authentik"`) {
		t.Fatalf("info: %s", info.Body)
	}

	start := do(httptest.NewRequest(http.MethodGet, "http://192.168.1.20:7331/api/v1/oidc/start?return=/chat", nil))
	if start.Code != http.StatusFound {
		t.Fatalf("start: %d %s", start.Code, start.Body)
	}
	to, _ := url.Parse(start.Header().Get("Location"))
	q := to.Query()
	nonce, challenge = q.Get("nonce"), q.Get("code_challenge")
	if q.Get("redirect_uri") != "http://192.168.1.20:7331/api/v1/oidc/callback" {
		t.Fatalf("redirect_uri %q", q.Get("redirect_uri"))
	}
	var stateCookie *http.Cookie
	for _, c := range start.Result().Cookies() {
		if c.Name == "toskar_oidc" {
			stateCookie = c
		}
	}
	if stateCookie == nil || stateCookie.Value != q.Get("state") || !stateCookie.HttpOnly {
		t.Fatalf("state cookie %+v", stateCookie)
	}
	callback := "http://192.168.1.20:7331/api/v1/oidc/callback?code=the-code&state=" + url.QueryEscape(q.Get("state"))

	// The same link opened in a browser that didn't start it.
	if rec := do(httptest.NewRequest(http.MethodGet, callback, nil)); !strings.Contains(rec.Header().Get("Location"), "oidc_error=failed") {
		t.Fatalf("another browser: %d %s", rec.Code, rec.Header().Get("Location"))
	}

	// The refused link used up nothing: the browser that started still
	// signs in with it.
	req := httptest.NewRequest(http.MethodGet, callback, nil)
	req.AddCookie(stateCookie)
	back := do(req)
	if back.Code != http.StatusSeeOther || back.Header().Get("Location") != "/chat" {
		t.Fatalf("callback: %d %s", back.Code, back.Header().Get("Location"))
	}
	var session *http.Cookie
	for _, c := range back.Result().Cookies() {
		if c.Name == auth.SessionCookie && c.Value != "" {
			session = c
		}
	}
	if session == nil {
		t.Fatal("no session")
	}
	me := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	me.AddCookie(session)
	rec := do(me)
	var p auth.Principal
	_ = json.NewDecoder(rec.Body).Decode(&p)
	if rec.Code != http.StatusOK || p.Person.Name != "Sam" || p.Person.Role != auth.RoleMember || p.Person.External != "sam@example.com" || p.Via != auth.ViaSession {
		t.Fatalf("me: %d %+v", rec.Code, p)
	}
}

func TestLocalPath(t *testing.T) {
	for in, want := range map[string]string{"/chat": "/chat", "//evil.example.com": "/", "https://evil.example.com": "/", "/\\evil": "/", "": "/"} {
		if got := localPath(in); got != want {
			t.Errorf("%q: %q", in, got)
		}
	}
}
