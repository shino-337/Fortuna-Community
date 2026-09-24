package collection

import (
	"fmt"
	"strings"
	"time"
)

const RuntimeSourceHealthVersion = 1

// RuntimeSourceHealthMaxGap bounds the interval over which consecutive healthy
// upstream probes may be treated as one continuous source-health observation.
// It is intentionally shorter than the producer lifecycle lease.
const RuntimeSourceHealthMaxGap = 45 * time.Second

const (
	RuntimeSourceHealthHealthy   = "healthy"
	RuntimeSourceHealthUnhealthy = "unhealthy"

	RuntimeSourceHealthProbeFalcoK8sReadiness = "falco-k8s-readiness"
)

var runtimeSourceHealthProofs = map[string]map[string]bool{
	// Falco authority may only come from a separate upstream readiness probe.
	// D3a defines and validates the contract; D3b wires the Agent probe.
	"falco": {
		RuntimeSourceHealthProbeFalcoK8sReadiness: true,
	},
}

type RuntimeSourceHealth struct {
	Version          int       `json:"version"`
	ID               string    `json:"id"`
	ProducerID       string    `json:"producerId"`
	SourceKind       string    `json:"sourceKind"`
	SessionID        string    `json:"sessionId"`
	ProbeKind        string    `json:"probeKind"`
	SourceInstanceID string    `json:"sourceInstanceId,omitempty"`
	Status           string    `json:"status"`
	ObservedAt       time.Time `json:"observedAt"`
	Reason           string    `json:"reason,omitempty"`
}

func (h RuntimeSourceHealth) Validate(now time.Time) error {
	if h.Version != RuntimeSourceHealthVersion {
		return fmt.Errorf("unsupported runtime source health version")
	}
	if len(h.ID) < 16 || len(h.ID) > 128 || strings.TrimSpace(h.ID) != h.ID {
		return fmt.Errorf("invalid runtime source health id")
	}
	if len(h.SessionID) < 16 || len(h.SessionID) > 128 || strings.TrimSpace(h.SessionID) != h.SessionID {
		return fmt.Errorf("invalid runtime source health session")
	}
	expected, ok := RuntimeProducerSource(h.ProducerID)
	if !ok || expected != h.SourceKind {
		return fmt.Errorf("invalid runtime source health producer/source")
	}
	proofs := runtimeSourceHealthProofs[h.ProducerID]
	if proofs == nil || !proofs[h.ProbeKind] {
		return fmt.Errorf("unsupported runtime source health proof")
	}
	if h.Status != RuntimeSourceHealthHealthy && h.Status != RuntimeSourceHealthUnhealthy {
		return fmt.Errorf("invalid runtime source health status")
	}
	if h.ObservedAt.IsZero() || h.ObservedAt.After(now.Add(time.Minute)) ||
		h.ObservedAt.Before(now.Add(-2*RuntimeProducerLeaseMaxAge)) {
		return fmt.Errorf("invalid runtime source health observation time")
	}
	instance := strings.TrimSpace(h.SourceInstanceID)
	if h.Status == RuntimeSourceHealthHealthy {
		if len(instance) < 8 || len(instance) > 255 || instance != h.SourceInstanceID {
			return fmt.Errorf("healthy runtime source health requires source instance identity")
		}
	}
	if h.Status == RuntimeSourceHealthUnhealthy && strings.TrimSpace(h.Reason) == "" {
		return fmt.Errorf("unhealthy runtime source health requires reason")
	}
	if len(h.Reason) > 512 {
		return fmt.Errorf("runtime source health reason too long")
	}
	return nil
}

func RuntimeSourceHealthProofAllowed(producerID, probeKind string) bool {
	proofs := runtimeSourceHealthProofs[producerID]
	return proofs != nil && proofs[probeKind]
}
