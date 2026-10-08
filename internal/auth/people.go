package auth

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// People and roles (#206). Toskar starts with one person, the Owner: the
// person at this computer. Others are added later with a role, which
// decides what they may see and change.

// Role is what a person may do.
type Role string

// Roles, from the most to the least.
const (
	// RoleOwner runs this Toskar: everything, including other people.
	RoleOwner Role = "owner"
	// RoleAdmin administers it: models, tools, computers, people other
	// than the Owner.
	RoleAdmin Role = "admin"
	// RoleMember uses it: their own chats, memories, files, and
	// automations.
	RoleMember Role = "member"
	// RoleVisitor chats in what they're given, such as a portal (#205).
	RoleVisitor Role = "visitor"
)

var rank = map[Role]int{RoleOwner: 4, RoleAdmin: 3, RoleMember: 2, RoleVisitor: 1}

// Valid reports a known role.
func (r Role) Valid() bool { return rank[r] > 0 }

// AtLeast reports a role that may do what min may.
func (r Role) AtLeast(min Role) bool { return rank[r] >= rank[min] && rank[min] > 0 }

// OwnerID is the Owner's person id, which the install's existing data
// belongs to.
const OwnerID = "owner"

// Person is someone who uses this Toskar.
type Person struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Username  string     `json:"username,omitempty"`
	Role      Role       `json:"role"`
	CreatedAt time.Time  `json:"created_at"`
	Disabled  *time.Time `json:"disabled_at,omitempty"`
	// SignIn is true once they've set a username and password.
	SignIn bool `json:"sign_in"`
	// External is the name a trusted proxy signs them in as (#206).
	External string `json:"external,omitempty"`
}

// ErrNoPerson is an unknown or disabled person.
var ErrNoPerson = errors.New("no such person")

// People finds people.
type People struct {
	db *sql.DB
}

// NewPeople reads people from db.
func NewPeople(db *sql.DB) *People { return &People{db: db} }

const personColumns = `id, name, COALESCE(username, ''), role, created_at, disabled_at, password_hash IS NOT NULL, COALESCE(external_id, '')`

func scanPerson(row interface{ Scan(...any) error }) (Person, error) {
	var p Person
	var created string
	var disabled sql.NullString
	if err := row.Scan(&p.ID, &p.Name, &p.Username, &p.Role, &created, &disabled, &p.SignIn, &p.External); err != nil {
		return Person{}, err
	}
	p.CreatedAt = parseTime(created)
	if disabled.Valid {
		t := parseTime(disabled.String)
		p.Disabled = &t
	}
	return p, nil
}

