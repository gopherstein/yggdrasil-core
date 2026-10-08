package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Invite kinds: a first sign-in, or a new password.
const (
	InviteNew   = "invite"
	InviteReset = "reset"
)

// Invite lifetimes: a week to accept an invitation, a day for a new
// password.
const (
	inviteTTL = 7 * 24 * time.Hour
	resetTTL  = 24 * time.Hour
)

// ErrNoInvite is a link that isn't, or is no longer, good: unknown, used,
// or expired.
var ErrNoInvite = errors.New("this link has expired or was already used; ask for a new one")

// Invites are one-time links that let a person set up their sign-in (#206).
// Only a hash of each token is kept.
type Invites struct {
	db  *sql.DB
	now func() time.Time
}

// NewInvites keeps invites in db.
func NewInvites(db *sql.DB) *Invites { return &Invites{db: db, now: time.Now} }

// Create makes a link for person, replacing any unused one, and returns
// its token.
func (i *Invites) Create(ctx context.Context, personID, kind, createdBy string) (string, time.Time, error) {
	if kind != InviteNew && kind != InviteReset {
		return "", time.Time{}, errors.New("unknown invite kind")
	}
	token, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	now := i.now().UTC()
	ttl := inviteTTL
	if kind == InviteReset {
		ttl = resetTTL
	}
	expires := now.Add(ttl)
	tx, err := i.db.BeginTx(ctx, nil)
	if err != nil {
		return "", time.Time{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM invites WHERE person_id = ? AND used_at IS NULL`, personID); err != nil {
		return "", time.Time{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO invites (token_hash, person_id, kind, created_by, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?)`, secretHash(token), personID, kind, createdBy, stamp(now), stamp(expires)); err != nil {
		return "", time.Time{}, err
	}
	return token, expires, tx.Commit()
}

// Peek is whose a link is and its kind, while it's good.
func (i *Invites) Peek(ctx context.Context, token string) (personID, kind string, err error) {
	var expires string
	err = i.db.QueryRowContext(ctx, `SELECT person_id, kind, expires_at FROM invites WHERE token_hash = ? AND used_at IS NULL`,
		secretHash(token)).Scan(&personID, &kind, &expires)
	if err != nil || !i.now().UTC().Before(parseTime(expires)) {
		return "", "", ErrNoInvite
	}
	return personID, kind, nil
}

// Use spends a link, once: two uses at the same moment can't both succeed.
func (i *Invites) Use(ctx context.Context, token string) (personID, kind string, err error) {
	personID, kind, err = i.Peek(ctx, token)
	if err != nil {
		return "", "", err
	}
	now := stamp(i.now())
	res, err := i.db.ExecContext(ctx, `UPDATE invites SET used_at = ? WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?`,
		now, secretHash(token), now)
	if err != nil {
		return "", "", err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return "", "", ErrNoInvite
	}
	return personID, kind, nil
}
