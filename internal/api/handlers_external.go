package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// ExternalServers is the OpenAI-compatible server whose models can be
// chosen for a chat (#111).
type ExternalServers interface {
	ExternalServer(ctx context.Context) (ExternalServer, error)
	SetExternalServer(ctx context.Context, in ExternalServerInput) (ExternalServer, error)
}

// ExternalServer is its settings and the models it offers.
type ExternalServer struct {
	BaseURL string `json:"base_url"`
	// HasKey reports a stored API key, never its value.
	HasKey bool     `json:"has_key"`
	Models []string `json:"models"`
	// Error is why the model list couldn't be read.
	Error string `json:"error,omitempty"`
}

// ExternalServerInput changes it. An empty APIKey keeps the stored key.
type ExternalServerInput struct {
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key,omitempty"`
	ClearKey bool   `json:"clear_key,omitempty"`
}

// BindExternal attaches the external server routes.
func (s *Server) BindExternal(e ExternalServers) { s.external = e }

func (s *Server) externalRoutes(api *mux.Router) {
	api.HandleFunc("/external-server", s.externalHandler(s.handleGetExternal)).Methods(http.MethodGet)
	api.HandleFunc("/external-server", s.externalHandler(s.handlePutExternal)).Methods(http.MethodPut)
}

func (s *Server) externalHandler(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.external == nil {
			writeErr(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "The external server is not available.", nil)
			return
		}
		h(w, r)
	}
}

func (s *Server) handleGetExternal(w http.ResponseWriter, r *http.Request) {
	out, err := s.external.ExternalServer(r.Context())
	if err != nil {
		writeErrFrom(w, http.StatusInternalServerError, "EXTERNAL_FAILED", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handlePutExternal(w http.ResponseWriter, r *http.Request) {
	var in ExternalServerInput
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&in); err != nil {
		writeErr(w, http.StatusBadRequest, "EXTERNAL_BAD_URL", "The body must be {\"base_url\", \"api_key\"}.", nil)
		return
	}
	out, err := s.external.SetExternalServer(r.Context(), in)
	if err != nil {
		writeErrFrom(w, http.StatusBadRequest, "EXTERNAL_FAILED", err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}
