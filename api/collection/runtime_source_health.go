package collection

import (
	"fmt"
	"strings"
	"time"
)

const RuntimeSourceHealthVersion = 1
const RuntimeSourceHealthLeaseMaxAge = time.Minute
const RuntimeSourceHealthReportInterval = 20 * time.Second

const (
	RuntimeSourceHealthHealthy   = "healthy"
	RuntimeSourceHealthUnhealthy = "unhealthy"

	RuntimeSourceHealthProofFalcoHTTP = "falco-health-http"
)

type RuntimeSourceHealthReport struct {
	Version    int       `json:"version"`
	ProducerID string    `json:"producerId"`
	SourceKind string    `json:"sourceKind"`
	SessionID  string    `json:"sessionId"`
	Status     string    `json:"status"`
	ProofKind  string    `json:"proofKind"`
	ObservedAt time.Time `json:"observedAt"`
	ValidUntil time.Time `json:"validUntil"`
	Reason     string    `json:"reason,omitempty"`
}

func (r RuntimeSourceHealthReport) Validate(now time.Time) error {
	if r.Version != RuntimeSourceHealthVersion {
		return fmt.Errorf("invalid runtime source-health version")
	}
	if len(r.SessionID) < 16 || len(r.SessionID) > 128 || strings.TrimSpace(r.SessionID) != r.SessionID {
		return fmt.Errorf("invalid runtime source-health session")
	}
	expected, ok := RuntimeProducerSource(r.ProducerID)
	if !ok || expected != r.SourceKind {
		return fmt.Errorf("invalid runtime source-health producer")
	}
	if r.Status != RuntimeSourceHealthHealthy && r.Status != RuntimeSourceHealthUnhealthy {
		return fmt.Errorf("invalid runtime source-health status")
	}
	if r.ObservedAt.IsZero() || r.ValidUntil.IsZero() || !r.ValidUntil.After(r.ObservedAt) ||
		r.ValidUntil.Sub(r.ObservedAt) > RuntimeSourceHealthLeaseMaxAge ||
		r.ObservedAt.After(now.Add(time.Minute)) ||
		r.ObservedAt.Before(now.Add(-2*RuntimeSourceHealthLeaseMaxAge)) {
		return fmt.Errorf("invalid runtime source-health window")
	}
	if r.Status == RuntimeSourceHealthHealthy {
		if r.ProducerID != "falco" || r.SourceKind != RuntimeSourceFalco ||
			r.ProofKind != RuntimeSourceHealthProofFalcoHTTP {
			return fmt.Errorf("runtime source-health proof is not authoritative for this producer")
		}
		if strings.TrimSpace(r.Reason) != "" {
			return fmt.Errorf("healthy runtime source-health report cannot carry a reason")
		}
	} else {
		if strings.TrimSpace(r.Reason) == "" {
			return fmt.Errorf("unhealthy runtime source-health report requires a reason")
		}
	}
	return nil
}
