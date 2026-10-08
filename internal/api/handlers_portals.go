package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/yeixio/toskar-core/internal/auth"
	"github.com/yeixio/toskar-core/internal/portals"
)

// Chat portals (#205). Admins manage portals; a portal's page shows its
// branding to anyone, and a visitor enters (with the passcode, when it has
// one) to become a guest: a person who belongs to the portal, signed in by
// a cookie of the portal's own. A request carries its portal in the
// X-Toskar-Portal header, so the guest cookie never reaches the main app,
// and a guest reaches only the portal's chat.

// portalHeader names the portal a portal page's request is for.
const portalHeader = "X-Toskar-Portal"

// portalCookie holds a portal guest's session.
func portalCookie(slug string) string { return "toskar_portal_" + slug }

// Wrong passcodes: ten per address in ten minutes, then it waits.
var portalByAddress = auth.NewLimiter(10, 10*time.Minute)

// guestRoutes are all a portal's guest reaches: its chats and their files,
// the live events about them, a tool's question, and leaving.
var guestRoutes = map[string]bool{
	"GET /api/v1/health":                       true,
	"GET /api/v1/me":                           true,
	"DELETE /api/v1/session":                   true,
	"GET /api/v1/events":                       true,
	"POST /api/v1/chat":                        true,
	"POST /api/v1/chat/stop":                   true,
	"POST /api/v1/tools/decide":                true,
	"GET /api/v1/conversations":                true,
	"POST /api/v1/conversations":               true,
	"PATCH /api/v1/conversations/{id}":         true,
	"DELETE /api/v1/conversations/{id}":        true,
	"GET /api/v1/conversations/{id}/messages":  true,
	"GET /api/v1/conversations/{id}/artifacts": true,
	"GET /api/v1/artifacts/{id}":               true,
	"GET /api/v1/artifacts/{id}/content":       true,
	"GET /api/v1/portals/{slug}/page":          true,
	"POST /api/v1/portals/{slug}/enter":        true,
}

func (s *Server) portalRoutes(api *mux.Router) {
	api.HandleFunc("/portals", s.portalsReady(s.handleListPortals)).Methods(http.MethodGet)
	api.HandleFunc("/portals", s.portalsReady(s.handleCreatePortal)).Methods(http.MethodPost)
	api.HandleFunc("/portals/{id}", s.portalsReady(s.handleGetPortal)).Methods(http.MethodGet)
	api.HandleFunc("/portals/{id}", s.portalsReady(s.handleUpdatePortal)).Methods(http.MethodPatch)
	api.HandleFunc("/portals/{id}", s.portalsReady(s.handleDeletePortal)).Methods(http.MethodDelete)
	api.HandleFunc("/portals/{slug}/page", s.portalsReady(s.handlePortalPage)).Methods(http.MethodGet)
	api.HandleFunc("/portals/{slug}/enter", s.portalsReady(s.handleEnterPortal)).Methods(http.MethodPost)
}

func (s *Server) portalsReady(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.deps.Portals == nil {
			writeErr(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Portals aren't available.", nil)
			return
		}
		h(w, r)
	}
}

func portalError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, portals.ErrNotFound):
		writeErr(w, http.StatusNotFound, "PORTAL_NOT_FOUND", err.Error(), nil)
	case errors.Is(err, portals.ErrSlugTaken):
		writeErr(w, http.StatusConflict, "PORTAL_SLUG_TAKEN", err.Error(), nil)
	case errors.Is(err, portals.ErrBadSlug), errors.Is(err, portals.ErrBadName), errors.Is(err, portals.ErrBadTools),
		errors.Is(err, portals.ErrBadAccess), errors.Is(err, portals.ErrNoPasscode), errors.Is(err, portals.ErrBadBranding),
		errors.Is(err, portals.ErrBadLanguage):
		writeErr(w, http.StatusBadRequest, "PORTAL_INVALID", err.Error(), nil)
	default:
		writeErr(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
	}
}

func (s *Server) handleListPortals(w http.ResponseWriter, r *http.Request) {
	list, err := s.deps.Portals.List(r.Context())
	if err != nil {
		portalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func decodePortal(w http.ResponseWriter, r *http.Request) (portals.Input, bool) {
	var in portals.Input
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "invalid body", nil)
		return in, false
	}
	return in, true
}

