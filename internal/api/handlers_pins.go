package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/yeixio/toskar-core/internal/auth"
)

// Pinning profiles to roles and people (#345), for Admins.

func (s *Server) pinRoutes(api *mux.Router) {
	api.HandleFunc("/profile-pins", s.atLeast(auth.RoleAdmin, s.handleProfilePins)).Methods(http.MethodGet)
	api.HandleFunc("/profile-pins/{role}", s.atLeast(auth.RoleAdmin, s.handleSetRoleProfiles)).Methods(http.MethodPut)
	api.HandleFunc("/people/{id}/profiles", s.atLeast(auth.RoleAdmin, s.handleSetPersonProfiles)).Methods(http.MethodPut)
}

// profilesExist refuses a pin to a profile that doesn't exist.
func (s *Server) profilesExist(w http.ResponseWriter, r *http.Request, ids []string) bool {
	if s.deps.GetProfile == nil {
		return true
	}
	for _, id := range ids {
		if _, err := s.deps.GetProfile(r.Context(), id); err != nil {
			writeErr(w, http.StatusBadRequest, "PIN_INVALID", "there's no profile "+id, map[string]any{"profile_id": id})
			return false
		}
	}
	return true
}

func (s *Server) pinError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrPinRole):
		writeErr(w, http.StatusBadRequest, "PIN_INVALID", err.Error(), nil)
	case errors.Is(err, auth.ErrNoPerson):
		writeErr(w, http.StatusNotFound, "PERSON_NOT_FOUND", err.Error(), nil)
	default:
		writeErr(w, http.StatusBadRequest, "PIN_INVALID", err.Error(), nil)
	}
}

func (s *Server) handleProfilePins(w http.ResponseWriter, r *http.Request) {
	if s.deps.People == nil {
		writeErr(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "People aren't available.", nil)
		return
	}
	roles, err := s.deps.People.RoleProfiles(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "PINS_FAILED", err.Error(), nil)
		return
	}
	out := map[string][]string{string(auth.RoleMember): {}, string(auth.RoleVisitor): {}}
	for role, list := range roles {
		out[string(role)] = list
	}
	writeJSON(w, http.StatusOK, map[string]any{"roles": out})
}

func (s *Server) handleSetRoleProfiles(w http.ResponseWriter, r *http.Request) {
	if s.deps.People == nil {
		writeErr(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "People aren't available.", nil)
		return
	}
	var body struct {
		Profiles []string `json:"profiles"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "invalid body", nil)
		return
	}
	if !s.profilesExist(w, r, body.Profiles) {
		return
	}
	role := auth.Role(mux.Vars(r)["role"])
	if err := s.deps.People.SetRoleProfiles(r.Context(), role, body.Profiles); err != nil {
		s.pinError(w, err)
		return
	}
	s.handleProfilePins(w, r)
}

// handleSetPersonProfiles pins one person: profiles null is their role's
// again, and [] is any profile.
func (s *Server) handleSetPersonProfiles(w http.ResponseWriter, r *http.Request) {
	if s.deps.People == nil {
		writeErr(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "People aren't available.", nil)
		return
	}
	var body struct {
		Profiles *[]string `json:"profiles"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "invalid body", nil)
		return
	}
	if body.Profiles != nil && !s.profilesExist(w, r, *body.Profiles) {
		return
	}
	person, err := s.deps.People.SetProfiles(r.Context(), mux.Vars(r)["id"], body.Profiles)
	if err != nil {
		s.pinError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, person)
}

// myProfiles are the profiles the request's person may chat with, or nil
// for any.
func (s *Server) myProfiles(r *http.Request) []string {
	who := auth.PrincipalFrom(r.Context()).Person
	if s.deps.People == nil || !who.Role.Pinnable() {
		return nil
	}
	person, err := s.deps.People.Get(r.Context(), who.ID)
	if err != nil {
		return nil
	}
	list, err := s.deps.People.AllowedProfiles(r.Context(), person)
	if err != nil || list == nil {
		return nil
	}
	out := []string{}
	for _, id := range list {
		if s.deps.GetProfile == nil {
			out = append(out, id)
		} else if _, err := s.deps.GetProfile(r.Context(), id); err == nil {
			out = append(out, id)
		}
	}
	return out
}
