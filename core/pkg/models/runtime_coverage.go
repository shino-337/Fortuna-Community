package models

import (
	"time"

	"github.com/fortuna/api/collection"
)

// RuntimeCoverage records the latest authenticated producer coverage window.
// It proves only the named producer/window; it is not cluster-wide coverage.
type RuntimeCoverage struct {
	ClusterID   string    `gorm:"primaryKey;size:255" json:"clusterId"`
	AgentID     string    `gorm:"primaryKey;size:255" json:"agentId"`
	ProducerID  string    `gorm:"primaryKey;size:128" json:"producerId"`
	CoverageID  string    `gorm:"size:128" json:"coverageId"`
	SourceKind  string    `gorm:"size:64;index" json:"sourceKind"`
	Status      string    `gorm:"size:32;index" json:"status"`
	WindowStart time.Time `json:"windowStart"`
	WindowEnd   time.Time `gorm:"index" json:"windowEnd"`
	ReceivedAt  time.Time `json:"receivedAt"`
	Emitted     uint64    `json:"emitted"`
	Delivered   uint64    `json:"delivered"`
	Dropped     uint64    `json:"dropped"`
	Invalid     uint64    `json:"invalid"`
	Reason      string    `gorm:"size:512" json:"reason,omitempty"`
}

func (c RuntimeCoverage) EffectiveStatus(now time.Time) string {
	if c.Status != "complete" {
		return c.Status
	}
	if c.WindowEnd.IsZero() || c.WindowEnd.After(now) || now.Sub(c.WindowEnd) > collection.RuntimeCoverageMaxAge {
		return "stale"
	}
	return "complete"
}
