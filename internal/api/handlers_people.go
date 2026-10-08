package api

import (
	"net/http"

	"github.com/yeixio/toskar-core/internal/auth"
)

// handleMe is who the request is from (#206): their person, role, and how
// Toskar knows, so a client can show what that person may do.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, auth.PrincipalFrom(r.Context()))
}
