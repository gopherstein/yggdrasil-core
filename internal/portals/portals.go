// Package portals keeps chat portals (#205): branded chat pages an Admin
// publishes for the people they serve, each answering with its own
// profile, tools, memory setting, and access.
package portals

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
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
	// DefaultRetentionDays is how long a new portal keeps its visitors'
	// conversations.
	DefaultRetentionDays = 30
)

// Access is who may chat in a portal.
const (
	// AccessOpen is anyone who can reach this Toskar.
	AccessOpen = "open"
	// AccessPasscode is anyone with the portal's shared passcode.
	AccessPasscode = "passcode"
	// AccessMembers is the people who sign in to this Toskar as Members
	// and up (#206), as themselves.
	AccessMembers = "members"
	// AccessInvited is the visitors an Admin invites by name, each with a
	// one-time link.
	AccessInvited = "invited"
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
	// RetentionDays is how long visitors' conversations are kept; 0 keeps
	// them.
	RetentionDays int `json:"retention_days"`
	// EmbedOrigins are the websites that may show the portal in a frame,
	// such as https://shop.example.com.
	EmbedOrigins []string `json:"embed_origins"`
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
	ErrBadOrigins    = errors.New("each website is an origin such as https://shop.example.com, with no path, and there can be 20")
	ErrBadRetention  = errors.New("visitors' chats are kept 1 to 3650 days, or 0 to keep them")
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

const columns = `id, slug, name, profile_id, tools, memory, language, access, COALESCE(passcode_hash, ''), branding, enabled, created_at, updated_at, hourly_limit, max_message, concurrency, embed_origins, retention_days`

func scan(row interface{ Scan(...any) error }) (Portal, error) {
	var p Portal
	var branding, created, updated, origins string
	if err := row.Scan(&p.ID, &p.Slug, &p.Name, &p.ProfileID, &p.Tools, &p.Memory, &p.Language, &p.Access, &p.passcodeHash, &branding, &p.Enabled, &created, &updated, &p.HourlyLimit, &p.MaxMessage, &p.Concurrency, &origins, &p.RetentionDays); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Portal{}, ErrNotFound
		}
		return Portal{}, err
	}
	p.Branding = json.RawMessage(branding)
	if json.Unmarshal([]byte(origins), &p.EmbedOrigins) != nil || p.EmbedOrigins == nil {
		p.EmbedOrigins = []string{}
	}
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
	Slug          *string          `json:"slug,omitempty"`
	Name          *string          `json:"name,omitempty"`
	ProfileID     *string          `json:"profile_id,omitempty"`
	Tools         *string          `json:"tools,omitempty"`
	Memory        *bool            `json:"memory,omitempty"`
	Language      *string          `json:"language,omitempty"`
	Access        *string          `json:"access,omitempty"`
	Passcode      *string          `json:"passcode,omitempty"`
	Branding      *json.RawMessage `json:"branding,omitempty"`
	Enabled       *bool            `json:"enabled,omitempty"`
	EmbedOrigins  *[]string        `json:"embed_origins,omitempty"`
	RetentionDays *int             `json:"retention_days,omitempty"`
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
		case AccessOpen, AccessPasscode, AccessMembers, AccessInvited:
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
	if in.EmbedOrigins != nil {
		origins, err := cleanOrigins(*in.EmbedOrigins)
		if err != nil {
			return err
		}
		p.EmbedOrigins = origins
	}
	if in.RetentionDays != nil {
		if *in.RetentionDays < 0 || *in.RetentionDays > 3650 {
			return ErrBadRetention
		}
		p.RetentionDays = *in.RetentionDays
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
		HourlyLimit: DefaultHourlyLimit, MaxMessage: DefaultMaxMessage, Concurrency: DefaultConcurrency, EmbedOrigins: []string{}, RetentionDays: DefaultRetentionDays}
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
	_, err := s.db.ExecContext(ctx, `INSERT INTO portals (id, slug, name, profile_id, tools, memory, language, access, passcode_hash, branding, enabled, created_at, updated_at, hourly_limit, max_message, concurrency, embed_origins, retention_days)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Slug, p.Name, p.ProfileID, p.Tools, p.Memory, p.Language, p.Access, nullable(p.passcodeHash), string(p.Branding), p.Enabled, now, now, p.HourlyLimit, p.MaxMessage, p.Concurrency, originsJSON(p.EmbedOrigins), p.RetentionDays)
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
	_, err = s.db.ExecContext(ctx, `UPDATE portals SET slug = ?, name = ?, profile_id = ?, tools = ?, memory = ?, language = ?, access = ?, passcode_hash = ?, branding = ?, enabled = ?, updated_at = ?, hourly_limit = ?, max_message = ?, concurrency = ?, embed_origins = ?, retention_days = ? WHERE id = ?`,
		p.Slug, p.Name, p.ProfileID, p.Tools, p.Memory, p.Language, p.Access, nullable(p.passcodeHash), string(p.Branding), p.Enabled, s.now().UTC().Format(time.RFC3339Nano), p.HourlyLimit, p.MaxMessage, p.Concurrency, originsJSON(p.EmbedOrigins), p.RetentionDays, id)
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

// cleanOrigins checks websites that may frame a portal: each an http or
// https origin with no path, at most 20, without repeats.
func cleanOrigins(in []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, raw := range in {
		o := strings.TrimRight(strings.TrimSpace(raw), "/")
		if o == "" {
			continue
		}
		u, err := url.Parse(o)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil ||
			strings.ContainsAny(o, " ;'\"*") {
			return nil, ErrBadOrigins
		}
		o = strings.ToLower(u.Scheme + "://" + u.Host)
		if !seen[o] {
			seen[o] = true
			out = append(out, o)
		}
	}
	if len(out) > 20 {
		return nil, ErrBadOrigins
	}
	return out, nil
}

func originsJSON(origins []string) string {
	if origins == nil {
		origins = []string{}
	}
	b, _ := json.Marshal(origins)
	return string(b)
}

// The Owner's view of a portal (#205): its visitors' conversations, which
// they're told the people who run it can read, and how much it's used.
// A Members only portal's chats are each Member's own, so they aren't here.

// Conversation is one of a portal's visitors' conversations.
type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Visitor   string    `json:"visitor"`
	Messages  int       `json:"messages"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Conversations is a portal's visitors' conversations, newest first.
func (s *Store) Conversations(ctx context.Context, portalID string, limit int) ([]Conversation, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, COALESCE(c.title, ''), p.name, (SELECT COUNT(*) FROM messages m WHERE m.conversation_id = c.id), c.created_at, c.updated_at
		FROM conversations c JOIN people p ON p.id = c.person_id
		WHERE p.portal_id = ? ORDER BY c.updated_at DESC LIMIT ?`, portalID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Conversation{}
	for rows.Next() {
		var c Conversation
		var created, updated string
		if err := rows.Scan(&c.ID, &c.Title, &c.Visitor, &c.Messages, &created, &updated); err != nil {
			return nil, err
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		c.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
		out = append(out, c)
	}
	return out, rows.Err()
}

// Message is one message of a visitor's conversation.
type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Messages is a visitor's conversation, oldest first, when it's one of the
// portal's.
func (s *Store) Messages(ctx context.Context, portalID, conversationID string) ([]Message, error) {
	var found string
	err := s.db.QueryRowContext(ctx, `SELECT c.id FROM conversations c JOIN people p ON p.id = c.person_id WHERE c.id = ? AND p.portal_id = ?`,
		conversationID, portalID).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT role, content, created_at FROM messages WHERE conversation_id = ? AND role IN ('user', 'assistant') ORDER BY created_at, rowid`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		var at string
		if err := rows.Scan(&m.Role, &m.Content, &at); err != nil {
			return nil, err
		}
		m.CreatedAt, _ = time.Parse(time.RFC3339Nano, at)
		out = append(out, m)
	}
	return out, rows.Err()
}

