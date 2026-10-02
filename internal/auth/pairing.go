package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/yeixio/yggdrasil-core/pkg/contracts"
)

// PairingState represents pairing session states.
type PairingState string

const (
	PairingPending  PairingState = "pending"
	PairingApproved PairingState = "approved"
	PairingRejected PairingState = "rejected"
	PairingExpired  PairingState = "expired"
)

// PairingSession is an in-flight pairing attempt.
type PairingSession struct {
	ID           string       `json:"id"`
	LocalNodeID  string       `json:"local_node_id"`
	RemoteNodeID string       `json:"remote_node_id"`
	RemoteName   string       `json:"remote_name"`
	RemoteAddr   string       `json:"remote_address,omitempty"`
	Code         string       `json:"code"`
	State        PairingState `json:"state"`
	RemoteCert   []byte       `json:"-"`
	CreatedAt    time.Time    `json:"created_at"`
	ExpiresAt    time.Time    `json:"expires_at"`
	Incoming     bool         `json:"incoming,omitempty"`
}

// PairingOffer is sent from initiator to the peer over Bifrost.
type PairingOffer struct {
	SessionID   string `json:"session_id"`
	FromNodeID  string `json:"from_node_id"`
	FromName    string `json:"from_name"`
	FromCertPEM string `json:"from_cert_pem"`
	FromAddress string `json:"from_address,omitempty"`
	Code        string `json:"code"`
	ExpiresAt   string `json:"expires_at"`
}

// PairingComplete finishes mutual trust on the initiator.
type PairingComplete struct {
	SessionID   string `json:"session_id"`
	FromNodeID  string `json:"from_node_id"`
	FromName    string `json:"from_name"`
	FromCertPEM string `json:"from_cert_pem"`
	FromAddress string `json:"from_address,omitempty"`
}

// PairingManager implements the pairing state machine.
type PairingManager struct {
	db        *sql.DB
	identity  *NodeIdentity
	mu        sync.Mutex
	sessions  map[string]*PairingSession
	codeIndex map[string]string // code -> session id
	incoming  map[string]*PairingSession
}

func NewPairingManager(db *sql.DB, identity *NodeIdentity) *PairingManager {
	return &PairingManager{
		db:        db,
		identity:  identity,
		sessions:  make(map[string]*PairingSession),
		codeIndex: make(map[string]string),
		incoming:  make(map[string]*PairingSession),
	}
}

// Identity returns the local node identity.
func (p *PairingManager) Identity() *NodeIdentity { return p.identity }

// StartPairing initiates pairing with a discovered node.
func (p *PairingManager) StartPairing(remoteNodeID, remoteName, remoteAddr string, remoteCert []byte) (*PairingSession, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	code, err := shortCode()
	if err != nil {
		return nil, err
	}
	s := &PairingSession{
		ID:           randomID(),
		LocalNodeID:  p.identity.NodeID,
		RemoteNodeID: remoteNodeID,
		RemoteName:   remoteName,
		RemoteAddr:   remoteAddr,
		Code:         code,
		State:        PairingPending,
		RemoteCert:   append([]byte(nil), remoteCert...),
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    time.Now().UTC().Add(10 * time.Minute),
	}
	p.sessions[s.ID] = s
	p.codeIndex[code] = s.ID
	return cloneSession(s), nil
}

// ReceiveOffer stores an incoming pairing offer from a peer.
func (p *PairingManager) ReceiveOffer(offer PairingOffer) (*PairingSession, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	exp, _ := time.Parse(time.RFC3339, offer.ExpiresAt)
	if exp.IsZero() {
		exp = time.Now().UTC().Add(10 * time.Minute)
	}
	if time.Now().After(exp) {
		return nil, fmt.Errorf("pairing offer expired")
	}
	s := &PairingSession{
		ID:           offer.SessionID,
		LocalNodeID:  p.identity.NodeID,
		RemoteNodeID: offer.FromNodeID,
		RemoteName:   offer.FromName,
		RemoteAddr:   offer.FromAddress,
		Code:         offer.Code,
		State:        PairingPending,
		RemoteCert:   []byte(offer.FromCertPEM),
		CreatedAt:    time.Now().UTC(),
		ExpiresAt:    exp,
		Incoming:     true,
	}
	p.incoming[s.ID] = s
	p.codeIndex[s.Code] = s.ID
	return cloneSession(s), nil
}