// Get is one person, who may be disabled.
func (p *People) Get(ctx context.Context, id string) (Person, error) {
	person, err := scanPerson(p.db.QueryRowContext(ctx, `SELECT `+personColumns+` FROM people WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Person{}, ErrNoPerson
	}
	return person, err
}

// Active is a person who may use Toskar now: known and not disabled.
func (p *People) Active(ctx context.Context, id string) (Person, error) {
	person, err := p.Get(ctx, id)
	if err != nil {
		return Person{}, err
	}
	if person.Disabled != nil {
		return Person{}, ErrNoPerson
	}
	return person, nil
}

// List is everyone, the Owner first.
func (p *People) List(ctx context.Context) ([]Person, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT `+personColumns+` FROM people
		ORDER BY CASE role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 WHEN 'member' THEN 2 ELSE 3 END, created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Person{}
	for rows.Next() {
		person, err := scanPerson(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, person)
	}
	return out, rows.Err()
}

// Errors from changing people.
var (
	ErrUsernameTaken = errors.New("that username is taken")
	ErrBadUsername   = errors.New("a username is 3 to 32 letters, digits, dots, dashes, or underscores")
	ErrBadRole       = errors.New("unknown role")
	ErrOwnerFixed    = errors.New("the Owner can't be given another role or disabled")
	ErrOneOwner      = errors.New("there is only one Owner")
	ErrBadName       = errors.New("a name is 1 to 80 characters")
)

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9._-]{3,32}$`)

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 80 {
		return "", ErrBadName
	}
	return name, nil
}

// Create adds a person with a role other than Owner. They sign in once
// they accept an invite.
func (p *People) Create(ctx context.Context, name string, role Role) (Person, error) {
	name, err := cleanName(name)
	if err != nil {
		return Person{}, err
	}
	if !role.Valid() {
		return Person{}, ErrBadRole
	}
	if role == RoleOwner {
		return Person{}, ErrOneOwner
	}
	id := uuid.NewString()
	if _, err := p.db.ExecContext(ctx, `INSERT INTO people (id, name, role, created_at) VALUES (?, ?, ?, ?)`,
		id, name, string(role), stamp(time.Now())); err != nil {
		return Person{}, err
	}
	return p.Get(ctx, id)
}

// Change is a change to a person; nil fields stay.
type Change struct {
	Name     *string `json:"name,omitempty"`
	Role     *Role   `json:"role,omitempty"`
	Disabled *bool   `json:"disabled,omitempty"`
}

// Update changes a person. The Owner keeps their role and can't be
// disabled; nobody else becomes Owner.
func (p *People) Update(ctx context.Context, id string, c Change) (Person, error) {
	person, err := p.Get(ctx, id)
	if err != nil {
		return Person{}, err
	}
	if c.Name != nil {
		name, err := cleanName(*c.Name)
		if err != nil {
			return Person{}, err
		}
		person.Name = name
	}
	if c.Role != nil && *c.Role != person.Role {
		if !c.Role.Valid() {
			return Person{}, ErrBadRole
		}
		if person.Role == RoleOwner {
			return Person{}, ErrOwnerFixed
		}
		if *c.Role == RoleOwner {
			return Person{}, ErrOneOwner
		}
		person.Role = *c.Role
	}
	disabled := person.Disabled != nil
	if c.Disabled != nil {
		if *c.Disabled && person.Role == RoleOwner {
			return Person{}, ErrOwnerFixed
		}
		disabled = *c.Disabled
	}
	var disabledAt any
	if disabled {
		if person.Disabled != nil {
			disabledAt = stamp(*person.Disabled)
		} else {
			disabledAt = stamp(time.Now())
		}
	}
	if _, err := p.db.ExecContext(ctx, `UPDATE people SET name = ?, role = ?, disabled_at = ? WHERE id = ?`,
		person.Name, string(person.Role), disabledAt, id); err != nil {
		return Person{}, err
	}
	return p.Get(ctx, id)
}

// CheckSignIn reports what's wrong with a username and password for
// person, without setting them: a bad or taken username, or a short
// password.
func (p *People) CheckSignIn(ctx context.Context, id, username, password string) error {
	username = strings.TrimSpace(username)
	if !usernameRe.MatchString(username) {
		return ErrBadUsername
	}
	if len([]rune(password)) < MinPassword {
		return ErrWeakPassword
	}
	var other string
	err := p.db.QueryRowContext(ctx, `SELECT id FROM people WHERE username = ? COLLATE NOCASE AND id <> ?`, username, id).Scan(&other)
	if err == nil {
		return ErrUsernameTaken
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return nil
}

// SetSignIn sets a person's username and password. A taken username, by
// someone else, is refused.
func (p *People) SetSignIn(ctx context.Context, id, username, password string) error {
	if err := p.CheckSignIn(ctx, id, username, password); err != nil {
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = p.db.ExecContext(ctx, `UPDATE people SET username = ?, password_hash = ? WHERE id = ?`, strings.TrimSpace(username), hash, id)
	return err
}

// SetPassword gives a person a new password, keeping their username.
func (p *People) SetPassword(ctx context.Context, id, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = p.db.ExecContext(ctx, `UPDATE people SET password_hash = ? WHERE id = ?`, hash, id)
	return err
}

// SignIn is the active person with this username and password. A wrong
// username takes as long as a wrong password, and both say the same.
func (p *People) SignIn(ctx context.Context, username, password string) (Person, error) {
	var id string
	var hash sql.NullString
	err := p.db.QueryRowContext(ctx, `SELECT id, password_hash FROM people WHERE username = ? COLLATE NOCASE AND disabled_at IS NULL`,
		strings.TrimSpace(username)).Scan(&id, &hash)
	if err != nil || !hash.Valid {
		CheckPassword(dummyHash, password)
		return Person{}, ErrSignIn
	}
	if !CheckPassword(hash.String, password) {
		return Person{}, ErrSignIn
	}
	return p.Active(ctx, id)
}

// HasSignIn reports whether a person has set a username and password.
func (p *People) HasSignIn(ctx context.Context, id string) bool {
	var hash sql.NullString
	err := p.db.QueryRowContext(ctx, `SELECT password_hash FROM people WHERE id = ?`, id).Scan(&hash)
	return err == nil && hash.Valid
}

// Principal is who a request is from, and how Toskar knows.
type Principal struct {
	Person Person `json:"person"`
	// Via is this_computer (a request from this computer, which is the
	// Owner's), api_key, or, later, a session.
	Via string `json:"via"`
	// KeyID is the API key the request used, when Via is api_key.
	KeyID string `json:"key_id,omitempty"`
}

// Ways a request is known to be from someone.
const (
	ViaThisComputer = "this_computer"
	ViaAPIKey       = "api_key"
	ViaSession      = "session"
	// ViaNone is a request from no one known yet.
	ViaNone = "none"
	// ViaSystem is Toskar's own work across everyone, such as the
	// scheduler finding what's due. It reaches every person's data; what it
	// then does for one person runs as them.
	ViaSystem = "system"
)

// Anonymous is a request no one is known to have made, such as signing in:
// it reaches no one's data.
func Anonymous() Principal { return Principal{Via: ViaNone} }

// WithSystem marks Toskar's own work across everyone.
func WithSystem(ctx context.Context) context.Context {
	return WithPrincipal(ctx, Principal{Person: Person{ID: OwnerID, Name: "Owner", Role: RoleOwner}, Via: ViaSystem})
}

// Scope is whose data ctx may reach: one person's, or everyone's for
// Toskar's own work.
func Scope(ctx context.Context) (personID string, everyone bool) {
	p := PrincipalFrom(ctx)
	return p.Person.ID, p.Via == ViaSystem
}

// AsPerson is ctx acting for person, such as an automation running as the
// person whose it is.
func AsPerson(ctx context.Context, person Person) context.Context {
	return WithPrincipal(ctx, Principal{Person: person, Via: ViaSystem + ":" + person.ID})
}

// PersonID is whose a request or job is: the person whose data it reads
// and writes (#206).
func PersonID(ctx context.Context) string { return PrincipalFrom(ctx).Person.ID }

type principalKey struct{}

// WithPrincipal carries who a request is from.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom is who a request is from. A context without one, such as
// the scheduler's own work, is the Owner's.
func PrincipalFrom(ctx context.Context) Principal {
	if p, ok := ctx.Value(principalKey{}).(Principal); ok {
		return p
	}
	return Principal{Person: Person{ID: OwnerID, Name: "Owner", Role: RoleOwner}, Via: ViaThisComputer}
}
