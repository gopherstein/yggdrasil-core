package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/yeixio/yggdrasil-core/internal/imagegen"
)

// BindImages attaches image generation setup (Gungnir §17).
func (s *Server) BindImages(setup *imagegen.Setup) {
	s.images = setup
}

func (s *Server) imageRoutes(api *mux.Router) {
	api.HandleFunc("/images/setup", s.handleImageSetup).Methods(http.MethodGet)
	api.HandleFunc("/images/setup", s.handleStartImageSetup).Methods(http.MethodPost)
	api.HandleFunc("/images/setup", s.handleCancelImageSetup).Methods(http.MethodDelete)
	api.HandleFunc("/images/models/{id}", s.handleRemoveImageModel).Methods(http.MethodDelete)
}

func (s *Server) imagesBound(w http.ResponseWriter) bool {
	if s.images == nil {
		writeErr(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Image generation is not available.", nil)
		return false
	}
	return true
}

// handleImageSetup reports what is installed, the models on offer, and any
// setup in progress.
func (s *Server) handleImageSetup(w http.ResponseWriter, r *http.Request) {
	if !s.imagesBound(w) {
		return
	}
	writeJSON(w, http.StatusOK, s.images.Status())
}

// handleStartImageSetup installs stable-diffusion.cpp and a model in the
// background.
func (s *Server) handleStartImageSetup(w http.ResponseWriter, r *http.Request) {
	if !s.imagesBound(w) {
		return
	}
	var body struct {
		ModelID string `json:"model_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "INVALID_JSON", "invalid body", nil)
		return
	}
	if err := s.images.Start(body.ModelID); err != nil {
		writeErr(w, http.StatusConflict, "IMAGE_SETUP_REFUSED", err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusAccepted, s.images.Status())
}

// handleCancelImageSetup stops a setup; what was downloaded is kept.
func (s *Server) handleCancelImageSetup(w http.ResponseWriter, r *http.Request) {
	if !s.imagesBound(w) {
		return
	}
	s.images.Cancel()
	writeJSON(w, http.StatusOK, s.images.Status())
}

// handleRemoveImageModel deletes an installed image model.
func (s *Server) handleRemoveImageModel(w http.ResponseWriter, r *http.Request) {
	if !s.imagesBound(w) {
		return
	}
	if err := s.images.Remove(mux.Vars(r)["id"]); err != nil {
		writeErr(w, http.StatusConflict, "IMAGE_MODEL_BUSY", err.Error(), nil)
		return
	}
	writeJSON(w, http.StatusOK, s.images.Status())
}
