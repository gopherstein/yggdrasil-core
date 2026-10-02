package nodes

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/yeixio/yggdrasil-core/internal/auth"
	"github.com/yeixio/yggdrasil-core/internal/config"
	"github.com/yeixio/yggdrasil-core/internal/store"
	"github.com/yeixio/yggdrasil-core/pkg/contracts"
	"github.com/yeixio/yggdrasil-core/pkg/pluginapi"
)

func setupInternalServer(t *testing.T) (*InternalServer, *auth.NodeIdentity, *auth.PairingManager, *auth.NodeIdentity) {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	local, err := auth.LoadOrCreateIdentity(auth.NewSecretStore(dir+"/local"), "local-node")
	if err != nil {
		t.Fatal(err)
	}
	peer, err := auth.LoadOrCreateIdentity(auth.NewSecretStore(dir+"/peer"), "peer-node")
	if err != nil {
		t.Fatal(err)
	}
	pm := auth.NewPairingManager(db.SQL, local)
	cfg := config.Config{NodeID: "local-node", NodeName: "Local"}

	srv := NewInternalServer(InternalDeps{
		Config:   cfg,
		Identity: local,
		Pairing:  pm,
		ListModels: func(ctx context.Context) ([]contracts.Model, error) {
			return []contracts.Model{{ID: "m1", Installed: true}}, nil
		},
		Chat: func(ctx context.Context, req RemoteChatRequest) (<-chan pluginapi.ChatChunk, error) {
			ch := make(chan pluginapi.ChatChunk, 1)
			ch <- pluginapi.ChatChunk{Content: "hi", Done: true}
			close(ch)
			return ch, nil
		},
		Tools: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("tools " + r.URL.Path))
		}),
	})
	return srv, local, pm, peer
}

func TestAuthMiddlewareRejectsUnsigned(t *testing.T) {
	srv, _, _, _ := setupInternalServer(t)
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/models", nil)
	rr := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthMiddlewareRejectsWrongPeer(t *testing.T) {
	srv, _, pm, peer := setupInternalServer(t)
	// Trust a different node, not the peer signing the request.
	otherDir := t.TempDir()
	other, err := auth.LoadOrCreateIdentity(auth.NewSecretStore(otherDir), "other-node")
	if err != nil {
		t.Fatal(err)
	}
	if err := storeTrustHelper(t, pm, other); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+peer.AuthToken())
	rr := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthMiddlewareAcceptsTrustedPeer(t *testing.T) {
	srv, _, pm, peer := setupInternalServer(t)
	if err := storeTrustHelper(t, pm, peer); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+peer.AuthToken())
	rr := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestAuthMiddlewareAllowsPublicRoutes(t *testing.T) {
	srv, _, _, _ := setupInternalServer(t)
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/health", nil)
	rr := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d", rr.Code)
	}
}

func TestAuthMiddlewareRejectsRevoked(t *testing.T) {
	srv, _, pm, peer := setupInternalServer(t)
	if err := storeTrustHelper(t, pm, peer); err != nil {
		t.Fatal(err)
	}
	if err := pm.RevokeTrust(context.Background(), peer.NodeID); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/internal/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+peer.AuthToken())
	rr := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestPairingOfferCompleteMutualTrust(t *testing.T) {
	dir := t.TempDir()
	dbA, err := store.Open(filepath.Join(dir, "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	dbB, err := store.Open(filepath.Join(dir, "b.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = dbA.Close()
		_ = dbB.Close()
	})

	idA, err := auth.LoadOrCreateIdentity(auth.NewSecretStore(dir+"/a"), "node-a")
	if err != nil {
		t.Fatal(err)
	}
	idB, err := auth.LoadOrCreateIdentity(auth.NewSecretStore(dir+"/b"), "node-b")
	if err != nil {
		t.Fatal(err)
	}
	pmA := auth.NewPairingManager(dbA.SQL, idA)
	pmB := auth.NewPairingManager(dbB.SQL, idB)

	session, err := pmA.StartPairing("node-b", "B", "127.0.0.1:7332", idB.CertPEM)
	if err != nil {
		t.Fatal(err)
	}
	offer := auth.PairingOffer{
		SessionID:   session.ID,
		FromNodeID:  idA.NodeID,
		FromName:    "A",
		FromCertPEM: string(idA.CertPEM),
		FromAddress: "127.0.0.1:7333",
		Code:        session.Code,
		ExpiresAt:   session.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if _, err := pmB.ReceiveOffer(offer); err != nil {
		t.Fatal(err)
	}
	approved, err := pmB.ApproveSession(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !pmB.IsTrusted(context.Background(), "node-a") {
		t.Fatal("B should trust A")
	}
	complete := auth.PairingComplete{
		SessionID:   approved.ID,
		FromNodeID:  idB.NodeID,
		FromName:    "B",
		FromCertPEM: string(idB.CertPEM),
		FromAddress: "127.0.0.1:7332",
	}
	if _, err := pmA.CompleteFromPeer(context.Background(), complete); err != nil {
		t.Fatal(err)
	}
	if !pmA.IsTrusted(context.Background(), "node-b") {
		t.Fatal("A should trust B")
	}
}

func TestRemoteChatStream(t *testing.T) {
	srv, _, pm, peer := setupInternalServer(t)
	if err := storeTrustHelper(t, pm, peer); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(RemoteChatRequest{
		ModelID:  "m1",
		Messages: []pluginapi.ChatMessage{{Role: "user", Content: "hi"}},
	})
	req := httptest.NewRequest(http.MethodPost, "/internal/v1/chat", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+peer.AuthToken())
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	var chunk pluginapi.ChatChunk
	if err := json.NewDecoder(rr.Body).Decode(&chunk); err != nil {
		t.Fatal(err)
	}
	if chunk.Content != "hi" || !chunk.Done {
		t.Fatalf("chunk=%+v", chunk)
	}
}

func storeTrustHelper(t *testing.T, pm *auth.PairingManager, peer *auth.NodeIdentity) error {
	t.Helper()
	s, err := pm.StartPairing(peer.NodeID, "Peer", "127.0.0.1:9", peer.CertPEM)
	if err != nil {
		return err
	}
	_, err = pm.ApproveByCode(context.Background(), s.Code)
	return err
}

// Manual two-machine checklist:
// 1. Enable discovery on A and B; InternalHost should be 0.0.0.0.
// 2. A discovers B with LAN host:port (not 127.0.0.1).
// 3. A pairs → B shows incoming offer → Approve; both trust each other.
// 4. Install a model only on B; pin Team worker→B (Profiles Edit roles).
// 5. Chat on A with Programming (Team) profile; timeline shows worker on B.

// Paired computers reach the remote tool protocol; others are refused.
func TestToolsRouteNeedsAPairedComputer(t *testing.T) {
	srv, _, pm, peer := setupInternalServer(t)
	call := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/internal/v1/tools/providers", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rr := httptest.NewRecorder()
		srv.http.Handler.ServeHTTP(rr, req)
		return rr
	}
	if rr := call(""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("unsigned: %d", rr.Code)
	}
	if rr := call(peer.AuthToken()); rr.Code == http.StatusOK {
		t.Fatal("an unpaired computer reached the tools")
	}
	if err := storeTrustHelper(t, pm, peer); err != nil {
		t.Fatal(err)
	}
	if rr := call(peer.AuthToken()); rr.Code != http.StatusOK || rr.Body.String() != "tools /tools/providers" {
		t.Fatalf("paired: %d %q", rr.Code, rr.Body.String())
	}
}