// ListIncoming returns pending incoming offers.
func (p *PairingManager) ListIncoming() []PairingSession {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]PairingSession, 0, len(p.incoming))
	now := time.Now()
	for id, s := range p.incoming {
		if s.State != PairingPending || now.After(s.ExpiresAt) {
			delete(p.incoming, id)
			continue
		}
		out = append(out, *cloneSession(s))
	}
	return out
}

// ApproveByCode approves pairing when codes match (incoming or local).
func (p *PairingManager) ApproveByCode(ctx context.Context, code string) (*PairingSession, error) {
	p.mu.Lock()
	sessionID, ok := p.codeIndex[code]
	if !ok {
		p.mu.Unlock()
		return nil, fmt.Errorf("invalid or expired pairing code")
	}
	s := p.sessions[sessionID]
	if s == nil {
		s = p.incoming[sessionID]
	}
	if s == nil || s.State != PairingPending || time.Now().After(s.ExpiresAt) {
		p.mu.Unlock()
		return nil, fmt.Errorf("pairing session expired")
	}
	s.State = PairingApproved
	cp := cloneSession(s)
	p.mu.Unlock()

	if err := p.storeTrust(ctx, cp.RemoteNodeID, cp.RemoteName, cp.RemoteAddr, cp.RemoteCert); err != nil {
		return nil, err
	}
	return cp, nil
}

// ApproveSession approves by session ID (local UI / incoming).
func (p *PairingManager) ApproveSession(ctx context.Context, sessionID string) (*PairingSession, error) {
	p.mu.Lock()
	s := p.sessions[sessionID]
	if s == nil {
		s = p.incoming[sessionID]
	}
	if s == nil || s.State != PairingPending {
		p.mu.Unlock()
		return nil, fmt.Errorf("session not found")
	}
	s.State = PairingApproved
	cp := cloneSession(s)
	p.mu.Unlock()
	if err := p.storeTrust(ctx, cp.RemoteNodeID, cp.RemoteName, cp.RemoteAddr, cp.RemoteCert); err != nil {
		return nil, err
	}
	return cp, nil
}

// CompleteFromPeer finalizes initiator trust after peer approval.
func (p *PairingManager) CompleteFromPeer(ctx context.Context, complete PairingComplete) (*PairingSession, error) {
	p.mu.Lock()
	s := p.sessions[complete.SessionID]
	if s == nil {
		p.mu.Unlock()
		return nil, fmt.Errorf("session not found")
	}
	s.State = PairingApproved
	s.RemoteCert = []byte(complete.FromCertPEM)
	s.RemoteName = complete.FromName
	if complete.FromAddress != "" {
		s.RemoteAddr = complete.FromAddress
	}
	cp := cloneSession(s)
	p.mu.Unlock()
	if err := p.storeTrust(ctx, complete.FromNodeID, complete.FromName, complete.FromAddress, []byte(complete.FromCertPEM)); err != nil {
		return nil, err
	}
	return cp, nil
}

// Reject cancels a pairing session.
func (p *PairingManager) Reject(sessionID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	s := p.sessions[sessionID]
	if s == nil {
		s = p.incoming[sessionID]
	}
	if s == nil {
		return fmt.Errorf("session not found")
	}
	s.State = PairingRejected
	delete(p.codeIndex, s.Code)
	delete(p.incoming, sessionID)
	return nil
}

// GetOutboundByCode returns a pending outbound session for peer pull/claim.
func (p *PairingManager) GetOutboundByCode(code string) (*PairingSession, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	id, ok := p.codeIndex[code]
	if !ok {
		return nil, false
	}
	s := p.sessions[id]
	if s == nil || s.Incoming || s.State != PairingPending || time.Now().After(s.ExpiresAt) {
		return nil, false
	}
	return cloneSession(s), true
}

// GetSession returns a local outbound session.
func (p *PairingManager) GetSession(sessionID string) (*PairingSession, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, ok := p.sessions[sessionID]
	if !ok {
		return nil, false
	}
	return cloneSession(s), true
}

