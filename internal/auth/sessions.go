package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

// SessionCookie is the cookie a signed-in browser carries (#206).
const SessionCookie = "toskar_session"

// Session lifetimes: a session lasts 30 days from its last use, and its
// last use is written at most hourly.
const (
	SessionTTL   = 30 * 24 * time.Hour
	sessionTouch = time.Hour
)

// ErrNoSession is a cookie that isn't, or is no longer, a session.
var ErrNoSession = errors.New("not signed in")

// Sessions are browsers signed in as a person. Only a hash of each cookie
// is kept, so the database can't sign anyone in.
type Sessions struct {
	db  *sql.DB
	now func() time.Time
}

// NewSessions keeps sessions in db.
func NewSessions(db *sql.DB) *Sessions { return &Sessions{db: db, now: time.Now} }

func secretHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// Create signs a browser in as person and returns its cookie value.
func (s *Sessions) Create(ctx context.Context, personID, userAgent, address string) (string, time.Time, error) {
	token, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	now := s.now().UTC()
	expires := now.Add(SessionTTL)
	if len(userAgent) > 200 {
		userAgent = userAgent[:200]
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO sessions (id_hash, person_id, created_at, last_seen_at, expires_at, user_agent, address)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, secretHash(token), personID, stamp(now), stamp(now), stamp(expires), userAgent, address)
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

// Person is who a session's cookie signs in, while it hasn't expired. Use
// keeps it alive.
func (s *Sessions) Person(ctx context.Context, token string) (string, error) {
	if token == "" {
		return "", ErrNoSession
	}
	var person, lastSeen, expires string
	err := s.db.QueryRowContext(ctx, `SELECT person_id, last_seen_at, expires_at FROM sessions WHERE id_hash = ?`, secretHash(token)).
		Scan(&person, &lastSeen, &expires)
	if err != nil {
		return "", ErrNoSession
	}
	now := s.now().UTC()
	if !now.Before(parseTime(expires)) {
		_, _ = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id_hash = ?`, secretHash(token))
		return "", ErrNoSession
	}
	if now.Sub(parseTime(lastSeen)) > sessionTouch {
		_, _ = s.db.ExecContext(ctx, `UPDATE sessions SET last_seen_at = ?, expires_at = ? WHERE id_hash = ?`,
			stamp(now), stamp(now.Add(SessionTTL)), secretHash(token))
	}
	return person, nil
}

// Delete signs a browser out.
func (s *Sessions) Delete(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id_hash = ?`, secretHash(token))
	return err
}

// DeleteFor signs a person out everywhere, such as when they're disabled
// or get a new password.
func (s *Sessions) DeleteFor(ctx context.Context, personID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE person_id = ?`, personID)
	return err
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
