package auth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Sign-in with OpenID Connect (#206): Google, Microsoft Entra ID, Okta,
// Authentik, Keycloak, Authelia, and other providers. Toskar sends the
// browser to the provider with PKCE, takes the code back, and swaps it for
// an ID token straight from the provider's token endpoint over TLS. The
// token's signature is checked against the provider's published keys too,
// with its issuer, audience, expiry, and nonce.

// ErrOIDCRefused is someone the provider signed in whom no role lets in.
var ErrOIDCRefused = errors.New("your account there doesn't give you a role on this Toskar")

// OIDCSettings configure sign-in with a provider; see config.OIDC.
type OIDCSettings struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	// RedirectURL is the callback the provider sends people back to;
	// empty is worked out from each request.
	RedirectURL  string
	Scopes       []string
	GroupsClaim  string
	AdminGroups  []string
	MemberGroups []string
	DefaultRole  string
	// OwnerEmail and OwnerSubject say who at the provider is the Owner.
	// An email counts only when the provider says it's verified.
	OwnerEmail   string
	OwnerSubject string
	// Label names the provider on the sign-in button.
	Label string
}

// OIDC is sign-in with one provider.
type OIDC struct {
	s      OIDCSettings
	roles  groupRoles
	client *http.Client
	now    func() time.Time

	mu      sync.Mutex
	disc    *oidcDiscovery
	discAt  time.Time
	keys    map[string]crypto.PublicKey
	keysAt  time.Time
	pending map[string]oidcPending
}

type oidcDiscovery struct {
	Issuer           string   `json:"issuer"`
	AuthEndpoint     string   `json:"authorization_endpoint"`
	TokenEndpoint    string   `json:"token_endpoint"`
	JWKSURI          string   `json:"jwks_uri"`
	TokenAuthMethods []string `json:"token_endpoint_auth_methods_supported"`
}

type oidcPending struct {
	nonce, verifier, redirect, returnTo string
	expires                             time.Time
}

// oidcPendingTTL is how long someone has to sign in at the provider.
const oidcPendingTTL = 10 * time.Minute

// NewOIDC is sign-in with the provider at s.Issuer, or nil when none is
// set. Settings that can't work are an error.
func NewOIDC(s OIDCSettings) (*OIDC, error) {
	s.Issuer = strings.TrimRight(strings.TrimSpace(s.Issuer), "/")
	if s.Issuer == "" {
		return nil, nil
	}
	u, err := url.Parse(s.Issuer)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return nil, errors.New("oidc.issuer must be an http or https address")
	}
	if strings.TrimSpace(s.ClientID) == "" {
		return nil, errors.New("oidc.client_id is required")
	}
	if s.RedirectURL != "" {
		if r, err := url.Parse(s.RedirectURL); err != nil || r.Host == "" || (r.Scheme != "https" && r.Scheme != "http") {
			return nil, errors.New("oidc.redirect_url must be an http or https address")
		}
	}
	roles, err := newGroupRoles(s.AdminGroups, s.MemberGroups, s.DefaultRole)
	if err != nil {
		return nil, errors.New("oidc." + err.Error())
	}
	if s.GroupsClaim == "" {
		s.GroupsClaim = "groups"
	}
	if len(s.Scopes) == 0 {
		s.Scopes = []string{"openid", "email", "profile"}
	}
	if !hasString(s.Scopes, "openid") {
		s.Scopes = append([]string{"openid"}, s.Scopes...)
	}
	if s.Label == "" {
		s.Label = u.Hostname()
	}
	return &OIDC{s: s, roles: roles, client: &http.Client{Timeout: 15 * time.Second}, now: time.Now, pending: map[string]oidcPending{}}, nil
}

