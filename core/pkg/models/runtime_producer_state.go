package models

import (
	"time"

	"github.com/fortuna/api/collection"
)

type RuntimeProducerState struct {
	ClusterID         string     `gorm:"primaryKey;size:255" json:"clusterId"`
	AgentID           string     `gorm:"primaryKey;size:255" json:"agentId"`
	ProducerID        string     `gorm:"primaryKey;size:128" json:"producerId"`
	SourceKind        string     `gorm:"size:64;index" json:"sourceKind"`
	SessionID         string     `gorm:"size:128;index" json:"sessionId"`
	SessionStartedAt  time.Time  `json:"sessionStartedAt"`
	Enabled           bool       `gorm:"index" json:"enabled"`
	Authoritative     bool       `gorm:"index" json:"authoritative"`
	State             string     `gorm:"size:32;index" json:"state"`
	LastManifestAt    time.Time  `json:"lastManifestAt"`
	LastHeartbeatAt   time.Time  `gorm:"index" json:"lastHeartbeatAt"`
	LastCoverageID    string     `gorm:"size:128" json:"lastCoverageId,omitempty"`
	LastCoverageEnd   *time.Time `gorm:"index" json:"lastCoverageEnd,omitempty"`
	GapSince          *time.Time `gorm:"index" json:"gapSince,omitempty"`
	GapReason         string     `gorm:"size:128" json:"gapReason,omitempty"`
}

func (s RuntimeProducerState) EffectiveStatus(now time.Time) string {
	if s.SessionID == "" || s.LastHeartbeatAt.IsZero() {
		return "unknown"
	}
	if now.Before(s.LastHeartbeatAt) || now.Sub(s.LastHeartbeatAt) > collection.RuntimeProducerLeaseMaxAge {
		return "stale"
	}
	if !s.Enabled {
		if s.State == collection.RuntimeProducerStopped {
			return collection.RuntimeProducerStopped
		}
		return collection.RuntimeProducerDisabled
	}
	if !s.Authoritative {
		return collection.RuntimeProducerNonAuthoritative
	}
	switch s.State {
	case collection.RuntimeProducerStarting, collection.RuntimeProducerActive, collection.RuntimeProducerDegraded:
		return s.State
	default:
		return "unknown"
	}
}
