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
	ClusterID         string     `gorm:"primaryKey;size:255" json:"clusterId"`
	AgentID           string     `gorm:"primaryKey;size:255" json:"agentId"`
	ProducerID        string     `gorm:"primaryKey;size:128" json:"producerId"`
	SourceKind        string     `gorm:"size:64;index" json:"sourceKind"`
	SessionID         string     `gorm:"size:128;index" json:"sessionId"`
	SessionStartedAt  time.Time  `json:"sessionStartedAt"`
	Enabled           bool       `gorm:"index" json:"enabled"`
	Authoritative     bool       `gorm:"index" json:"authoritative"`
	SourceHealthStatus string    `gorm:"size:32;index" json:"sourceHealthStatus,omitempty"`
	SourceHealthProofKind string `gorm:"size:64" json:"sourceHealthProofKind,omitempty"`
	SourceHealthObservedAt *time.Time `gorm:"index" json:"sourceHealthObservedAt,omitempty"`
	SourceHealthContinuousSince *time.Time `gorm:"index" json:"sourceHealthContinuousSince,omitempty"`
	SourceHealthValidUntil *time.Time `gorm:"index" json:"sourceHealthValidUntil,omitempty"`
	State             string     `gorm:"size:32;index" json:"state"`
	LastManifestAt    time.Time  `json:"lastManifestAt"`
	LastHeartbeatAt   time.Time  `gorm:"index" json:"lastHeartbeatAt"`
	LastCoverageID    string     `gorm:"size:128" json:"lastCoverageId,omitempty"`
	LastCoverageEnd   *time.Time `gorm:"index" json:"lastCoverageEnd,omitempty"`
	GapSince          *time.Time `gorm:"index" json:"gapSince,omitempty"`
	GapReason         string     `gorm:"size:128" json:"gapReason,omitempty"`
}

func (s RuntimeProducerState) LeaseFresh(now time.Time) bool {
	return s.SessionID != "" && !s.LastHeartbeatAt.IsZero() &&
		!now.Before(s.LastHeartbeatAt) &&
		now.Sub(s.LastHeartbeatAt) <= collection.RuntimeProducerLeaseMaxAge
}

func (s RuntimeProducerState) SourceHealthCovers(start, end, now time.Time) bool {
	return s.Authoritative &&
		s.SourceHealthStatus == collection.RuntimeSourceHealthHealthy &&
		s.SourceHealthContinuousSince != nil &&
		s.SourceHealthValidUntil != nil &&
		!start.Before(*s.SourceHealthContinuousSince) &&
		!end.After(*s.SourceHealthValidUntil) &&
		!now.Before(*s.SourceHealthObservedAt) &&
		now.Before(*s.SourceHealthValidUntil)
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
		if !s.Authoritative ||
			s.SourceHealthStatus != collection.RuntimeSourceHealthHealthy ||
			s.SourceHealthObservedAt == nil || s.SourceHealthValidUntil == nil ||
			now.Before(*s.SourceHealthObservedAt) || !now.Before(*s.SourceHealthValidUntil) {
			return collection.RuntimeProducerNonAuthoritative
		}
		return s.State
	case collection.RuntimeProducerStarting, collection.RuntimeProducerDegraded:
		return s.State
	default:
		return "unknown"
	}
}
