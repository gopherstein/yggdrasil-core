package join

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Limits on the handshake.
const (
	challengeTTL  = 2 * time.Minute
	maxChallenges = 1000
	// A source address may start this many handshakes, and fail this many
	// proofs, per window.
	helloLimit = 30
	failLimit  = 10
	limitWin   = 10 * time.Minute
	maxBody    = 64 << 10
)

// Acceptor answers join handshakes on the issuing computer.
type Acceptor struct {
	Tokens *Tokens
	// NodeID, Name, and Key are this computer's identity.
	NodeID string
	Name   func() string
	Key    ed25519.PrivateKey
	// NetworkID is this computer's network, made when the first token is.
	NetworkID func(ctx context.Context) (string, error)
	// Address is where this computer's Bifrost can be reached.
	Address func() string
	// Admit trusts a joining computer and returns the name it is known by.
	Admit func(ctx context.Context, node JoiningNode) (string, error)
	// Joined hears each computer that joined, for records and notices.
	Joined func(ctx context.Context, node JoiningNode, name, tokenID, source string)
	// Refused hears each refused join, without the token.
	Refused func(ctx context.Context, reason, tokenID, source string)
	Logger  *slog.Logger
	Now     func() time.Time

	mu         sync.Mutex
	challenges map[string]challenge
	hits       map[string]*window
}

type challenge struct {
	tokenID, clientNonce string
	at                   time.Time
}

type window struct {
	start         time.Time
	hellos, fails int
}

func (a *Acceptor) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

func nonce() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// source is the request's IP address.
func source(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// count records a hello or a failed proof from a source and reports
// whether it is within the limit.
func (a *Acceptor) count(src string, fail bool) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.hits == nil {
		a.hits = map[string]*window{}
	}
	now := a.now()
	w := a.hits[src]
	if w == nil || now.Sub(w.start) > limitWin {
		w = &window{start: now}
		a.hits[src] = w
		if len(a.hits) > 10000 {
			for k, v := range a.hits {
				if now.Sub(v.start) > limitWin {
					delete(a.hits, k)
				}
			}
		}
	}
	if fail {
		w.fails++
		return w.fails <= failLimit
	}
	w.hellos++
	return w.hellos <= helloLimit && w.fails < failLimit
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func refuse(w http.ResponseWriter, status int, code string, err error) {
	writeJSON(w, status, ErrorResponse{Code: code, Message: err.Error()})
}

// ErrRateLimited is too many attempts from one address.
var ErrRateLimited = errors.New("too many join attempts from this address; wait a few minutes and try again")

// Hello answers the first step: who this computer is, signed, and a
// challenge.
func (a *Acceptor) Hello(w http.ResponseWriter, r *http.Request) {
	src := source(r)
	if !a.count(src, false) {
		refuse(w, http.StatusTooManyRequests, "JOIN_RATE_LIMITED", ErrRateLimited)
		return
	}
	var req HelloRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody)).Decode(&req); err != nil || req.TokenID == "" || len(req.ClientNonce) < 32 || len(req.ClientNonce) > 128 {
		refuse(w, http.StatusBadRequest, "JOIN_BAD_REQUEST", errors.New("the join request is malformed"))
		return
	}
	network, err := a.NetworkID(r.Context())
	if err != nil {
		refuse(w, http.StatusInternalServerError, "JOIN_FAILED", errors.New("this computer could not answer the join"))
		return
	}
	sn, err := nonce()
	if err != nil {
		refuse(w, http.StatusInternalServerError, "JOIN_FAILED", errors.New("this computer could not answer the join"))
		return
	}
	a.mu.Lock()
	if a.challenges == nil {
		a.challenges = map[string]challenge{}
	}
	now := a.now()
	if len(a.challenges) >= maxChallenges {
		for k, c := range a.challenges {
			if now.Sub(c.at) > challengeTTL {
				delete(a.challenges, k)
			}
		}
	}
	full := len(a.challenges) >= maxChallenges
	if !full {
		a.challenges[sn] = challenge{tokenID: req.TokenID, clientNonce: req.ClientNonce, at: now}
	}
	a.mu.Unlock()
	if full {
		refuse(w, http.StatusTooManyRequests, "JOIN_RATE_LIMITED", ErrRateLimited)
		return
	}
	h := HelloResponse{NodeID: a.NodeID, Name: a.Name(), PublicKey: base64.StdEncoding.EncodeToString(a.Key.Public().(ed25519.PublicKey)),
		NetworkID: network, ServerNonce: sn}
	h.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(a.Key, h.message(req.TokenID, req.ClientNonce)))
	writeJSON(w, http.StatusOK, h)
}

