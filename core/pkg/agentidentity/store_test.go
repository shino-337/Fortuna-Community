package agentidentity

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func digest(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
func save(t *testing.T, path string, credentials ...Credential) {
	t.Helper()
	data, err := json.Marshal(registry{Credentials: credentials})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path+".new", data, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(path+".new", path); err != nil {
		t.Fatal(err)
	}
}
func fixture(t *testing.T) (Store, Credential, string) {
	t.Helper()
	now := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	token := strings.Repeat("a", 32)
	s := Store{Path: filepath.Join(t.TempDir(), "credentials.json"), now: func() time.Time { return now }}
	c := Credential{ID: "a-1", ClusterID: "cluster-a", AgentID: "agent-a", TokenSHA256: digest(token), ExpiresAt: now.Add(time.Hour)}
	save(t, s.Path, c)
	return s, c, token
}

func TestCredentialIdentityIsolation(t *testing.T) {
	s, c, token := fixture(t)
	p, err := s.AuthenticateToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.CheckClaims(c.ClusterID, c.AgentID); err != nil {
		t.Fatal(err)
	}
	for _, claim := range [][2]string{{"cluster-b", "agent-a"}, {"cluster-a", "agent-b"}, {"", "agent-a"}, {"cluster-a", ""}} {
		if !errors.Is(p.CheckClaims(claim[0], claim[1]), ErrIdentityMismatch) {
			t.Fatalf("accepted foreign/missing claim %v", claim)
		}
	}
	if _, err := s.AuthenticateToken(strings.Repeat("b", 32)); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal(err)
	}
	if err := (Principal{}).CheckClaims("", ""); err == nil {
		t.Fatal("empty principal accepted")
	}
}
func TestCredentialRotationRevocationAndExpiry(t *testing.T) {
	s, c, old := fixture(t)
	newToken := strings.Repeat("b", 32)
	next := c
	next.ID = "a-2"
	next.TokenSHA256 = digest(newToken)
	save(t, s.Path, c, next)
	for _, token := range []string{old, newToken} {
		if _, err := s.AuthenticateToken(token); err != nil {
			t.Fatal(err)
		}
	}
	c.Revoked = true
	save(t, s.Path, c, next)
	if _, err := s.AuthenticateToken(old); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("revoked credential accepted", err)
	}
	if _, err := s.AuthenticateToken(newToken); err != nil {
		t.Fatal(err)
	}
	next.ExpiresAt = s.clock()
	save(t, s.Path, next)
	if _, err := s.AuthenticateToken(newToken); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("expired credential accepted", err)
	}
	next.ExpiresAt = s.clock().Add(time.Hour)
	next.NotBefore = s.clock().Add(time.Minute)
	save(t, s.Path, next)
	if _, err := s.AuthenticateToken(newToken); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("future credential accepted", err)
	}
}
func TestCredentialRegistryFailsClosed(t *testing.T) {
	s, c, token := fixture(t)
	if _, err := s.AuthenticateToken(token); err != nil {
		t.Fatal(err)
	}
	for _, data := range []string{`{}`, `null`, `{"credentials":null}`, `{"credentials":[],"typo":1}`, `{"credentials":[]} {}`, `broken`, strings.Repeat(" ", (1<<20)+1)} {
		if err := os.WriteFile(s.Path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AuthenticateToken(token); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("invalid registry accepted: %v", err)
		}
	}
	duplicate := c
	duplicate.ID = "duplicate"
	duplicate.ClusterID = "cluster-b"
	save(t, s.Path, c, duplicate)
	if _, err := s.AuthenticateToken(token); !errors.Is(err, ErrUnavailable) {
		t.Fatal("ambiguous credential accepted", err)
	}
	c.CertificateSHA256 = c.TokenSHA256
	save(t, s.Path, c)
	if _, err := s.AuthenticateToken(token); !errors.Is(err, ErrUnavailable) {
		t.Fatal("mixed credential type accepted", err)
	}
	if err := os.Remove(s.Path); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AuthenticateToken(token); !errors.Is(err, ErrUnavailable) {
		t.Fatal("cached credential accepted after file removal", err)
	}
}
func TestCredentialRequiresVerifiedTLS(t *testing.T) {
	s, c, _ := fixture(t)
	now := s.clock()
	leaf := &x509.Certificate{Raw: []byte("verified-leaf"), NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour)}
	c.TokenSHA256 = ""
	c.CertificateSHA256 = digest(string(leaf.Raw))
	save(t, s.Path, c)
	state := tls.ConnectionState{HandshakeComplete: true, PeerCertificates: []*x509.Certificate{leaf}, VerifiedChains: [][]*x509.Certificate{{leaf}}}
	p, err := s.AuthenticateTLS(state)
	if err != nil || p.AgentID != c.AgentID {
		t.Fatal(p, err)
	}
	for _, invalid := range []tls.ConnectionState{
		{},
		{HandshakeComplete: true, PeerCertificates: []*x509.Certificate{nil}, VerifiedChains: [][]*x509.Certificate{{leaf}}},
		{HandshakeComplete: true, PeerCertificates: []*x509.Certificate{leaf}, VerifiedChains: [][]*x509.Certificate{{nil}}},
		{HandshakeComplete: true, PeerCertificates: []*x509.Certificate{leaf}, VerifiedChains: [][]*x509.Certificate{{{Raw: []byte("other-leaf")}}}},
	} {
		if _, err := s.AuthenticateTLS(invalid); !errors.Is(err, ErrUnauthenticated) {
			t.Fatal("untrusted TLS state accepted", err)
		}
	}
	unverified := state
	unverified.VerifiedChains = nil
	if _, err := s.AuthenticateTLS(unverified); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("unverified TLS accepted", err)
	}
	unverified = state
	unverified.HandshakeComplete = false
	if _, err := s.AuthenticateTLS(unverified); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("unfinished handshake accepted", err)
	}
	c.Revoked = true
	save(t, s.Path, c)
	if _, err := s.AuthenticateTLS(state); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("existing connection bypassed revocation", err)
	}
	c.Revoked = false
	save(t, s.Path, c)
	leaf.NotAfter = now
	if _, err := s.AuthenticateTLS(state); !errors.Is(err, ErrUnauthenticated) {
		t.Fatal("existing connection bypassed expiry", err)
	}
}
