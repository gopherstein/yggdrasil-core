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
	adminGroups  map[string]bool
	memberGroups map[string]bool
	defaultRole  Role // "" refuses
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
		adminGroups:  set(s.AdminGroups),
		memberGroups: set(s.MemberGroups),
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
	switch strings.ToLower(strings.TrimSpace(s.DefaultRole)) {
	case "", "member":
		p.defaultRole = RoleMember
	case "visitor":
		p.defaultRole = RoleVisitor
	case "none":
		p.defaultRole = ""
	default:
		return nil, errors.New("proxy_auth.default_role must be member, visitor, or none")
	}
	return p, nil
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
func (p *Proxy) Role(id ProxyIdentity) (Role, bool) {
	for _, g := range id.Groups {
		if p.adminGroups[g] {
			return RoleAdmin, true
		}
	}
	for _, g := range id.Groups {
		if p.memberGroups[g] {
			return RoleMember, true
		}
	}
	return p.defaultRole, p.defaultRole != ""
}

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
	if len([]rune(name)) > 80 {
		name = string([]rune(name)[:80])
	}
	var personID string
	err := p.db.QueryRowContext(ctx, `SELECT id FROM people WHERE external_id = ?`, id.User).Scan(&personID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		personID = uuid.NewString()
		if _, err := p.db.ExecContext(ctx, `INSERT INTO people (id, name, role, created_at, external_id) VALUES (?, ?, ?, ?, ?)`,
			personID, name, string(role), stamp(time.Now()), id.User); err != nil {
			// Made at the same moment by another request.
			if err2 := p.db.QueryRowContext(ctx, `SELECT id FROM people WHERE external_id = ?`, id.User).Scan(&personID); err2 != nil {
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
	if person.Role != role && person.Role != RoleOwner {
		if _, err := p.db.ExecContext(ctx, `UPDATE people SET role = ? WHERE id = ?`, string(role), personID); err != nil {
			return Person{}, err
		}
		person.Role = role
	}
	return person, nil
}
