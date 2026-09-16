// Package agentidentity authenticates explicitly provisioned agent credentials.
// Transport adapters must enforce the returned identity before writing data.
package agentidentity

import (
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"time"
)

var (
	ErrUnavailable      = errors.New("agent credential registry unavailable")
	ErrUnauthenticated  = errors.New("agent credential rejected")
	ErrIdentityMismatch = errors.New("agent identity mismatch")
)

// Principal is resolved from trusted configuration, never request metadata.
type Principal struct{ CredentialID, ClusterID, AgentID string }

func (p Principal) CheckClaims(clusterID, agentID string) error {
	if p.CredentialID == "" || p.ClusterID == "" || p.AgentID == "" || clusterID != p.ClusterID || agentID != p.AgentID {
		return ErrIdentityMismatch
	}
	return nil
}

type Credential struct {
	ID                string    `json:"id"`
	ClusterID         string    `json:"cluster_id"`
	AgentID           string    `json:"agent_id"`
	TokenSHA256       string    `json:"token_sha256,omitempty"`
	CertificateSHA256 string    `json:"certificate_sha256,omitempty"`
	NotBefore         time.Time `json:"not_before,omitempty"`
	ExpiresAt         time.Time `json:"expires_at"`
	Revoked           bool      `json:"revoked,omitempty"`
}

type registry struct {
	Credentials []Credential `json:"credentials"`
}

// Store reloads for every authentication. Atomic file replacement permits rotation
// and revocation without process restart. Unreadable/invalid configuration fails
// closed; no last-known-good credential cache is used.
type Store struct {
	Path string
	now  func() time.Time
}

func (s Store) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

func (s Store) load() ([]Credential, error) {
	f, err := os.Open(s.Path)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return nil, ErrUnavailable
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	var r registry
	if err := dec.Decode(&r); err != nil || r.Credentials == nil {
		return nil, ErrUnavailable
	}
	var extra any
	if dec.Decode(&extra) != io.EOF {
		return nil, ErrUnavailable
	}
	ids := map[string]bool{}
	digests := map[string]bool{}
	for _, c := range r.Credentials {
		if c.ID == "" || c.ClusterID == "" || c.AgentID == "" || strings.TrimSpace(c.ID) != c.ID || strings.TrimSpace(c.ClusterID) != c.ClusterID || strings.TrimSpace(c.AgentID) != c.AgentID || ids[c.ID] || c.ExpiresAt.IsZero() || (!c.NotBefore.IsZero() && !c.NotBefore.Before(c.ExpiresAt)) {
			return nil, ErrUnavailable
		}
		ids[c.ID] = true
		if (c.TokenSHA256 == "") == (c.CertificateSHA256 == "") {
			return nil, ErrUnavailable
		}
		digest := c.TokenSHA256
		kind := "token:"
		if digest == "" {
			digest = c.CertificateSHA256
			kind = "certificate:"
		}
		decoded, err := hex.DecodeString(digest)
		if err != nil || len(decoded) != sha256.Size || hex.EncodeToString(decoded) != digest || digests[kind+digest] {
			return nil, ErrUnavailable
		}
		digests[kind+digest] = true
	}
	return r.Credentials, nil
}

func (s Store) authenticate(digest string, certificate bool) (Principal, error) {
	credentials, err := s.load()
	if err != nil {
		return Principal{}, err
	}
	now := s.clock()
	for _, c := range credentials {
		expected := c.TokenSHA256
		if certificate {
			expected = c.CertificateSHA256
		}
		if subtle.ConstantTimeCompare([]byte(expected), []byte(digest)) != 1 || c.Revoked || now.Before(c.NotBefore) || !now.Before(c.ExpiresAt) {
			continue
		}
		return Principal{CredentialID: c.ID, ClusterID: c.ClusterID, AgentID: c.AgentID}, nil
	}
	return Principal{}, ErrUnauthenticated
}

func (s Store) AuthenticateToken(token string) (Principal, error) {
	if len(token) < 32 || len(token) > 4096 {
		return Principal{}, ErrUnauthenticated
	}
	hash := sha256.Sum256([]byte(token))
	return s.authenticate(hex.EncodeToString(hash[:]), false)
}

// AuthenticateTLS requires a completed, verified mTLS handshake. A certificate
// fingerprint alone is not authentication. Recheck leaf expiry on every call,
// including stream messages, because the connection can outlive the certificate.
func (s Store) AuthenticateTLS(state tls.ConnectionState) (Principal, error) {
	if !state.HandshakeComplete || len(state.PeerCertificates) == 0 || len(state.VerifiedChains) == 0 {
		return Principal{}, ErrUnauthenticated
	}
	leaf := state.PeerCertificates[0]
	if leaf == nil {
		return Principal{}, ErrUnauthenticated
	}
	verified := false
	for _, chain := range state.VerifiedChains {
		if len(chain) > 0 && chain[0] != nil && chain[0].Equal(leaf) {
			verified = true
			break
		}
	}
	now := s.clock()
	if !verified || now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
		return Principal{}, ErrUnauthenticated
	}
	hash := sha256.Sum256(leaf.Raw)
	return s.authenticate(hex.EncodeToString(hash[:]), true)
}
