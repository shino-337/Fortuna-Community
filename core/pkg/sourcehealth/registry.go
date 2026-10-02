package sourcehealth

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fortuna/api/collection"
)

type Key struct {
	ID         string    `json:"id"`
	ClusterID  string    `json:"clusterId"`
	AgentID    string    `json:"agentId"`
	ProducerID string    `json:"producerId"`
	PublicKey  string    `json:"publicKey"`
	NotBefore  time.Time `json:"notBefore"`
	ExpiresAt  time.Time `json:"expiresAt"`
	Revoked    bool      `json:"revoked"`
}

type Registry struct {
	Version int   `json:"version"`
	Keys    []Key `json:"keys"`
}

// A separate operator-managed public-key registry prevents an Agent credential
// from granting itself sensor authority. Reload on every request for rotation.
func Verify(path string, signed collection.SignedRuntimeSourceHealth, now time.Time) error {
	if path == "" {
		return fmt.Errorf("source-health trust registry is not configured")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || st.Size() > 1<<20 {
		return fmt.Errorf("invalid source-health registry size/type")
	}
	decoder := json.NewDecoder(io.LimitReader(f, (1<<20)+1))
	decoder.DisallowUnknownFields()
	var registry Registry
	if err := decoder.Decode(&registry); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("trailing source-health registry data")
	}
	if registry.Version != 1 || len(registry.Keys) == 0 || len(registry.Keys) > 1024 {
		return fmt.Errorf("invalid source-health registry")
	}
	seen := map[string]bool{}
	var match *Key
	for i := range registry.Keys {
		key := &registry.Keys[i]
		for _, id := range []string{key.ID, key.ClusterID, key.AgentID, key.ProducerID} {
			if id == "" || strings.TrimSpace(id) != id {
				return fmt.Errorf("invalid source-health key identity")
			}
		}
		public, err := base64.StdEncoding.DecodeString(key.PublicKey)
		if err != nil || len(public) != ed25519.PublicKeySize || seen[key.ID] || key.NotBefore.IsZero() || !key.ExpiresAt.After(key.NotBefore) {
			return fmt.Errorf("invalid source-health key")
		}
		seen[key.ID] = true
		if key.ID == signed.Report.KeyID {
			match = key
		}
	}
	r := signed.Report
	if match == nil || match.Revoked || now.Before(match.NotBefore) || !now.Before(match.ExpiresAt) || r.ValidUntil.After(match.ExpiresAt) || r.WindowStart.Before(match.NotBefore) || r.ClusterID != match.ClusterID || r.AgentID != match.AgentID || r.ProducerID != match.ProducerID {
		return fmt.Errorf("untrusted source-health principal/key")
	}
	public, _ := base64.StdEncoding.DecodeString(match.PublicKey)
	signature, err := base64.StdEncoding.DecodeString(signed.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("invalid source-health signature")
	}
	payload, err := r.SigningBytes()
	if err != nil || !ed25519.Verify(public, payload, signature) {
		return fmt.Errorf("invalid source-health signature")
	}
	return nil
}
