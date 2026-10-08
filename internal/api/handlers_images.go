package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/yeixio/toskar-core/internal/imagegen"
)

// BindImages attaches image generation setup (Gungnir §17).
func (s *Server) BindImages(setup *imagegen.Setup) {
	s.images = setup
}

// BindVideo attaches video generation setup (Gungnir §27).
func (s *Server) BindVideo(setup *imagegen.Setup) {
	s.video = setup
}

// RemoteMediaSetup answers a setup request for a paired computer: status,
// start, or stop there (#153). handled is false for this computer.
type RemoteMediaSetup func(ctx context.Context, nodeID, kind, method string, body []byte) (status int, raw []byte, handled bool, err error)

// BindRemoteMediaSetup lets ?node_id= send the setup routes to a paired
// computer.
func (s *Server) BindRemoteMediaSetup(fn RemoteMediaSetup) {
	s.remoteMedia = fn
}

func (s *Server) imageRoutes(api *mux.Router) {
	s.setupRoutes(api, "/images", func() *imagegen.Setup { return s.images }, "Image generation is not available.")
	s.setupRoutes(api, "/video", func() *imagegen.Setup { return s.video }, "Video generation is not available.")
}

// onPeer serves a setup route on the paired computer ?node_id= names, and
// reports whether it did.
func (s *Server) onPeer(w http.ResponseWriter, r *http.Request, kind string) bool {
	nodeID := r.URL.Query().Get("node_id")
	if nodeID == "" || s.remoteMedia == nil {
		return false
	}
	var body []byte
	if r.Body != nil && r.Method != http.MethodGet {
		body, _ = io.ReadAll(io.LimitReader(r.Body, 4096))
	}
	status, raw, handled, err := s.remoteMedia(r.Context(), nodeID, kind, r.Method, body)
	if !handled {
		if body != nil {
			r.Body = io.NopCloser(bytes.NewReader(body))
		}
		return false
	}
	if err != nil {
		writeErrFrom(w, http.StatusBadGateway, "SETUP_REFUSED", err)
		return true
	}
	if status >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &e)
		writeErr(w, status, "SETUP_REFUSED", e.Error, map[string]any{"node_id": nodeID})
		return true
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
	return true
}

// setupRoutes serves one kind's setup: status, start, stop, and removing a
// model.
func (s *Server) setupRoutes(api *mux.Router, prefix string, setup func() *imagegen.Setup, missing string) {
	bound := func(w http.ResponseWriter) *imagegen.Setup {
		st := setup()
		if st == nil {
			writeErr(w, http.StatusNotImplemented, "NOT_IMPLEMENTED", missing, nil)
		}
		return st
	}
	kind := strings.TrimPrefix(prefix, "/")
	// What is installed, the models on offer, and any setup in progress.
	// ?node_id= names a paired computer's instead (#153), for start and stop
	// too.
	api.HandleFunc(prefix+"/setup", func(w http.ResponseWriter, r *http.Request) {
		if s.onPeer(w, r, kind) {
			return
		}
		if st := bound(w); st != nil {
			writeJSON(w, http.StatusOK, st.Status())
		}
	}).Methods(http.MethodGet)
	// Install stable-diffusion.cpp and a model in the background.
	api.HandleFunc(prefix+"/setup", func(w http.ResponseWriter, r *http.Request) {
		if s.onPeer(w, r, kind) {
			return
		}
		st := bound(w)
		if st == nil {
			return
		}
		var body struct {
			ModelID string `json:"model_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "INVALID_JSON", "invalid body", nil)
			return
		}
		if err := st.Start(body.ModelID); err != nil {
			writeErrFrom(w, http.StatusConflict, "SETUP_REFUSED", err)
			return
		}
		writeJSON(w, http.StatusAccepted, st.Status())
	}).Methods(http.MethodPost)
	// Switch to the CPU build (cpu) or back to the GPU build (gpu), which
	// images and video share (#154).
	api.HandleFunc(prefix+"/setup/build", func(w http.ResponseWriter, r *http.Request) {
		st := bound(w)
		if st == nil {
			return
		}
		var body struct {
			Build string `json:"build"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeErr(w, http.StatusBadRequest, "INVALID_JSON", "invalid body", nil)
			return
		}
		if err := st.UseBuild(body.Build); err != nil {
			writeErrFrom(w, http.StatusConflict, "SETUP_REFUSED", err)
			return
		}
		writeJSON(w, http.StatusAccepted, st.Status())
	}).Methods(http.MethodPost)
	// Stop a setup; what was downloaded is kept.
	api.HandleFunc(prefix+"/setup", func(w http.ResponseWriter, r *http.Request) {
		if s.onPeer(w, r, kind) {
			return
		}
		if st := bound(w); st != nil {
			st.Cancel()
			writeJSON(w, http.StatusOK, st.Status())
		}
	}).Methods(http.MethodDelete)
	// Delete an installed model.
	api.HandleFunc(prefix+"/models/{id}", func(w http.ResponseWriter, r *http.Request) {
		st := bound(w)
		if st == nil {
			return
		}
		if err := st.Remove(mux.Vars(r)["id"]); err != nil {
			writeErrFrom(w, http.StatusConflict, "MODEL_BUSY", err)
			return
		}
		writeJSON(w, http.StatusOK, st.Status())
	}).Methods(http.MethodDelete)
}