func hasString(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// Label names the provider on the sign-in button.
func (o *OIDC) Label() string { return o.s.Label }

// RedirectURL is the callback to use for a request reaching Toskar at
// base, such as https://toskar.example.com.
func (o *OIDC) RedirectURL(base string) string {
	if o.s.RedirectURL != "" {
		return o.s.RedirectURL
	}
	return strings.TrimRight(base, "/") + "/api/v1/oidc/callback"
}

// SetHTTPClient replaces the client used to reach the provider, for tests.
func (o *OIDC) SetHTTPClient(c *http.Client) { o.client = c }

func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Start is the provider's address to send someone to, who comes back to
// redirect and then goes on to returnTo, a path in Toskar.
func (o *OIDC) Start(ctx context.Context, redirect, returnTo string) (string, error) {
	d, err := o.discovery(ctx)
	if err != nil {
		return "", err
	}
	state, nonce, verifier := randomToken(), randomToken(), randomToken()
	sum := sha256.Sum256([]byte(verifier))
	now := o.now()
	o.mu.Lock()
	for k, p := range o.pending {
		if now.After(p.expires) {
			delete(o.pending, k)
		}
	}
	if len(o.pending) >= 1000 {
		o.mu.Unlock()
		return "", errors.New("too many sign-ins waiting; try again in a few minutes")
	}
	o.pending[state] = oidcPending{nonce: nonce, verifier: verifier, redirect: redirect, returnTo: returnTo, expires: now.Add(oidcPendingTTL)}
	o.mu.Unlock()
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {o.s.ClientID},
		"redirect_uri":          {redirect},
		"scope":                 {strings.Join(o.s.Scopes, " ")},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {"S256"},
	}
	sep := "?"
	if strings.Contains(d.AuthEndpoint, "?") {
		sep = "&"
	}
	return d.AuthEndpoint + sep + q.Encode(), nil
}

// OIDCIdentity is who the provider signed in.
type OIDCIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	Groups        []string
}

// Finish takes the provider's answer, state and code, and says who signed
// in and where in Toskar they were going.
func (o *OIDC) Finish(ctx context.Context, state, code string) (OIDCIdentity, string, error) {
	o.mu.Lock()
	p, ok := o.pending[state]
	delete(o.pending, state)
	o.mu.Unlock()
	if !ok || state == "" || o.now().After(p.expires) {
		return OIDCIdentity{}, "", errors.New("this sign-in expired or was already used; start again")
	}
	if code == "" {
		return OIDCIdentity{}, p.returnTo, errors.New("the provider sent no code")
	}
	raw, err := o.exchange(ctx, code, p)
	if err != nil {
		return OIDCIdentity{}, p.returnTo, err
	}
	id, err := o.verify(ctx, raw, p.nonce)
	return id, p.returnTo, err
}