// GetIncoming returns an incoming session.
func (p *PairingManager) GetIncoming(sessionID string) (*PairingSession, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	s, ok := p.incoming[sessionID]
	if !ok {
		return nil, false
	}
	return cloneSession(s), true
}

// RevokeTrust removes trust for a node.
func (p *PairingManager) RevokeTrust(ctx context.Context, nodeID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := p.db.ExecContext(ctx, `UPDATE node_trust SET revoked_at = ? WHERE node_id = ?`, now, nodeID)
	return err
}

// TrustedCertPEM returns the stored cert for a paired node.
func (p *PairingManager) TrustedCertPEM(ctx context.Context, nodeID string) ([]byte, error) {
	var pem string
	err := p.db.QueryRowContext(ctx, `
		SELECT cert_pem FROM node_trust WHERE node_id = ? AND revoked_at IS NULL`, nodeID).Scan(&pem)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("node %q not trusted", nodeID)
	}
	if err != nil {
		return nil, err
	}
	return []byte(pem), nil
}

// IsTrusted reports whether nodeID is paired and not revoked.
func (p *PairingManager) IsTrusted(ctx context.Context, nodeID string) bool {
	_, err := p.TrustedCertPEM(ctx, nodeID)
	return err == nil
}

// Trust pairs a computer whose key was verified another way, such as by a
// one-line join token (#40).
func (p *PairingManager) Trust(ctx context.Context, nodeID, name, address string, cert []byte) error {
	return p.storeTrust(ctx, nodeID, name, address, cert)
}

func (p *PairingManager) storeTrust(ctx context.Context, nodeID, name, address string, cert []byte) error {
	fp := Fingerprint(string(cert))
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO nodes (id, name, status, is_local, address)
		VALUES (?, ?, 'online', 0, ?)
		ON CONFLICT(id) DO UPDATE SET name=excluded.name, status='online', address=COALESCE(NULLIF(excluded.address,''), nodes.address), updated_at=datetime('now')`,
		nodeID, name, address)
	if err != nil {
		return err
	}
	_, err = p.db.ExecContext(ctx, `
		INSERT INTO node_trust (node_id, fingerprint, cert_pem, paired_at)
		VALUES (?, ?, ?, datetime('now'))
		ON CONFLICT(node_id) DO UPDATE SET fingerprint=excluded.fingerprint, cert_pem=excluded.cert_pem, revoked_at=NULL, paired_at=datetime('now')`,
		nodeID, fp, string(cert))
	return err
}

// SetNodeAddress updates a paired node's reachable address.
func (p *PairingManager) SetNodeAddress(ctx context.Context, nodeID, address string) error {
	_, err := p.db.ExecContext(ctx, `UPDATE nodes SET address = ?, updated_at=datetime('now') WHERE id = ?`, address, nodeID)
	return err
}

// SetNodeStatus persists online/offline (and similar) for a paired node.
func (p *PairingManager) SetNodeStatus(ctx context.Context, nodeID, status string) error {
	_, err := p.db.ExecContext(ctx, `
		UPDATE nodes SET status = ?, last_seen_at = CASE WHEN ? = 'online' THEN datetime('now') ELSE last_seen_at END, updated_at=datetime('now')
		WHERE id = ?`, status, status, nodeID)
	return err
}

// SetNodeHardware stores the latest hardware snapshot for a paired node.
func (p *PairingManager) SetNodeHardware(ctx context.Context, nodeID string, inv contracts.HardwareInventory) error {
	raw, err := json.Marshal(inv)
	if err != nil {
		return err
	}
	_, err = p.db.ExecContext(ctx, `
		UPDATE nodes SET hardware_json = ?, os = ?, arch = ?, updated_at=datetime('now')
		WHERE id = ?`, string(raw), inv.OS, inv.Arch, nodeID)
	return err
}

func cloneSession(s *PairingSession) *PairingSession {
	if s == nil {
		return nil
	}
	cp := *s
	cp.RemoteCert = append([]byte(nil), s.RemoteCert...)
	return &cp
}

func shortCode() (string, error) {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	n := int(b[0])<<16 | int(b[1])<<8 | int(b[2])
	n = n % 1000000
	return fmt.Sprintf("%06d", n), nil
}

func randomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
