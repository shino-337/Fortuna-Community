package collection

import (
	"fmt"
	"strings"
	"time"
)

const RuntimeCoverageVersion = 1
const RuntimeCoverageMaxAge = 10 * time.Minute

const (
	RuntimeSourceFile  = "file"
	RuntimeSourceFalco = "falco"
	RuntimeSourceEBPF  = "ebpf"
)

// RuntimeCoverage describes one producer observation window. Counters are local
// to this window. "Emitted" means events attempted for delivery in the window,
// so retries in a later window are counted again and cannot erase an earlier
// delivery outage. A complete zero-event window is explicit producer evidence;
// runtime-event silence by itself never creates coverage.
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
	Errors      uint64    `json:"errors"`
	Reason      string    `json:"reason,omitempty"`
}

func (c RuntimeCoverage) Validate(now time.Time) error {
	if c.Version != RuntimeCoverageVersion || len(c.ID) < 16 || len(c.ID) > 128 || strings.TrimSpace(c.ID) != c.ID {
		return fmt.Errorf("invalid runtime coverage identity/version")
	}
	if strings.TrimSpace(c.ProducerID) == "" || len(c.ProducerID) > 128 || strings.TrimSpace(c.ProducerID) != c.ProducerID {
		return fmt.Errorf("runtime coverage producer required")
	}
	validProducer := false
	switch c.SourceKind {
	case RuntimeSourceFile:
		validProducer = c.ProducerID == "runtime-file"
	case RuntimeSourceFalco:
		validProducer = c.ProducerID == "falco"
	case RuntimeSourceEBPF:
		validProducer = c.ProducerID == "ebpf-exec" || c.ProducerID == "ebpf-connect" || c.ProducerID == "ebpf-all"
	default:
		return fmt.Errorf("unsupported runtime coverage source")
	}
	if !validProducer {
		return fmt.Errorf("runtime coverage producer/source mismatch")
	}
	if c.Status != "complete" && c.Status != "failed" {
		return fmt.Errorf("invalid runtime coverage status")
	}
	// Historical windows are accepted so an immutable pending report can drain
	// after a long Core outage. Freshness is enforced by EffectiveStatus/CoversSince,
	// never by rejecting the historical report and wedging the producer queue.
	if c.WindowStart.IsZero() || c.WindowEnd.Before(c.WindowStart) || c.WindowEnd.After(now.Add(time.Minute)) {
		return fmt.Errorf("invalid runtime coverage interval")
	}
	if c.Delivered > c.Emitted {
		return fmt.Errorf("runtime delivered count exceeds emitted count")
	}
	if c.Status == "complete" && (c.Dropped != 0 || c.Invalid != 0 || c.Errors != 0 || c.Delivered != c.Emitted) {
		return fmt.Errorf("complete runtime coverage cannot contain loss or errors")
	}
	if c.Status == "failed" && strings.TrimSpace(c.Reason) == "" {
		return fmt.Errorf("failed runtime coverage requires reason")
	}
	if len(c.Reason) > 512 {
		return fmt.Errorf("runtime coverage reason too long")
	}
	return nil
}
