// Package join adds a computer to a Yggdrasil network with one command
// (#40): a short-lived, one-time join token made on a computer already in
// the network, and a handshake that turns it into normal Bifrost trust.
//
// Bifrost speaks plain HTTP, so the handshake protects itself: the command
// carries the issuing computer's key fingerprint, the issuer signs a fresh
// challenge with that key, and the joining computer proves it holds the
// token with an HMAC bound to the challenge and to its own public key. The
// token's secret never crosses the network.
package join

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// TokenPrefix starts every join token.
const TokenPrefix = "ygj_"

// Token defaults.
const (
	DefaultTTL = 15 * time.Minute
	MaxTTL     = 24 * time.Hour
	// keepRecords is how long used, revoked, and expired tokens are listed.
	keepRecords = 7 * 24 * time.Hour
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// Errors about tokens. ErrInvalid never says whether a token exists.
var (
	ErrInvalid  = errors.New("this join token isn't valid")
	ErrExpired  = errors.New("this join token has expired")
	ErrUsed     = errors.New("this join token has already been used")
	ErrRevoked  = errors.New("this join token was revoked")
	ErrNotFound = errors.New("no join token has that ID")
)

// Token is a join token's record; the secret is never kept.
type Token struct {
	ID        string     `json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	// UsedBy is the name of the computer that joined with it.
	UsedBy    string     `json:"used_by,omitempty"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	// Status is active, used, revoked, or expired.
	Status string `json:"status"`
}

func (t *Token) setStatus(now time.Time) {
	switch {
	case t.UsedAt != nil:
		t.Status = "used"
	case t.RevokedAt != nil:
		t.Status = "revoked"
	case !now.Before(t.ExpiresAt):
		t.Status = "expired"
	default:
		t.Status = "active"
	}
}

// err is why the token can no longer be used, or nil.
func (t *Token) err() error {
	switch t.Status {
	case "used":
		return ErrUsed
	case "revoked":
		return ErrRevoked
	case "expired":
		return ErrExpired
	}
	return nil
}

// Tokens keeps join tokens.
type Tokens struct {
	DB  *sql.DB
	Now func() time.Time
}

func (s *Tokens) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func randomB32(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.ToLower(b32.EncodeToString(b)), nil
}

// Create makes a token that can be used once within ttl, and returns it;
// it is never shown again.
func (s *Tokens) Create(ctx context.Context, ttl time.Duration) (string, Token, error) {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	if ttl > MaxTTL {
		return "", Token{}, fmt.Errorf("a join token lasts at most %s", MaxTTL)
	}
	id, err := randomB32(5) // 8 characters
	if err != nil {
		return "", Token{}, err
	}
	secret, err := randomB32(20) // 32 characters, 160 bits
	if err != nil {
		return "", Token{}, err
	}
	now := s.now()
	t := Token{ID: id, CreatedAt: now, ExpiresAt: now.Add(ttl)}
	t.setStatus(now)
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO join_tokens (id, proof_key, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		id, hex.EncodeToString(ProofKey(secret)), now.Format(time.RFC3339Nano), t.ExpiresAt.Format(time.RFC3339Nano)); err != nil {
		return "", Token{}, err
	}
	return TokenPrefix + id + "_" + secret, t, nil
}

// ParseToken splits a token into its ID and secret.
func ParseToken(raw string) (id, secret string, err error) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(raw), TokenPrefix)
	if !ok {
		return "", "", errors.New("a join token starts with " + TokenPrefix)
	}
	id, secret, ok = strings.Cut(rest, "_")
	if !ok || len(id) != 8 || len(secret) != 32 {
		return "", "", errors.New("this doesn't look like a whole join token; copy the command again")
	}
	if _, err := b32.DecodeString(strings.ToUpper(id + secret)); err != nil {
		return "", "", errors.New("this doesn't look like a whole join token; copy the command again")
	}
	return id, secret, nil
}

// ProofKey is derived from a token's secret: what both computers key the
// handshake's proofs with.
func ProofKey(secret string) []byte {
	m := hmac.New(sha256.New, []byte("yggdrasil join token v1"))
	m.Write([]byte(secret))
	return m.Sum(nil)
}

func parseTime(s sql.NullString) *time.Time {
	if !s.Valid {
		return nil
	}
	t, err := time.Parse(time.RFC3339Nano, s.String)
	if err != nil {
		return nil
	}
	return &t
}

func (s *Tokens) get(ctx context.Context, id string) (Token, []byte, error) {
	var t Token
	var key, created, expires string
	var used, usedBy, revoked sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT id, proof_key, created_at, expires_at, used_at, used_by, revoked_at FROM join_tokens WHERE id = ?`, id).
		Scan(&t.ID, &key, &created, &expires, &used, &usedBy, &revoked)
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, nil, ErrNotFound
	}
	if err != nil {
		return Token{}, nil, err
	}
	t.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	t.ExpiresAt, _ = time.Parse(time.RFC3339Nano, expires)
	t.UsedAt, t.UsedBy, t.RevokedAt = parseTime(used), usedBy.String, parseTime(revoked)
	t.setStatus(s.now())
	k, err := hex.DecodeString(key)
	if err != nil {
		return Token{}, nil, err
	}
	return t, k, nil
}

// List returns active tokens and those used, revoked, or expired in the
// last week, newest first; older records are removed.
func (s *Tokens) List(ctx context.Context) ([]Token, error) {
	now := s.now()
	_, _ = s.DB.ExecContext(ctx, `DELETE FROM join_tokens WHERE expires_at < ?`, now.Add(-keepRecords).Format(time.RFC3339Nano))
	rows, err := s.DB.QueryContext(ctx, `SELECT id FROM join_tokens ORDER BY created_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	_ = rows.Close()
	out := []Token{}
	for _, id := range ids {
		t, _, err := s.get(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// Revoke stops an unused token from being used.
func (s *Tokens) Revoke(ctx context.Context, id string) (Token, error) {
	t, _, err := s.get(ctx, id)
	if err != nil {
		return Token{}, err
	}
	if t.Status != "active" {
		return t, t.err()
	}
	if _, err := s.DB.ExecContext(ctx, `UPDATE join_tokens SET revoked_at = ? WHERE id = ? AND used_at IS NULL`, s.now().Format(time.RFC3339Nano), id); err != nil {
		return Token{}, err
	}
	t, _, err = s.get(ctx, id)
	return t, err
}

// consume marks a token used by a computer, once: a second use, even at the
// same moment, fails.
func (s *Tokens) consume(ctx context.Context, id, usedBy string) error {
	now := s.now()
	res, err := s.DB.ExecContext(ctx, `
		UPDATE join_tokens SET used_at = ?, used_by = ?
		WHERE id = ? AND used_at IS NULL AND revoked_at IS NULL AND expires_at > ?`,
		now.Format(time.RFC3339Nano), usedBy, id, now.Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t, _, err := s.get(ctx, id)
		if err != nil {
			return ErrInvalid
		}
		if e := t.err(); e != nil {
			return e
		}
		return ErrInvalid
	}
	return nil
}