// take uses up a challenge.
func (a *Acceptor) take(serverNonce string) (challenge, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	c, ok := a.challenges[serverNonce]
	delete(a.challenges, serverNonce)
	if !ok || a.now().Sub(c.at) > challengeTTL {
		return challenge{}, false
	}
	return c, true
}

func (a *Acceptor) refused(ctx context.Context, reason, tokenID, src string) {
	if a.Refused != nil {
		a.Refused(ctx, reason, tokenID, src)
	}
}

// Join checks the proof, uses the token up, and trusts the new computer.
func (a *Acceptor) Join(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	src := source(r)
	var req JoinRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody)).Decode(&req); err != nil {
		refuse(w, http.StatusBadRequest, "JOIN_BAD_REQUEST", errors.New("the join request is malformed"))
		return
	}
	invalid := func(reason string) {
		a.refused(ctx, reason, req.TokenID, src)
		if !a.count(src, true) {
			refuse(w, http.StatusTooManyRequests, "JOIN_RATE_LIMITED", ErrRateLimited)
			return
		}
		refuse(w, http.StatusUnauthorized, "JOIN_TOKEN_INVALID", ErrInvalid)
	}
	c, ok := a.take(req.ServerNonce)
	if !ok || c.tokenID != req.TokenID || c.clientNonce != req.ClientNonce {
		invalid("the challenge is unknown or expired")
		return
	}
	if _, err := PublicKeyFromPEM(req.Node.PublicKeyPEM); err != nil || req.Node.ID == "" || req.Node.ID == a.NodeID ||
		strings.TrimSpace(req.Node.Name) == "" || len(req.Node.Name) > 100 || len(req.Node.Address) > 300 {
		refuse(w, http.StatusBadRequest, "JOIN_BAD_REQUEST", errors.New("the joining computer's identity is malformed"))
		return
	}
	t, key, err := a.Tokens.get(ctx, req.TokenID)
	if err != nil {
		invalid("no such token")
		return
	}
	if !checkProof(key, req, Fingerprint(a.Key.Public().(ed25519.PublicKey))) {
		invalid("the proof is wrong")
		return
	}
	// The proof shows the token itself, so saying why it can't be used now
	// gives nothing away.
	if err := t.err(); err != nil {
		a.refused(ctx, err.Error(), req.TokenID, src)
		refuse(w, http.StatusGone, tokenCode(err), err)
		return
	}
	// Use the token up before trusting anyone, so two joins racing with it
	// cannot both get in.
	if err := a.Tokens.consume(ctx, req.TokenID, req.Node.Name); err != nil {
		a.refused(ctx, err.Error(), req.TokenID, src)
		refuse(w, http.StatusGone, tokenCode(err), err)
		return
	}
	name, err := a.Admit(ctx, req.Node)
	if err != nil {
		a.Logger.Error("join: trust the new computer", "err", err)
		refuse(w, http.StatusInternalServerError, "JOIN_FAILED", errors.New("this computer could not record the new computer; make a new join command and try again"))
		return
	}
	network, err := a.NetworkID(ctx)
	if err != nil {
		refuse(w, http.StatusInternalServerError, "JOIN_FAILED", errors.New("this computer could not answer the join"))
		return
	}
	pub := a.Key.Public().(ed25519.PublicKey)
	acc := Accepted{NetworkID: network, Name: name, ClientNonce: req.ClientNonce,
		Server: Peer{ID: a.NodeID, Name: a.Name(), Address: a.Address(), PublicKeyPEM: publicKeyPEM(pub)}}
	payload, _ := json.Marshal(acc)
	if a.Joined != nil {
		a.Joined(ctx, req.Node, name, req.TokenID, src)
	}
	writeJSON(w, http.StatusOK, JoinResponse{Payload: string(payload), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(a.Key, append([]byte("yggdrasil-join-accept-v1\n"), payload...)))})
}

func tokenCode(err error) string {
	switch {
	case errors.Is(err, ErrExpired):
		return "JOIN_TOKEN_EXPIRED"
	case errors.Is(err, ErrUsed):
		return "JOIN_TOKEN_USED"
	case errors.Is(err, ErrRevoked):
		return "JOIN_TOKEN_REVOKED"
	}
	return "JOIN_TOKEN_INVALID"
}
