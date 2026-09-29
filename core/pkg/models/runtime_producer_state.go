package models

import (
	"time"

	"github.com/fortuna/api/collection"
)

// RuntimeProducerState is the persisted lifecycle/enablement lease for a
// configured Agent producer. Enabled is configuration state; Authoritative means
// an independent upstream-health contract exists. A fresh reader heartbeat alone
// must never imply Authoritative.
type RuntimeProducerState struct {
	ClusterID              string     `gorm:"primaryKey;size:255" json:"clusterId"`
	AgentID                string     `gorm:"primaryKey;size:255" json:"agentId"`
	ProducerID             string     `gorm:"primaryKey;size:128" json:"producerId"`
	SourceKind             string     `gorm:"size:64;index" json:"sourceKind"`
	SessionID              string     `gorm:"size:128;index" json:"sessionId"`
	SessionStartedAt       time.Time  `json:"sessionStartedAt"`
	Enabled                bool       `gorm:"index" json:"enabled"`
	Authoritative          bool       `gorm:"index" json:"authoritative"`
	State                  string     `gorm:"size:32;index" json:"state"`
	LastManifestAt         time.Time  `json:"lastManifestAt"`
	LastHeartbeatAt        time.Time  `gorm:"index" json:"lastHeartbeatAt"`
	LastCoverageID         string     `gorm:"size:128" json:"lastCoverageId,omitempty"`
	LastCoverageEnd        *time.Time `gorm:"index" json:"lastCoverageEnd,omitempty"`
	GapSince               *time.Time `gorm:"index" json:"gapSince,omitempty"`
	GapReason              string     `gorm:"size:128" json:"gapReason,omitempty"`
	SourceSessionID        string     `gorm:"size:128" json:"sourceSessionId,omitempty"`
	SourceStartedAt        *time.Time `json:"sourceStartedAt,omitempty"`
	SourceHealthSince      *time.Time `json:"sourceHealthSince,omitempty"`
	SourceHealthEnd        *time.Time `json:"sourceHealthEnd,omitempty"`
	SourceHealthReceivedAt *time.Time `json:"sourceHealthReceivedAt,omitempty"`
	SourceHealthSequence   int64      `json:"sourceHealthSequence"`
	SourceHealthExpiresAt  *time.Time `json:"sourceHealthExpiresAt,omitempty"`
}

func (s RuntimeProducerState) SourceHealthCovers(start, end, now time.Time) bool {
	return s.Authoritative && s.SourceSessionID != "" && s.SourceHealthSince != nil && s.SourceHealthEnd != nil && s.SourceHealthReceivedAt != nil && s.SourceHealthExpiresAt != nil && now.Before(*s.SourceHealthExpiresAt) &&
		!start.IsZero() && !end.Before(start) && !s.SourceHealthSince.After(start) && !s.SourceHealthEnd.Before(end) &&
		!s.SourceHealthEnd.After(now) && now.Sub(*s.SourceHealthEnd) <= collection.RuntimeSourceHealthMaxAge &&
		!s.SourceHealthReceivedAt.After(now) && now.Sub(*s.SourceHealthReceivedAt) <= collection.RuntimeSourceHealthMaxAge
}

func (s *RuntimeProducerState) InvalidateSourceHealth() {
	s.Authoritative = false
	s.SourceHealthSince = nil
	// Preserve signed sequence/session ordering so an old receipt cannot renew.
}

func (s RuntimeProducerState) LeaseFresh(now time.Time) bool {
	return s.SessionID != "" && !s.LastHeartbeatAt.IsZero() &&
		!now.Before(s.LastHeartbeatAt) &&
		now.Sub(s.LastHeartbeatAt) <= collection.RuntimeProducerLeaseMaxAge
}

func (s RuntimeProducerState) EffectiveStatus(now time.Time) string {
	if s.SessionID == "" || s.LastHeartbeatAt.IsZero() {
		return "unknown"
	}
	if !s.LeaseFresh(now) {
		return "stale"
	}
	if !s.Enabled {
		if s.State == collection.RuntimeProducerStopped {
			return collection.RuntimeProducerStopped
		}
		return collection.RuntimeProducerDisabled
	}
	switch s.State {
	case collection.RuntimeProducerActive:
		if s.LastCoverageEnd == nil || s.LastCoverageEnd.IsZero() || now.Before(*s.LastCoverageEnd) ||
			now.Sub(*s.LastCoverageEnd) > collection.RuntimeProducerLeaseMaxAge {
			return "stale"
		}
		if !s.SourceHealthCovers(*s.LastCoverageEnd, *s.LastCoverageEnd, now) {
			return collection.RuntimeProducerNonAuthoritative
		}
		return s.State
	case collection.RuntimeProducerStarting, collection.RuntimeProducerDegraded:
		return s.State
	default:
		return "unknown"
	}
}