func (o *OIDC) exchange(ctx context.Context, code string, p oidcPending) (string, error) {
	d, err := o.discovery(ctx)
	if err != nil {
		return "", err
	}
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {p.redirect},
		"code_verifier": {p.verifier},
	}
	basic := o.s.ClientSecret != "" && (len(d.TokenAuthMethods) == 0 || hasString(d.TokenAuthMethods, "client_secret_basic"))
	if !basic {
		form.Set("client_id", o.s.ClientID)
		if o.s.ClientSecret != "" {
			form.Set("client_secret", o.s.ClientSecret)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if basic {
		req.SetBasicAuth(url.QueryEscape(o.s.ClientID), url.QueryEscape(o.s.ClientSecret))
	}
	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("reach the provider: %w", err)
	}
	defer resp.Body.Close()
	var body struct {
		IDToken string `json:"id_token"`
		Error   string `json:"error"`
		Desc    string `json:"error_description"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return "", fmt.Errorf("the provider's answer: %w", err)
	}
	if resp.StatusCode != http.StatusOK || body.IDToken == "" {
		if body.Error != "" {
			return "", fmt.Errorf("the provider refused: %s %s", body.Error, body.Desc)
		}
		return "", fmt.Errorf("the provider answered %d without an ID token", resp.StatusCode)
	}
	return body.IDToken, nil
}

// verify checks an ID token and says who it names.
func (o *OIDC) verify(ctx context.Context, raw, nonce string) (OIDCIdentity, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return OIDCIdentity{}, errors.New("the ID token isn't a JWT")
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := decodeSegment(parts[0], &header); err != nil {
		return OIDCIdentity{}, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return OIDCIdentity{}, errors.New("the ID token's signature isn't base64url")
	}
	key, err := o.key(ctx, header.Kid)
	if err != nil {
		return OIDCIdentity{}, err
	}
	if err := verifyJWS(header.Alg, key, []byte(parts[0]+"."+parts[1]), sig); err != nil {
		return OIDCIdentity{}, err
	}
	var claims map[string]any
	if err := decodeSegment(parts[1], &claims); err != nil {
		return OIDCIdentity{}, err
	}
	d, err := o.discovery(ctx)
	if err != nil {
		return OIDCIdentity{}, err
	}
	if iss, _ := claims["iss"].(string); strings.TrimRight(iss, "/") != strings.TrimRight(d.Issuer, "/") {
		return OIDCIdentity{}, fmt.Errorf("the ID token is from %q, not %q", iss, d.Issuer)
	}
	auds := stringList(claims["aud"])
	if !hasString(auds, o.s.ClientID) {
		return OIDCIdentity{}, errors.New("the ID token isn't for this Toskar")
	}
	if azp, ok := claims["azp"].(string); ok && azp != o.s.ClientID {
		return OIDCIdentity{}, errors.New("the ID token was issued to another client")
	}
	now := o.now()
	exp, ok := claims["exp"].(float64)
	if !ok || now.After(time.Unix(int64(exp), 0).Add(time.Minute)) {
		return OIDCIdentity{}, errors.New("the ID token has expired")
	}
	if iat, ok := claims["iat"].(float64); ok && time.Unix(int64(iat), 0).After(now.Add(5*time.Minute)) {
		return OIDCIdentity{}, errors.New("the ID token is from the future; check this computer's clock")
	}
	if got, _ := claims["nonce"].(string); subtle.ConstantTimeCompare([]byte(got), []byte(nonce)) != 1 {
		return OIDCIdentity{}, errors.New("the ID token isn't for this sign-in")
	}
	id := OIDCIdentity{Groups: stringList(claims[o.s.GroupsClaim])}
	id.Subject, _ = claims["sub"].(string)
	if id.Subject == "" {
		return OIDCIdentity{}, errors.New("the ID token names nobody")
	}
	id.Email, _ = claims["email"].(string)
	switch v := claims["email_verified"].(type) {
	case bool:
		id.EmailVerified = v
	case string:
		id.EmailVerified = v == "true"
	}
	id.Name, _ = claims["name"].(string)
	if id.Name == "" {
		id.Name, _ = claims["preferred_username"].(string)
	}
	return id, nil
}

func decodeSegment(seg string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(seg)
	if err != nil {
		return errors.New("the ID token isn't base64url")
	}
	if err := json.Unmarshal(b, v); err != nil {
		return errors.New("the ID token isn't JSON")
	}
	return nil
}

func stringList(v any) []string {
	switch x := v.(type) {
	case string:
		if x == "" {
			return nil
		}
		return []string{x}
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			if s, ok := e.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func verifyJWS(alg string, key crypto.PublicKey, signed, sig []byte) error {
	var h crypto.Hash
	switch alg {
	case "RS256", "ES256":
		h = crypto.SHA256
	case "RS384", "ES384":
		h = crypto.SHA384
	case "RS512", "ES512":
		h = crypto.SHA512
	default:
		return fmt.Errorf("the ID token is signed with %q, which Toskar doesn't accept", alg)
	}
	hasher := h.New()
	hasher.Write(signed)
	digest := hasher.Sum(nil)
	switch k := key.(type) {
	case *rsa.PublicKey:
		if !strings.HasPrefix(alg, "RS") {
			break
		}
		if rsa.VerifyPKCS1v15(k, h, digest, sig) != nil {
			return errors.New("the ID token's signature doesn't match the provider's key")
		}
		return nil
	case *ecdsa.PublicKey:
		if !strings.HasPrefix(alg, "ES") {
			break
		}
		size := (k.Curve.Params().BitSize + 7) / 8
		if len(sig) != 2*size {
			return errors.New("the ID token's signature has the wrong length")
		}
		r, s := new(big.Int).SetBytes(sig[:size]), new(big.Int).SetBytes(sig[size:])
		if !ecdsa.Verify(k, digest, r, s) {
			return errors.New("the ID token's signature doesn't match the provider's key")
		}
		return nil
	}
	return fmt.Errorf("the provider's key doesn't fit %q", alg)
}

// discovery is the provider's configuration, read once an hour.
func (o *OIDC) discovery(ctx context.Context) (*oidcDiscovery, error) {
	o.mu.Lock()
	if o.disc != nil && o.now().Sub(o.discAt) < time.Hour {
		d := o.disc
		o.mu.Unlock()
		return d, nil
	}
	o.mu.Unlock()
	var d oidcDiscovery
	if err := o.getJSON(ctx, o.s.Issuer+"/.well-known/openid-configuration", &d); err != nil {
		return nil, fmt.Errorf("read the provider's configuration: %w", err)
	}
	if strings.TrimRight(d.Issuer, "/") != o.s.Issuer {
		return nil, fmt.Errorf("the provider says it is %q, not %q", d.Issuer, o.s.Issuer)
	}
	if d.AuthEndpoint == "" || d.TokenEndpoint == "" || d.JWKSURI == "" {
		return nil, errors.New("the provider's configuration is missing its endpoints")
	}
	o.mu.Lock()
	o.disc, o.discAt = &d, o.now()
	o.mu.Unlock()
	return &d, nil
}

// key is the provider's signing key kid, read again when it's unknown, at
// most once a minute.
func (o *OIDC) key(ctx context.Context, kid string) (crypto.PublicKey, error) {
	o.mu.Lock()
	k, ok := o.find(kid)
	stale := o.now().Sub(o.keysAt) > time.Minute
	o.mu.Unlock()
	if ok {
		return k, nil
	}
	if !stale && o.keys != nil {
		return nil, errors.New("the ID token is signed with a key the provider doesn't publish")
	}
	d, err := o.discovery(ctx)
	if err != nil {
		return nil, err
	}
	var set struct {
		Keys []jwk `json:"keys"`
	}
	if err := o.getJSON(ctx, d.JWKSURI, &set); err != nil {
		return nil, fmt.Errorf("read the provider's keys: %w", err)
	}
	keys := map[string]crypto.PublicKey{}
	for i, j := range set.Keys {
		if j.Use != "" && j.Use != "sig" {
			continue
		}
		pub, err := j.publicKey()
		if err != nil {
			continue
		}
		id := j.Kid
		if id == "" {
			id = fmt.Sprintf("#%d", i)
		}
		keys[id] = pub
	}
	o.mu.Lock()
	o.keys, o.keysAt = keys, o.now()
	k, ok = o.find(kid)
	o.mu.Unlock()
	if !ok {
		return nil, errors.New("the ID token is signed with a key the provider doesn't publish")
	}
	return k, nil
}

// find is key kid, or the only key when the token names none. o.mu is held.
func (o *OIDC) find(kid string) (crypto.PublicKey, bool) {
	if k, ok := o.keys[kid]; ok && kid != "" {
		return k, true
	}
	if kid == "" && len(o.keys) == 1 {
		for _, k := range o.keys {
			return k, true
		}
	}
	return nil, false
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func (j jwk) publicKey() (crypto.PublicKey, error) {
	num := func(s string) (*big.Int, error) {
		b, err := base64.RawURLEncoding.DecodeString(s)
		if err != nil || len(b) == 0 {
			return nil, errors.New("bad key number")
		}
		return new(big.Int).SetBytes(b), nil
	}
	switch j.Kty {
	case "RSA":
		n, err := num(j.N)
		if err != nil {
			return nil, err
		}
		e, err := num(j.E)
		if err != nil || !e.IsInt64() || e.Int64() < 3 {
			return nil, errors.New("bad RSA exponent")
		}
		if n.BitLen() < 2048 {
			return nil, errors.New("RSA key too short")
		}
		return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
	case "EC":
		var curve elliptic.Curve
		switch j.Crv {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve = elliptic.P384()
		case "P-521":
			curve = elliptic.P521()
		default:
			return nil, errors.New("unknown curve")
		}
		x, err := num(j.X)
		if err != nil {
			return nil, err
		}
		y, err := num(j.Y)
		if err != nil {
			return nil, err
		}
		pub := &ecdsa.PublicKey{Curve: curve, X: x, Y: y}
		if !curve.IsOnCurve(x, y) { //nolint:staticcheck // checking a published key
			return nil, errors.New("point not on curve")
		}
		return pub, nil
	}
	return nil, errors.New("unknown key type")
}

func (o *OIDC) getJSON(ctx context.Context, address string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s answered %d", address, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(v)
}

// SignedInWith is the person the provider signed in as id: the Owner, or the
// person with that subject, made the first time with the role their groups
// give. A disabled person, or one no role lets in, is refused.
func (p *People) SignedInWith(ctx context.Context, o *OIDC, id OIDCIdentity) (Person, error) {
	if (o.s.OwnerSubject != "" && id.Subject == o.s.OwnerSubject) ||
		(o.s.OwnerEmail != "" && id.EmailVerified && strings.EqualFold(id.Email, o.s.OwnerEmail)) {
		return p.Active(ctx, OwnerID)
	}
	role, ok := o.roles.role(id.Groups)
	if !ok {
		return Person{}, ErrOIDCRefused
	}
	label := id.Email
	if label == "" {
		label = id.Name
	}
	if label == "" {
		label = id.Subject
	}
	name := id.Name
	if name == "" && id.Email != "" {
		name = id.Email
		if at := strings.IndexByte(name, '@'); at > 0 {
			name = name[:at]
		}
	}
	return p.External(ctx, ExternalSignIn{ID: "oidc:" + id.Subject, Label: label, Name: name, Role: role})
}