func (s *Server) handleCreatePortal(w http.ResponseWriter, r *http.Request) {
	in, ok := decodePortal(w, r)
	if !ok {
		return
	}
	p, err := s.deps.Portals.Create(r.Context(), in)
	if err != nil {
		portalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleGetPortal(w http.ResponseWriter, r *http.Request) {
	p, err := s.deps.Portals.Get(r.Context(), mux.Vars(r)["id"])
	if err != nil {
		portalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleUpdatePortal(w http.ResponseWriter, r *http.Request) {
	in, ok := decodePortal(w, r)
	if !ok {
		return
	}
	p, err := s.deps.Portals.Update(r.Context(), mux.Vars(r)["id"], in)
	if err != nil {
		portalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleDeletePortal(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Portals.Delete(r.Context(), mux.Vars(r)["id"]); err != nil {
		portalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// portalPage is what a portal's page shows before anyone enters.
type portalPage struct {
	Slug     string          `json:"slug"`
	Name     string          `json:"name"`
	Access   string          `json:"access"`
	Language string          `json:"language"`
	Branding json.RawMessage `json:"branding"`
	// Entered is true when this browser is already the portal's guest.
	Entered bool `json:"entered"`
}

// livePortal is the enabled portal at slug.
func (s *Server) livePortal(r *http.Request, slug string) (portals.Portal, bool) {
	p, err := s.deps.Portals.BySlug(r.Context(), slug)
	if err != nil || !p.Enabled {
		return portals.Portal{}, false
	}
	return p, true
}

func (s *Server) handlePortalPage(w http.ResponseWriter, r *http.Request) {
	p, ok := s.livePortal(r, mux.Vars(r)["slug"])
	if !ok {
		writeErr(w, http.StatusNotFound, "PORTAL_NOT_FOUND", "There's no portal at this address, or it's turned off.", nil)
		return
	}
	_, entered := s.guestOf(r, p)
	writeJSON(w, http.StatusOK, portalPage{Slug: p.Slug, Name: p.Name, Access: p.Access, Language: p.Language, Branding: p.Branding, Entered: entered})
}

// handleEnterPortal makes this browser the portal's guest: again the same
// guest when it already is one, else a new one, with the passcode when the
// portal has one.
func (s *Server) handleEnterPortal(w http.ResponseWriter, r *http.Request) {
	if !s.signInReady(w) {
		return
	}
	p, ok := s.livePortal(r, mux.Vars(r)["slug"])
	if !ok {
		writeErr(w, http.StatusNotFound, "PORTAL_NOT_FOUND", "There's no portal at this address, or it's turned off.", nil)
		return
	}
	if guest, ok := s.guestOf(r, p); ok {
		writeJSON(w, http.StatusOK, auth.Principal{Person: guest, Via: auth.ViaSession})
		return
	}
	if p.Access == portals.AccessPasscode {
		var body struct {
			Passcode string `json:"passcode"`
		}
		_ = json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&body)
		address := remoteIP(r)
		if portalByAddress.Blocked(address) {
			writeErr(w, http.StatusTooManyRequests, "SIGN_IN_THROTTLED", "too many tries; wait 10 minutes and try again", nil)
			return
		}
		if !p.CheckPasscode(strings.TrimSpace(body.Passcode)) {
			portalByAddress.Fail(address)
			writeErr(w, http.StatusUnauthorized, "PORTAL_PASSCODE", portals.ErrWrongPasscode.Error(), nil)
			return
		}
	}
	guest, err := s.deps.People.CreateGuest(r.Context(), p.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	token, expires, err := s.deps.Sessions.Create(r.Context(), guest.ID, r.UserAgent(), remoteIP(r))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error(), nil)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: portalCookie(p.Slug), Value: token, Path: "/", Expires: expires,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: r.TLS != nil,
	})
	writeJSON(w, http.StatusOK, auth.Principal{Person: guest, Via: auth.ViaSession})
}

// guestOf is this browser's guest of portal p, while the guest and their
// session are good.
func (s *Server) guestOf(r *http.Request, p portals.Portal) (auth.Person, bool) {
	if s.deps.Sessions == nil || s.deps.People == nil {
		return auth.Person{}, false
	}
	cookie, err := r.Cookie(portalCookie(p.Slug))
	if err != nil {
		return auth.Person{}, false
	}
	id, err := s.deps.Sessions.Person(r.Context(), cookie.Value)
	if err != nil {
		return auth.Person{}, false
	}
	person, err := s.deps.People.Active(r.Context(), id)
	if err != nil || person.PortalID != p.ID {
		return auth.Person{}, false
	}
	return person, true
}

// guestPrincipal is the guest a portal page's request comes from, by its
// X-Toskar-Portal header and that portal's cookie. off is true when the
// portal is unknown or turned off.
func (s *Server) guestPrincipal(r *http.Request) (principal auth.Principal, ok, off bool) {
	slug := strings.ToLower(strings.TrimSpace(r.Header.Get(portalHeader)))
	if slug == "" || s.deps.Portals == nil {
		return auth.Principal{}, false, false
	}
	p, live := s.livePortal(r, slug)
	if !live {
		return auth.Principal{}, false, true
	}
	guest, ok := s.guestOf(r, p)
	if !ok {
		return auth.Principal{}, false, false
	}
	return auth.Principal{Person: guest, Via: auth.ViaSession}, true, false
}

// portalOf is the portal a guest belongs to, when ctx's person is one.
func (s *Server) portalOf(r *http.Request) (portals.Portal, bool) {
	id := auth.PrincipalFrom(r.Context()).Person.PortalID
	if id == "" || s.deps.Portals == nil {
		return portals.Portal{}, false
	}
	p, err := s.deps.Portals.Get(r.Context(), id)
	if err != nil || !p.Enabled {
		return portals.Portal{}, false
	}
	return p, true
}
