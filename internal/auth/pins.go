package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"strings"
)

// Pinning profiles (#345): an Admin keeps Members and Visitors, or one
// person, to one or more profiles, such as a topic-controlled assistant.
// The Owner and Admins are never pinned, so whoever runs Toskar keeps the
// whole assistant; a portal's guests use the portal's profile.

// ErrPinRole is pinning someone who can't be pinned.
var ErrPinRole = errors.New("only Members and Visitors are pinned to profiles")

// Pinnable reports a role that can be pinned to profiles.
func (r Role) Pinnable() bool { return r == RoleMember || r == RoleVisitor }

// MaxPinnedProfiles is the most profiles a pin names.
const MaxPinnedProfiles = 50

func cleanProfiles(ids []string) ([]string, error) {
	out := []string{}
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	if len(out) > MaxPinnedProfiles {
		return nil, errors.New("a pin names up to 50 profiles")
	}
	return out, nil
}

// SetProfiles pins a person to profiles in place of their role's: nil is
// their role's again, and an empty list is any profile.
func (p *People) SetProfiles(ctx context.Context, id string, profiles *[]string) (Person, error) {
	person, err := p.Get(ctx, id)
	if err != nil {
		return Person{}, err
	}
	if !person.Role.Pinnable() || person.PortalID != "" {
		return Person{}, ErrPinRole
	}
	var raw any
	if profiles != nil {
		list, err := cleanProfiles(*profiles)
		if err != nil {
			return Person{}, err
		}
		b, _ := json.Marshal(list)
		raw = string(b)
	}
	if _, err := p.db.ExecContext(ctx, `UPDATE people SET profiles_json = ? WHERE id = ?`, raw, id); err != nil {
		return Person{}, err
	}
	return p.Get(ctx, id)
}

// RoleProfiles are the profiles each pinned role is kept to.
func (p *People) RoleProfiles(ctx context.Context) (map[Role][]string, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT role, profiles_json FROM role_profiles`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[Role][]string{}
	for rows.Next() {
		var role, raw string
		if err := rows.Scan(&role, &raw); err != nil {
			return nil, err
		}
		var list []string
		if json.Unmarshal([]byte(raw), &list) == nil && len(list) > 0 {
			out[Role(role)] = list
		}
	}
	return out, rows.Err()
}

// SetRoleProfiles pins a role to profiles; an empty list is any profile.
func (p *People) SetRoleProfiles(ctx context.Context, role Role, profiles []string) error {
	if !role.Pinnable() {
		return ErrPinRole
	}
	list, err := cleanProfiles(profiles)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		_, err := p.db.ExecContext(ctx, `DELETE FROM role_profiles WHERE role = ?`, string(role))
		return err
	}
	b, _ := json.Marshal(list)
	_, err = p.db.ExecContext(ctx, `INSERT INTO role_profiles (role, profiles_json) VALUES (?, ?)
		ON CONFLICT(role) DO UPDATE SET profiles_json = excluded.profiles_json`, string(role), string(b))
	return err
}

// AllowedProfiles are the profiles a person may chat with, or nil for any:
// their own pin, else their role's.
func (p *People) AllowedProfiles(ctx context.Context, person Person) ([]string, error) {
	if !person.Role.Pinnable() || person.PortalID != "" {
		return nil, nil
	}
	if person.Profiles != nil {
		if len(*person.Profiles) == 0 {
			return nil, nil
		}
		return *person.Profiles, nil
	}
	var raw string
	err := p.db.QueryRowContext(ctx, `SELECT profiles_json FROM role_profiles WHERE role = ?`, string(person.Role)).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var list []string
	if err := json.Unmarshal([]byte(raw), &list); err != nil || len(list) == 0 {
		return nil, err
	}
	return list, nil
}
