package collection

import (
	"fmt"
	"strings"
	"time"
)

const RuntimeCoverageVersion = 1
const RuntimeCoverageMaxAge = 10 * time.Minute

// RuntimeCoverage describes one producer observation window. A complete window
// with zero delivered events is explicit clean telemetry only for that producer;
// it is never inferred from runtime-event silence.
type RuntimeCoverage struct {
	Version     int       `json:"version"`
	ID          string    `json:"id"`
	ProducerID  string    `json:"producerId"`
	SourceKind  string    `json:"sourceKind"`
	Status      string    `json:"status"` // complete or failed
	WindowStart time.Time `json:"windowStart"`
	WindowEnd   time.Time `json:"windowEnd"`
	Emitted     uint64    `json:"emitted"`
	Delivered   uint64    `json:"delivered"`
	Dropped     uint64    `json:"dropped"`
	Invalid     uint64    `json:"invalid"`
	Reason      string    `json:"reason,omitempty"`
}

func (c RuntimeCoverage) Validate(now time.Time) error {
	if c.Version != RuntimeCoverageVersion || len(c.ID) < 16 || len(c.ID) > 128 || strings.TrimSpace(c.ID) != c.ID {
		return fmt.Errorf("invalid runtime coverage identity/version")
	}
	if strings.TrimSpace(c.ProducerID) == "" || len(c.ProducerID) > 128 || strings.TrimSpace(c.SourceKind) == "" || len(c.SourceKind) > 64 {
		return fmt.Errorf("runtime coverage producer/source required")
	}
	if c.Status != "complete" && c.Status != "failed" {
		return fmt.Errorf("invalid runtime coverage status")
	}
	if c.WindowStart.IsZero() || c.WindowEnd.Before(c.WindowStart) || c.WindowEnd.After(now.Add(time.Minute)) || c.WindowStart.Before(now.Add(-RuntimeCoverageMaxAge)) {
		return fmt.Errorf("invalid or stale runtime coverage interval")
	}
	if c.Delivered > c.Emitted {
		return fmt.Errorf("runtime delivered count exceeds emitted count")
	}
	if c.Status == "complete" && (c.Dropped != 0 || c.Invalid != 0 || c.Delivered != c.Emitted) {
		return fmt.Errorf("complete runtime coverage cannot contain loss")
	}
	if len(c.Reason) > 512 {
		return fmt.Errorf("runtime coverage reason too long")
	}
	return nil
}
