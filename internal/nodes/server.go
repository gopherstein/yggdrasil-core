package nodes

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/yeixio/yggdrasil-core/internal/auth"
	"github.com/yeixio/yggdrasil-core/internal/config"
	"github.com/yeixio/yggdrasil-core/internal/version"
	"github.com/yeixio/yggdrasil-core/pkg/contracts"
	"github.com/yeixio/yggdrasil-core/pkg/pluginapi"
)

// InternalServer serves /internal/v1/* node RPC.
type InternalServer struct {
	cfg    config.Config
	logger *slog.Logger
	deps   InternalDeps
	http   *http.Server
}

// InternalDeps wires local services into the cluster RPC.
type InternalDeps struct {
	Config   config.Config
	Logger   *slog.Logger
	Identity *auth.NodeIdentity
	Pairing  *auth.PairingManager
	Hardware func(ctx context.Context) (contracts.HardwareInventory, error)

	ListModels     func(ctx context.Context) ([]contracts.Model, error)
	InstallModel   func(ctx context.Context, id string, wait bool) error
	InstallFromURL func(ctx context.Context, req contracts.InstallFromURLRequest, wait bool) (string, error)
	DeleteModel    func(ctx context.Context, id string) error
	ListRunning    func(ctx context.Context) ([]contracts.RunningModelView, error)
	StartModel     func(ctx context.Context, modelID string) (contracts.RunningModelView, error)
	StopModel      func(ctx context.Context, instanceID string) error
	Chat           func(ctx context.Context, req RemoteChatRequest) (<-chan pluginapi.ChatChunk, error)

	ReceiveOffer    func(offer auth.PairingOffer) (*auth.PairingSession, error)
	CompletePairing func(ctx context.Context, complete auth.PairingComplete) (*auth.PairingSession, error)
	ListIncoming    func() []auth.PairingSession
	LookupOutbound  func(code string) (*auth.PairingSession, bool)
	AdvertiseAddr   func() string

	// Training serves /internal/v1/training/ for paired computers that send
	// training runs here.
	Training http.Handler
}

