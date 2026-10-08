package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
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
}

// ErrNoPerson is an unknown or disabled person.
var ErrNoPerson = errors.New("no such person")

// People finds people.
type People struct {
	db *sql.DB
}

// NewPeople reads people from db.
func NewPeople(db *sql.DB) *People { return &People{db: db} }

const personColumns = `id, name, COALESCE(username, ''), role, created_at, disabled_at`

func scanPerson(row interface{ Scan(...any) error }) (Person, error) {
	var p Person
	var created string
	var disabled sql.NullString
	if err := row.Scan(&p.ID, &p.Name, &p.Username, &p.Role, &created, &disabled); err != nil {
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
	// ViaSystem is Toskar's own work across everyone, such as the
	// scheduler finding what's due. It reaches every person's data; what it
	// then does for one person runs as them.
	ViaSystem = "system"
)

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