// Usage is how much a portal was used.
type Usage struct {
	// Visitors who wrote, conversations they started, and messages they
	// sent, over the last 7 and 30 days.
	Visitors7       int `json:"visitors_7"`
	Conversations7  int `json:"conversations_7"`
	Messages7       int `json:"messages_7"`
	Visitors30      int `json:"visitors_30"`
	Conversations30 int `json:"conversations_30"`
	Messages30      int `json:"messages_30"`
}

// Usage counts a portal's use, by its visitors' messages.
func (s *Store) Usage(ctx context.Context, portalID string) (Usage, error) {
	var u Usage
	now := s.now().UTC()
	for _, span := range []struct {
		days                              int
		visitors, conversations, messages *int
	}{{7, &u.Visitors7, &u.Conversations7, &u.Messages7}, {30, &u.Visitors30, &u.Conversations30, &u.Messages30}} {
		since := now.AddDate(0, 0, -span.days).Format(time.RFC3339Nano)
		err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(DISTINCT c.person_id), COUNT(DISTINCT c.id), COUNT(m.id)
			FROM messages m JOIN conversations c ON c.id = m.conversation_id JOIN people p ON p.id = c.person_id
			WHERE p.portal_id = ? AND m.role = 'user' AND m.created_at >= ?`, portalID, since).
			Scan(span.visitors, span.conversations, span.messages)
		if err != nil {
			return Usage{}, err
		}
	}
	return u, nil
}

// Prune deletes visitors' conversations older than each portal keeps, and
// anonymous visitors left with no conversations; it says how many
// conversations went.
func (s *Store) Prune(ctx context.Context) (int, error) {
	list, err := s.List(ctx)
	if err != nil {
		return 0, err
	}
	gone := 0
	for _, p := range list {
		if p.RetentionDays <= 0 {
			continue
		}
		cutoff := s.now().UTC().AddDate(0, 0, -p.RetentionDays).Format(time.RFC3339Nano)
		res, err := s.db.ExecContext(ctx, `DELETE FROM conversations WHERE updated_at < ? AND person_id IN (SELECT id FROM people WHERE portal_id = ?)`, cutoff, p.ID)
		if err != nil {
			return gone, err
		}
		n, _ := res.RowsAffected()
		gone += int(n)
		if _, err := s.db.ExecContext(ctx, `DELETE FROM people WHERE portal_id = ? AND invited = 0 AND created_at < ?
			AND NOT EXISTS (SELECT 1 FROM conversations c WHERE c.person_id = people.id)`, p.ID, cutoff); err != nil {
			return gone, err
		}
	}
	return gone, nil
}
