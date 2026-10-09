// Package topiclog keeps the messages an Enforce profile held, or whose
// answer it replaced (#345), so an Admin sees who tries to take it off
// topic, and when the topic is set too narrowly. They are run records:
// kept and deleted with them.
package topiclog

import (
	"context"
	"database/sql"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Labels an attempt can have.
const (
	// Held is an off-topic message answered with the set reply.
	Held = "off_topic"
	// Replaced is an answer that went off topic and was replaced.
	Replaced = "answer_off_topic"
)

// MaxMessage is the most of a message kept, in characters.
const MaxMessage = 500

// Attempt is one off-topic message.
type Attempt struct {
	ID             string    `json:"id"`
	At             time.Time `json:"at"`
	ProfileID      string    `json:"profile_id"`
	Label          string    `json:"label"`
	Source         string    `json:"source,omitempty"`
	PortalID       string    `json:"portal_id,omitempty"`
	KeyID          string    `json:"key_id,omitempty"`
	PersonID       string    `json:"person_id,omitempty"`
	ConversationID string    `json:"conversation_id,omitempty"`
	Message        string    `json:"message"`
}

// Store keeps attempts.
type Store struct {
	db  *sql.DB
	now func() time.Time
}

// New keeps attempts in db.
func New(db *sql.DB) *Store { return &Store{db: db, now: time.Now} }

func clip(s string) string {
	if utf8.RuneCountInString(s) <= MaxMessage {
		return s
	}
	return string([]rune(s)[:MaxMessage]) + "…"
}

// Add records an attempt.
func (s *Store) Add(ctx context.Context, a Attempt) error {
	if s == nil || s.db == nil {
		return nil
	}
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	if a.At.IsZero() {
		a.At = s.now()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO topic_attempts (id, at, profile_id, label, source, portal_id, key_id, person_id, conversation_id, message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.At.UTC().Format(time.RFC3339Nano), a.ProfileID, a.Label, a.Source, a.PortalID, a.KeyID, a.PersonID, a.ConversationID, clip(a.Message))
	return err
}

// List is a profile's attempts since a time, newest first.
func (s *Store) List(ctx context.Context, profileID string, since time.Time, limit int) ([]Attempt, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, at, profile_id, label, source, portal_id, key_id, person_id, conversation_id, message
		FROM topic_attempts WHERE profile_id = ? AND at >= ? ORDER BY at DESC LIMIT ?`,
		profileID, since.UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Attempt{}
	for rows.Next() {
		var a Attempt
		var at string
		if err := rows.Scan(&a.ID, &at, &a.ProfileID, &a.Label, &a.Source, &a.PortalID, &a.KeyID, &a.PersonID, &a.ConversationID, &a.Message); err != nil {
			return nil, err
		}
		a.At, _ = time.Parse(time.RFC3339Nano, at)
		out = append(out, a)
	}
	return out, rows.Err()
}

// Get is one attempt.
func (s *Store) Get(ctx context.Context, id string) (Attempt, error) {
	var a Attempt
	var at string
	err := s.db.QueryRowContext(ctx, `SELECT id, at, profile_id, label, source, portal_id, key_id, person_id, conversation_id, message
		FROM topic_attempts WHERE id = ?`, id).Scan(&a.ID, &at, &a.ProfileID, &a.Label, &a.Source, &a.PortalID, &a.KeyID, &a.PersonID, &a.ConversationID, &a.Message)
	a.At, _ = time.Parse(time.RFC3339Nano, at)
	return a, err
}

// ForgetMessage removes a profile's attempts with this message, once it
// is marked as on topic.
func (s *Store) ForgetMessage(ctx context.Context, profileID, message string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM topic_attempts WHERE profile_id = ? AND message = ?`, profileID, message)
	return err
}
