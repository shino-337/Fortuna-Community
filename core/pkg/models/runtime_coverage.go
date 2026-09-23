package models

import (
	"time"

	"github.com/fortuna/api/collection"
)

// RuntimeCoverage is the mutable latest-state projection for one Agent producer.
// Every accepted non-replay window is also written immutably to
// RuntimeCoverageReceipt in the same transaction. This row exists for ordering,
// lifecycle evaluation and fast reads; it is not the audit history table.
type RuntimeCoverage struct {
	ClusterID       string     `gorm:"primaryKey;size:255" json:"clusterId"`
	AgentID         string     `gorm:"primaryKey;size:255" json:"agentId"`
	ProducerID      string     `gorm:"primaryKey;size:128" json:"producerId"`
	SessionID       string     `gorm:"size:128;index" json:"sessionId"`
	CoverageID      string     `gorm:"size:128" json:"coverageId"`
	SourceKind      string     `gorm:"size:64;index" json:"sourceKind"`
	Status          string     `gorm:"size:32;index" json:"status"`
	WindowStart     time.Time  `json:"windowStart"`
	WindowEnd       time.Time  `gorm:"index" json:"windowEnd"`
	ReceivedAt      time.Time  `json:"receivedAt"`
	ContinuousSince *time.Time `gorm:"index" json:"continuousSince,omitempty"`
	Emitted         uint64     `json:"emitted"`
	Delivered       uint64     `json:"delivered"`
	Dropped         uint64     `json:"dropped"`
	Invalid         uint64     `json:"invalid"`
	Errors          uint64     `json:"errors"`
	Reason          string     `gorm:"size:512" json:"reason,omitempty"`
}

func (c RuntimeCoverage) EffectiveStatus(producer *RuntimeProducerState, now time.Time) string {
	if producer == nil || producer.ClusterID != c.ClusterID || producer.AgentID != c.AgentID ||
		producer.ProducerID != c.ProducerID || producer.SessionID != c.SessionID {
		return "unknown"
	}
	if producer.EffectiveStatus(now) != collection.RuntimeProducerActive {
		return producer.EffectiveStatus(now)
	}
	if c.Status != "complete" {
		return c.Status
	}
	if c.ContinuousSince == nil || c.ContinuousSince.IsZero() {
		return "unknown"
	}
	if c.WindowEnd.IsZero() || c.WindowEnd.After(now) || now.Sub(c.WindowEnd) > collection.RuntimeCoverageMaxAge {
		return "stale"
	}
	return "complete"
}

// CoversInterval is the fail-closed primitive for later absence-based reasoning.
// It requires both coverage continuity and a currently active lifecycle lease for
// the same Agent execution session.
func (c RuntimeCoverage) CoversInterval(producer *RuntimeProducerState, requiredStart, requiredEnd, now time.Time) bool {
	if requiredStart.IsZero() || requiredEnd.IsZero() || requiredEnd.Before(requiredStart) {
		return false
	}
	return c.EffectiveStatus(producer, now) == "complete" &&
		c.ContinuousSince != nil &&
		!c.ContinuousSince.After(requiredStart) &&
		!c.WindowEnd.Before(requiredEnd)
}