func NewInternalServer(deps InternalDeps) *InternalServer {
	s := &InternalServer{
		cfg:    deps.Config,
		logger: deps.Logger,
		deps:   deps,
	}
	r := mux.NewRouter()
	api := r.PathPrefix("/internal/v1").Subrouter()
	api.Use(s.authMiddleware)

	api.HandleFunc("/health", s.handleHealth).Methods(http.MethodGet)
	api.HandleFunc("/node", s.handleNode).Methods(http.MethodGet)
	api.HandleFunc("/hardware", s.handleHardware).Methods(http.MethodGet)
	api.HandleFunc("/models", s.handleListModels).Methods(http.MethodGet)
	api.HandleFunc("/models/install", s.handleInstallModel).Methods(http.MethodPost)
	api.HandleFunc("/models/install-from-url", s.handleInstallFromURL).Methods(http.MethodPost)
	api.HandleFunc("/models/{id}", s.handleDeleteModel).Methods(http.MethodDelete)
	api.HandleFunc("/models/running", s.handleListRunning).Methods(http.MethodGet)
	api.HandleFunc("/models/{id}/start", s.handleStartModel).Methods(http.MethodPost)
	api.HandleFunc("/models/instances/{id}/stop", s.handleStopModel).Methods(http.MethodPost)
	api.HandleFunc("/chat", s.handleChat).Methods(http.MethodPost)
	api.HandleFunc("/pairing/offer", s.handlePairingOffer).Methods(http.MethodPost)
	api.HandleFunc("/pairing/complete", s.handlePairingComplete).Methods(http.MethodPost)
	api.HandleFunc("/pairing/pending", s.handlePairingPending).Methods(http.MethodGet)
	api.HandleFunc("/pairing/outbound/{code}", s.handlePairingOutbound).Methods(http.MethodGet)
	if deps.Training != nil {
		api.PathPrefix("/training/").Handler(http.StripPrefix("/internal/v1", deps.Training))
	}

	s.http = &http.Server{
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

func (s *InternalServer) ListenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.logger.Info("internal node api listening", "addr", ln.Addr().String())
	return s.http.Serve(ln)
}

func (s *InternalServer) Shutdown(ctx context.Context) error {
	if s.http == nil {
		return nil
	}
	return s.http.Shutdown(ctx)
}

func (s *InternalServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if isPublicInternalPath(path) {
			next.ServeHTTP(w, r)
			return
		}
		if s.deps.Pairing == nil {
			http.Error(w, "auth unavailable", http.StatusServiceUnavailable)
			return
		}
		authz := r.Header.Get("Authorization")
		if !strings.HasPrefix(authz, "Bearer ") {
			http.Error(w, "authorization required", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(authz, "Bearer ")
		// Peek node id from token without verify to load cert.
		nodeID, cert, err := s.lookupTokenPeer(r.Context(), token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if _, err := auth.ParseAndVerifyAuthToken(token, cert); err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		_ = nodeID
		next.ServeHTTP(w, r)
	})
}

func isPublicInternalPath(path string) bool {
	switch {
	case strings.HasSuffix(path, "/health"),
		strings.HasSuffix(path, "/node"),
		strings.HasSuffix(path, "/pairing/offer"),
		strings.HasSuffix(path, "/pairing/complete"),
		strings.HasSuffix(path, "/pairing/pending"),
		strings.Contains(path, "/pairing/outbound/"):
		return true
	default:
		return false
	}
}

func (s *InternalServer) lookupTokenPeer(ctx context.Context, token string) (string, []byte, error) {
	claimed, parseErr := auth.PeekAuthTokenNodeID(token)
	if parseErr != nil {
		return "", nil, parseErr
	}
	cert, err := s.deps.Pairing.TrustedCertPEM(ctx, claimed)
	if err != nil {
		return "", nil, err
	}
	return claimed, cert, nil
}

func (s *InternalServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, contracts.HealthResponse{Status: "ok", Product: "Yggdrasil", Version: version.Version})
}

func (s *InternalServer) handleNode(w http.ResponseWriter, r *http.Request) {
	cert := ""
	if s.deps.Identity != nil {
		cert = string(s.deps.Identity.CertPEM)
	}
	writeJSON(w, map[string]any{
		"id":       s.cfg.NodeID,
		"name":     s.cfg.NodeName,
		"cert_pem": cert,
	})
}

func (s *InternalServer) handleHardware(w http.ResponseWriter, r *http.Request) {
	if s.deps.Hardware == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	inv, err := s.deps.Hardware(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, inv)
}

func (s *InternalServer) handleListModels(w http.ResponseWriter, r *http.Request) {
	if s.deps.ListModels == nil {
		writeJSON(w, []contracts.Model{})
		return
	}
	items, err := s.deps.ListModels(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, items)
}

func (s *InternalServer) handleInstallModel(w http.ResponseWriter, r *http.Request) {
	if s.deps.InstallModel == nil {
		http.Error(w, "unavailable", http.StatusNotImplemented)
		return
	}
	var body struct {
		ModelID string `json:"model_id"`
		Wait    bool   `json:"wait"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ModelID == "" {
		http.Error(w, "model_id required", http.StatusBadRequest)
		return
	}
	if err := s.deps.InstallModel(r.Context(), body.ModelID, body.Wait); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "model_id": body.ModelID})
}

func (s *InternalServer) handleInstallFromURL(w http.ResponseWriter, r *http.Request) {
	if s.deps.InstallFromURL == nil {
		http.Error(w, "unavailable", http.StatusNotImplemented)
		return
	}
	var req contracts.InstallFromURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	id, err := s.deps.InstallFromURL(r.Context(), req, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "model_id": id})
}

func (s *InternalServer) handleDeleteModel(w http.ResponseWriter, r *http.Request) {
	if s.deps.DeleteModel == nil {
		http.Error(w, "unavailable", http.StatusNotImplemented)
		return
	}
	id := mux.Vars(r)["id"]
	if id == "" {
		http.Error(w, "model id required", http.StatusBadRequest)
		return
	}
	if err := s.deps.DeleteModel(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *InternalServer) handleListRunning(w http.ResponseWriter, r *http.Request) {
	if s.deps.ListRunning == nil {
		writeJSON(w, []contracts.RunningModelView{})
		return
	}
	items, err := s.deps.ListRunning(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, items)
}

func (s *InternalServer) handleStartModel(w http.ResponseWriter, r *http.Request) {
	if s.deps.StartModel == nil {
		http.Error(w, "unavailable", http.StatusNotImplemented)
		return
	}
	id := mux.Vars(r)["id"]
	view, err := s.deps.StartModel(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, view)
}

func (s *InternalServer) handleStopModel(w http.ResponseWriter, r *http.Request) {
	if s.deps.StopModel == nil {
		http.Error(w, "unavailable", http.StatusNotImplemented)
		return
	}
	id := mux.Vars(r)["id"]
	if err := s.deps.StopModel(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *InternalServer) handleChat(w http.ResponseWriter, r *http.Request) {
	if s.deps.Chat == nil {
		http.Error(w, "unavailable", http.StatusNotImplemented)
		return
	}
	var req RemoteChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.ModelID == "" {
		http.Error(w, "model_id required", http.StatusBadRequest)
		return
	}
	ch, err := s.deps.Chat(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	enc := json.NewEncoder(w)
	for chunk := range ch {
		if err := enc.Encode(chunk); err != nil {
			return
		}
		if flusher != nil {
			flusher.Flush()
		}
		if chunk.Done || chunk.Error != "" {
			return
		}
	}
}

func (s *InternalServer) handlePairingOffer(w http.ResponseWriter, r *http.Request) {
	if s.deps.ReceiveOffer == nil {
		http.Error(w, "unavailable", http.StatusNotImplemented)
		return
	}
	var offer auth.PairingOffer
	if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	session, err := s.deps.ReceiveOffer(offer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, session)
}

func (s *InternalServer) handlePairingComplete(w http.ResponseWriter, r *http.Request) {
	if s.deps.CompletePairing == nil {
		http.Error(w, "unavailable", http.StatusNotImplemented)
		return
	}
	var complete auth.PairingComplete
	if err := json.NewDecoder(r.Body).Decode(&complete); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	session, err := s.deps.CompletePairing(r.Context(), complete)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, session)
}

func (s *InternalServer) handlePairingPending(w http.ResponseWriter, r *http.Request) {
	if s.deps.ListIncoming == nil {
		writeJSON(w, []auth.PairingSession{})
		return
	}
	writeJSON(w, s.deps.ListIncoming())
}

func (s *InternalServer) handlePairingOutbound(w http.ResponseWriter, r *http.Request) {
	if s.deps.LookupOutbound == nil {
		http.Error(w, "unavailable", http.StatusNotImplemented)
		return
	}
	code := mux.Vars(r)["code"]
	session, ok := s.deps.LookupOutbound(code)
	if !ok || session == nil {
		http.Error(w, "unknown or expired code", http.StatusNotFound)
		return
	}
	cert := ""
	if s.deps.Identity != nil {
		cert = string(s.deps.Identity.CertPEM)
	}
	fromAddr := ""
	if s.deps.AdvertiseAddr != nil {
		fromAddr = s.deps.AdvertiseAddr()
	}
	name := s.cfg.NodeName
	if name == "" {
		name = session.LocalNodeID
	}
	writeJSON(w, auth.PairingOffer{
		SessionID:   session.ID,
		FromNodeID:  session.LocalNodeID,
		FromName:    name,
		FromCertPEM: cert,
		FromAddress: fromAddr,
		Code:        session.Code,
		ExpiresAt:   session.ExpiresAt.Format(time.RFC3339),
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
