package collection

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const RuntimeSourceHealthVersion = 1
const RuntimeSourceHealthMaxAge = time.Minute

// RuntimeSourceHealth is signed by an independently trusted sensor. It attests
// to the entire observed interval, never to Agent/file-reader liveness alone.
type RuntimeSourceHealth struct {
	Version         int       `json:"version"`
	KeyID           string    `json:"keyId"`
	ClusterID       string    `json:"clusterId"`
	AgentID         string    `json:"agentId"`
	ProducerID      string    `json:"producerId"`
	SessionID       string    `json:"sessionId"`
	SourceKind      string    `json:"sourceKind"`
	SourceSessionID string    `json:"sourceSessionId"`
	SourceStartedAt time.Time `json:"sourceStartedAt"`
	Sequence        int64     `json:"sequence"`
	WindowStart     time.Time `json:"windowStart"`
	WindowEnd       time.Time `json:"windowEnd"`
	ValidUntil      time.Time `json:"validUntil"`
	Status          string    `json:"status"`
	Dropped         uint64    `json:"dropped"`
	Errors          uint64    `json:"errors"`
	Reason          string    `json:"reason,omitempty"`
}

type SignedRuntimeSourceHealth struct {
	Report    RuntimeSourceHealth `json:"report"`
	Signature string              `json:"signature"`
}

func (r RuntimeSourceHealth) SigningBytes() ([]byte, error) {
	r.SourceStartedAt = r.SourceStartedAt.UTC().Truncate(time.Microsecond)
	r.WindowStart = r.WindowStart.UTC().Truncate(time.Microsecond)
	r.WindowEnd = r.WindowEnd.UTC().Truncate(time.Microsecond)
	r.ValidUntil = r.ValidUntil.UTC().Truncate(time.Microsecond)
	body, err := json.Marshal(r)
	return append([]byte("fortuna/runtime-source-health/v1\n"), body...), err
}

func (r RuntimeSourceHealth) Validate(now time.Time) error {
	for _, value := range []string{r.KeyID, r.ClusterID, r.AgentID, r.ProducerID, r.SessionID, r.SourceSessionID} {
		if value == "" || len(value) > 128 || strings.TrimSpace(value) != value {
			return fmt.Errorf("invalid source-health identity")
		}
	}
	source, known := RuntimeProducerSource(r.ProducerID)
	if r.Version != RuntimeSourceHealthVersion || !known || source != r.SourceKind || source == RuntimeSourceEBPF || len(r.SessionID) < 16 || len(r.SourceSessionID) < 16 || r.Sequence < 1 {
		return fmt.Errorf("unsupported source-health protocol or producer")
	}
	if r.SourceStartedAt.IsZero() || r.WindowStart.Before(r.SourceStartedAt) || !r.WindowEnd.After(r.WindowStart) || r.WindowEnd.Sub(r.WindowStart) > RuntimeSourceHealthMaxAge || r.WindowEnd.After(now) || now.Sub(r.WindowEnd) > RuntimeSourceHealthMaxAge {
		return fmt.Errorf("invalid or stale source-health interval")
	}
	if !r.ValidUntil.After(now) || r.ValidUntil.After(r.WindowEnd.Add(RuntimeSourceHealthMaxAge)) {
		return fmt.Errorf("invalid source-health lease")
	}
	if len(r.Reason) > 512 || (r.Status != "healthy" && r.Status != "failed") || (r.Status == "healthy" && (r.Dropped != 0 || r.Errors != 0)) || (r.Status == "failed" && strings.TrimSpace(r.Reason) == "") {
		return fmt.Errorf("invalid source-health status")
	}
	return nil
}
