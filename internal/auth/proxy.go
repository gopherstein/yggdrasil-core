package auth

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Sign-in by a trusted reverse proxy (#206): Authelia, Authentik,
// Cloudflare Access, Tailscale, and the like sign people in, then name them
// in a header. Toskar believes that header only from the proxies the Owner
// lists, by the address the connection comes from, never by a forwarded
// address a client could write.

// ViaProxy is a request a trusted proxy signed in.
const ViaProxy = "proxy"

// ErrProxyRefused is someone the proxy signed in whom no role lets in.
var ErrProxyRefused = errors.New("your sign-in doesn't give you a role on this Toskar")

// Proxy is sign-in by trusted reverse proxies.
type Proxy struct {
	trusted      []netip.Prefix
	userHeader   string
	nameHeader   string
	groupsHeader string
	roles        groupRoles
	ownerUser    string
}

// ProxySettings are what NewProxy reads; see config.ProxyAuth.
type ProxySettings struct {
	Trusted                              []string
	UserHeader, NameHeader, GroupsHeader string
	AdminGroups, MemberGroups            []string
	DefaultRole                          string
	OwnerUser                            string
}

// NewProxy is sign-in by the proxies at trusted (IPs or CIDRs), or nil
// when none are listed. A bad address or default role is an error, so a
// mistake doesn't quietly let people in.
func NewProxy(s ProxySettings) (*Proxy, error) {
	if len(s.Trusted) == 0 {
		return nil, nil
	}
	p := &Proxy{
		userHeader:   orDefault(s.UserHeader, "Remote-User"),
		nameHeader:   orDefault(s.NameHeader, "Remote-Name"),
		groupsHeader: orDefault(s.GroupsHeader, "Remote-Groups"),
		ownerUser:    strings.TrimSpace(s.OwnerUser),
	}
	for _, t := range s.Trusted {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if prefix, err := netip.ParsePrefix(t); err == nil {
			p.trusted = append(p.trusted, prefix.Masked())
			continue
		}
		addr, err := netip.ParseAddr(t)
		if err != nil {
			return nil, errors.New("trusted_proxies: " + t + " is not an IP address or CIDR")
		}
		addr = addr.Unmap()
		p.trusted = append(p.trusted, netip.PrefixFrom(addr, addr.BitLen()))
	}
	roles, err := newGroupRoles(s.AdminGroups, s.MemberGroups, s.DefaultRole)
	if err != nil {
		return nil, errors.New("proxy_auth." + err.Error())
	}
	p.roles = roles
	return p, nil
}

// groupRoles gives roles by the groups someone is in where they sign in.
type groupRoles struct {
	admin, member map[string]bool
	def           Role // "" refuses
}

func newGroupRoles(admin, member []string, def string) (groupRoles, error) {
	g := groupRoles{admin: set(admin), member: set(member)}
	switch strings.ToLower(strings.TrimSpace(def)) {
	case "", "member":
		g.def = RoleMember
	case "visitor":
		g.def = RoleVisitor
	case "none":
	default:
		return groupRoles{}, errors.New("default_role must be member, visitor, or none")
	}
	return g, nil
}

// role is the role groups give, or false when none lets them in.
func (g groupRoles) role(groups []string) (Role, bool) {
	for _, x := range groups {
		if g.admin[x] {
			return RoleAdmin, true
		}
	}
	for _, x := range groups {
		if g.member[x] {
			return RoleMember, true
		}
	}
	return g.def, g.def != ""
}

func orDefault(v, def string) string {
	if v = strings.TrimSpace(v); v != "" {
		return v
	}
	return def
}

func set(list []string) map[string]bool {
	out := map[string]bool{}
	for _, v := range list {
		if v = strings.TrimSpace(v); v != "" {
			out[v] = true
		}
	}
	return out
}

// FromProxy reports whether the connection comes from a trusted proxy.
func (p *Proxy) FromProxy(r *http.Request) bool {
	if p == nil {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, t := range p.trusted {
		if t.Contains(addr) {
			return true
		}
	}
	return false
}

// ProxyIdentity is who a trusted proxy says signed in.
type ProxyIdentity struct {
	User   string
	Name   string
	Groups []string
}

// Identity is who a request from a trusted proxy names; ok is false when
// it names nobody.
func (p *Proxy) Identity(r *http.Request) (ProxyIdentity, bool) {
	user := strings.TrimSpace(r.Header.Get(p.userHeader))
	if user == "" || len(user) > 320 {
		return ProxyIdentity{}, false
	}
	id := ProxyIdentity{User: user, Name: strings.TrimSpace(r.Header.Get(p.nameHeader))}
	for _, g := range strings.FieldsFunc(r.Header.Get(p.groupsHeader), func(c rune) bool { return c == ',' || c == '|' }) {
		if g = strings.TrimSpace(g); g != "" {
			id.Groups = append(id.Groups, g)
		}
	}
	return id, true
}

// Role is the role id's groups give, or false when none lets them in.
func (p *Proxy) Role(id ProxyIdentity) (Role, bool) { return p.roles.role(id.Groups) }

// IsOwner reports whether id is the Owner.
func (p *Proxy) IsOwner(id ProxyIdentity) bool {
	return p.ownerUser != "" && strings.EqualFold(id.User, p.ownerUser)
}

// Proxied is the person a trusted proxy signed in as id: the Owner, or the
// person with that name, made the first time and given the role their
// groups give each time. A disabled person, or one no role lets in, is
// refused.
func (p *People) Proxied(ctx context.Context, proxy *Proxy, id ProxyIdentity) (Person, error) {
	if proxy.IsOwner(id) {
		return p.Active(ctx, OwnerID)
	}
	role, ok := proxy.Role(id)
	if !ok {
		return Person{}, ErrProxyRefused
	}
	name := id.Name
	if name == "" {
		name = id.User
		if at := strings.IndexByte(name, '@'); at > 0 {
			name = name[:at]
		}
	}
	return p.External(ctx, ExternalSignIn{ID: id.User, Label: id.User, Name: name, Role: role})
}

// ExternalSignIn is someone signed in elsewhere (#206): by a trusted proxy
// or an OpenID Connect provider.
type ExternalSignIn struct {
	// ID is who they are there, unique and unchanging.
	ID string
	// Label shows who they are there, such as an email address.
	Label string
	// Name is their display name, for a new person.
	Name string
	// Role is what their groups there give.
	Role Role
}

// External is the person signed in elsewhere as in: the one with that ID,
// made the first time, with the role in gives each time. A disabled
// person is refused.
func (p *People) External(ctx context.Context, in ExternalSignIn) (Person, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = in.Label
	}
	if len([]rune(name)) > 80 {
		name = string([]rune(name)[:80])
	}
	var personID string
	err := p.db.QueryRowContext(ctx, `SELECT id FROM people WHERE external_id = ?`, in.ID).Scan(&personID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		personID = uuid.NewString()
		if _, err := p.db.ExecContext(ctx, `INSERT INTO people (id, name, role, created_at, external_id, external_label) VALUES (?, ?, ?, ?, ?, ?)`,
			personID, name, string(in.Role), stamp(time.Now()), in.ID, in.Label); err != nil {
			// Made at the same moment by another request.
			if err2 := p.db.QueryRowContext(ctx, `SELECT id FROM people WHERE external_id = ?`, in.ID).Scan(&personID); err2 != nil {
				return Person{}, err
			}
		}
	case err != nil:
		return Person{}, err
	}
	person, err := p.Active(ctx, personID)
	if err != nil {
		return Person{}, err
	}
	if person.Role != RoleOwner && (person.Role != in.Role || person.External != in.Label) {
		if _, err := p.db.ExecContext(ctx, `UPDATE people SET role = ?, external_label = ? WHERE id = ?`, string(in.Role), in.Label, personID); err != nil {
			return Person{}, err
		}
		person.Role, person.External = in.Role, in.Label
	}
	return person, nil
}
