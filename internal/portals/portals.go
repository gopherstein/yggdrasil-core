// Package portals keeps chat portals (#205): branded chat pages an Admin
// publishes for the people they serve, each answering with its own
// profile, tools, memory setting, and access.
package portals

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yeixio/toskar-core/internal/auth"
)

// Tool levels a portal allows.
const (
	ToolsNone     = "none"
	ToolsReadOnly = "read_only"
	ToolsProfile  = "profile"
)

// A new portal's limits.
const (
	DefaultHourlyLimit = 30
	DefaultMaxMessage  = 2000
	DefaultConcurrency = 2
)

// Access is who may chat in a portal.
const (
	// AccessOpen is anyone who can reach this Toskar.
	AccessOpen = "open"
	// AccessPasscode is anyone with the portal's shared passcode.
	AccessPasscode = "passcode"
)

// Portal is one chat portal.
type Portal struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
	// ProfileID answers its chats; empty is the default profile.
	ProfileID string `json:"profile_id"`
	// Tools is none, read_only, or profile (the profile's own).
	Tools string `json:"tools"`
	// Memory lets a visitor's chats keep and use memories of their own.
	Memory bool `json:"memory"`
	// Language answers in this language; empty follows the visitor.
	Language string `json:"language"`
	// Access is open or passcode.
	Access      string `json:"access"`
	HasPasscode bool   `json:"has_passcode"`
	// HourlyLimit is how many messages each visitor may send an hour;
	// 0 is no limit.
	HourlyLimit int `json:"hourly_limit"`
	// MaxMessage is the longest message, in characters.
	MaxMessage int `json:"max_message"`
	// Concurrency is how many of the portal's chats may run at once.
	Concurrency int `json:"concurrency"`
	// Branding is how the page looks, as the page reads it.
	Branding  json.RawMessage `json:"branding"`
	Enabled   bool            `json:"enabled"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`

	passcodeHash string
}

// Errors a change can have.
var (
	ErrNotFound      = errors.New("no such portal")
	ErrBadSlug       = errors.New("a portal's address is 2 to 40 lowercase letters, digits, or dashes")
	ErrSlugTaken     = errors.New("another portal has that address")
	ErrBadName       = errors.New("a portal's name is 1 to 80 characters")
	ErrBadTools      = errors.New("tools must be none, read_only, or profile")
	ErrBadAccess     = errors.New("access must be open or passcode")
	ErrNoPasscode    = errors.New("a passcode portal needs a passcode of at least 4 characters")
	ErrBadBranding   = errors.New("branding must be a JSON object under 16 KB")
	ErrBadLanguage   = errors.New("language must be a language tag such as en or pt-BR, or empty to follow the visitor")
	ErrWrongPasscode = errors.New("that passcode isn't right")
	ErrBadLimits     = errors.New("limits: 0 to 1000 messages an hour, 100 to 20000 characters a message, and 1 to 20 chats at once")
)

