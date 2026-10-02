package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fortuna/agent/internal/corehttp"
	"github.com/fortuna/api/collection"
)

// SourceHealthRelay forwards sensor-signed reports. It owns no signing key and
// cannot turn a reader heartbeat or an empty input file into source authority.
// One instance is consumed by a single polling goroutine.
type SourceHealthRelay struct {
	CoreURL, Path, ClusterID, AgentID, SessionID string
	Client                                       *http.Client
	accepted                                     map[string][32]byte
	retryAt                                      time.Time
}

func WriteSourceHealthChallenge(path, clusterID, agentID, sessionID string, started time.Time) error {
	if path == "" {
		return nil
	}
	body, err := json.Marshal(struct {
		ClusterID string    `json:"clusterId"`
		AgentID   string    `json:"agentId"`
		SessionID string    `json:"sessionId"`
		StartedAt time.Time `json:"sessionStartedAt"`
	}{clusterID, agentID, sessionID, started.UTC()})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".health-challenge-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(body); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func (r *SourceHealthRelay) Poll(ctx context.Context) error {
	if r.Path == "" || time.Now().Before(r.retryAt) {
		return nil
	}
	f, err := os.Open(r.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || st.Size() > 65536 {
		return fmt.Errorf("invalid signed health file")
	}
	decoder := json.NewDecoder(io.LimitReader(f, 65537))
	decoder.DisallowUnknownFields()
	var reports []collection.SignedRuntimeSourceHealth
	if err = decoder.Decode(&reports); err != nil {
		return err
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("trailing signed health data")
	}
	if len(reports) > 2 {
		return fmt.Errorf("too many source health reports")
	}
	seen := map[string]bool{}
	// Validate the whole file before posting any report. Restart requires a fresh
	// signature bound to the challenge's Agent session.
	for _, signed := range reports {
		h := signed.Report
		if err = h.Validate(time.Now().UTC()); err != nil {
			return err
		}
		if h.ClusterID != r.ClusterID || h.AgentID != r.AgentID || h.SessionID != r.SessionID || seen[h.ProducerID] {
			return fmt.Errorf("source health session/identity mismatch")
		}
		seen[h.ProducerID] = true
	}
	if r.Client == nil {
		r.Client = &http.Client{Timeout: 10 * time.Second}
	}
	if r.accepted == nil {
		r.accepted = map[string][32]byte{}
	}
	for _, signed := range reports {
		body, err := json.Marshal(signed)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(body)
		if r.accepted[signed.Report.ProducerID] == hash {
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(r.CoreURL, "/")+"/api/v2/runtime/source-health", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		corehttp.ApplyOptionalAuthorization(req)
		resp, err := r.Client.Do(req)
		if err != nil {
			r.retryAt = time.Now().Add(corehttp.DeliveryBackoff)
			return err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			postErr := corehttp.DecodePostError(resp, time.Now())
			resp.Body.Close()
			r.retryAt = time.Now().Add(postErr.RetryAfter)
			return postErr
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		r.accepted[signed.Report.ProducerID] = hash
	}
	return nil
}
func (r *SourceHealthRelay) Start(ctx context.Context) {
	if r.Path == "" {
		return
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		_ = r.Poll(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