var (
	slugRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,38}[a-z0-9]$`)
	languageRe = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)
)

// Store keeps portals.
type Store struct {
	db  *sql.DB
	now func() time.Time
}

// NewStore keeps portals in db.
func NewStore(db *sql.DB) *Store { return &Store{db: db, now: time.Now} }

const columns = `id, slug, name, profile_id, tools, memory, language, access, COALESCE(passcode_hash, ''), branding, enabled, created_at, updated_at, hourly_limit, max_message, concurrency`

func scan(row interface{ Scan(...any) error }) (Portal, error) {
	var p Portal
	var branding, created, updated string
	if err := row.Scan(&p.ID, &p.Slug, &p.Name, &p.ProfileID, &p.Tools, &p.Memory, &p.Language, &p.Access, &p.passcodeHash, &branding, &p.Enabled, &created, &updated, &p.HourlyLimit, &p.MaxMessage, &p.Concurrency); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Portal{}, ErrNotFound
		}
		return Portal{}, err
	}
	p.Branding = json.RawMessage(branding)
	p.HasPasscode = p.passcodeHash != ""
	p.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	p.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return p, nil
}

// List is every portal, by name.
func (s *Store) List(ctx context.Context) ([]Portal, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+columns+` FROM portals ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Portal{}
	for rows.Next() {
		p, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Get is one portal by id.
func (s *Store) Get(ctx context.Context, id string) (Portal, error) {
	return scan(s.db.QueryRowContext(ctx, `SELECT `+columns+` FROM portals WHERE id = ?`, id))
}

// BySlug is one portal by its address.
func (s *Store) BySlug(ctx context.Context, slug string) (Portal, error) {
	return scan(s.db.QueryRowContext(ctx, `SELECT `+columns+` FROM portals WHERE slug = ?`, strings.ToLower(slug)))
}

// Input is a new portal, or a change to one; nil fields stay.
type Input struct {
	Slug      *string          `json:"slug,omitempty"`
	Name      *string          `json:"name,omitempty"`
	ProfileID *string          `json:"profile_id,omitempty"`
	Tools     *string          `json:"tools,omitempty"`
	Memory    *bool            `json:"memory,omitempty"`
	Language  *string          `json:"language,omitempty"`
	Access    *string          `json:"access,omitempty"`
	Passcode  *string          `json:"passcode,omitempty"`
	Branding  *json.RawMessage `json:"branding,omitempty"`
	Enabled   *bool            `json:"enabled,omitempty"`
	// Limits.
	HourlyLimit *int `json:"hourly_limit,omitempty"`
	MaxMessage  *int `json:"max_message,omitempty"`
	Concurrency *int `json:"concurrency,omitempty"`
}

// apply checks in and sets it on p.
func (in Input) apply(p *Portal) error {
	if in.Slug != nil {
		slug := strings.ToLower(strings.TrimSpace(*in.Slug))
		if !slugRe.MatchString(slug) {
			return ErrBadSlug
		}
		p.Slug = slug
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" || len([]rune(name)) > 80 {
			return ErrBadName
		}
		p.Name = name
	}
	if in.ProfileID != nil {
		p.ProfileID = strings.TrimSpace(*in.ProfileID)
	}
	if in.Tools != nil {
		switch *in.Tools {
		case ToolsNone, ToolsReadOnly, ToolsProfile:
			p.Tools = *in.Tools
		default:
			return ErrBadTools
		}
	}
	if in.Memory != nil {
		p.Memory = *in.Memory
	}
	if in.Language != nil {
		lang := strings.TrimSpace(*in.Language)
		if lang != "" && !languageRe.MatchString(lang) {
			return ErrBadLanguage
		}
		p.Language = lang
	}
	if in.Access != nil {
		switch *in.Access {
		case AccessOpen, AccessPasscode:
			p.Access = *in.Access
		default:
			return ErrBadAccess
		}
	}
	if in.Passcode != nil {
		code := strings.TrimSpace(*in.Passcode)
		switch {
		case code == "":
			p.passcodeHash = ""
		case len([]rune(code)) < 6 || len(code) > 200:
			return ErrNoPasscode
		default:
			hash, err := auth.HashSecret(code)
			if err != nil {
				return err
			}
			p.passcodeHash = hash
		}
	}
	if p.Access == AccessPasscode && p.passcodeHash == "" {
		return ErrNoPasscode
	}
	if in.Branding != nil {
		raw := []byte(*in.Branding)
		var obj map[string]any
		if len(raw) > 16<<10 || json.Unmarshal(raw, &obj) != nil || obj == nil {
			return ErrBadBranding
		}
		p.Branding = json.RawMessage(raw)
	}
	if in.Enabled != nil {
		p.Enabled = *in.Enabled
	}
	if in.HourlyLimit != nil {
		p.HourlyLimit = *in.HourlyLimit
	}
	if in.MaxMessage != nil {
		p.MaxMessage = *in.MaxMessage
	}
	if in.Concurrency != nil {
		p.Concurrency = *in.Concurrency
	}
	if p.HourlyLimit < 0 || p.HourlyLimit > 1000 || p.MaxMessage < 100 || p.MaxMessage > 20000 || p.Concurrency < 1 || p.Concurrency > 20 {
		return ErrBadLimits
	}
	p.HasPasscode = p.passcodeHash != ""
	return nil
}

// Create adds a portal; slug and name are required. It starts with no
// tools, no memory, passcode access, and on.
func (s *Store) Create(ctx context.Context, in Input) (Portal, error) {
	p := Portal{ID: uuid.NewString(), Tools: ToolsNone, Access: AccessPasscode, Branding: json.RawMessage(`{}`), Enabled: true,
		HourlyLimit: DefaultHourlyLimit, MaxMessage: DefaultMaxMessage, Concurrency: DefaultConcurrency}
	if in.Slug == nil {
		return Portal{}, ErrBadSlug
	}
	if in.Name == nil {
		return Portal{}, ErrBadName
	}
	if err := in.apply(&p); err != nil {
		return Portal{}, err
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT INTO portals (id, slug, name, profile_id, tools, memory, language, access, passcode_hash, branding, enabled, created_at, updated_at, hourly_limit, max_message, concurrency)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Slug, p.Name, p.ProfileID, p.Tools, p.Memory, p.Language, p.Access, nullable(p.passcodeHash), string(p.Branding), p.Enabled, now, now, p.HourlyLimit, p.MaxMessage, p.Concurrency)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Portal{}, ErrSlugTaken
		}
		return Portal{}, err
	}
	return s.Get(ctx, p.ID)
}

// Update changes a portal.
func (s *Store) Update(ctx context.Context, id string, in Input) (Portal, error) {
	p, err := s.Get(ctx, id)
	if err != nil {
		return Portal{}, err
	}
	if err := in.apply(&p); err != nil {
		return Portal{}, err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE portals SET slug = ?, name = ?, profile_id = ?, tools = ?, memory = ?, language = ?, access = ?, passcode_hash = ?, branding = ?, enabled = ?, updated_at = ?, hourly_limit = ?, max_message = ?, concurrency = ? WHERE id = ?`,
		p.Slug, p.Name, p.ProfileID, p.Tools, p.Memory, p.Language, p.Access, nullable(p.passcodeHash), string(p.Branding), p.Enabled, s.now().UTC().Format(time.RFC3339Nano), p.HourlyLimit, p.MaxMessage, p.Concurrency, id)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Portal{}, ErrSlugTaken
		}
		return Portal{}, err
	}
	return s.Get(ctx, id)
}

// Delete removes a portal and disables its guests, who can no longer
// chat; their conversations stay until the guests are removed.
func (s *Store) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM portals WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	_, err = s.db.ExecContext(ctx, `UPDATE people SET disabled_at = COALESCE(disabled_at, ?) WHERE portal_id = ?`, s.now().UTC().Format(time.RFC3339Nano), id)
	return err
}

// CheckPasscode reports whether code opens p.
func (p Portal) CheckPasscode(code string) bool {
	return p.passcodeHash != "" && auth.CheckPassword(p.passcodeHash, code)
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
