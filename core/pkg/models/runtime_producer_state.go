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
	State             string     `gorm:"size:32;index" json:"state"`
	LastManifestAt    time.Time  `json:"lastManifestAt"`
	LastHeartbeatAt   time.Time  `gorm:"index" json:"lastHeartbeatAt"`
	LastCoverageID          string     `gorm:"size:128" json:"lastCoverageId,omitempty"`
	LastCoverageEnd         *time.Time `gorm:"index" json:"lastCoverageEnd,omitempty"`
	SourceHealthID          string     `gorm:"size:128" json:"sourceHealthId,omitempty"`
	SourceHealthStatus      string     `gorm:"size:32;index" json:"sourceHealthStatus,omitempty"`
	SourceHealthProbeKind   string     `gorm:"size:64;index" json:"sourceHealthProbeKind,omitempty"`
	SourceInstanceID        string     `gorm:"size:255;index" json:"sourceInstanceId,omitempty"`
	SourceHealthObservedAt  *time.Time `gorm:"index" json:"sourceHealthObservedAt,omitempty"`
	SourceHealthHealthySince *time.Time `gorm:"index" json:"sourceHealthHealthySince,omitempty"`
	SourceHealthReason      string     `gorm:"size:512" json:"sourceHealthReason,omitempty"`
	GapSince                *time.Time `gorm:"index" json:"gapSince,omitempty"`
	GapReason               string     `gorm:"size:128" json:"gapReason,omitempty"`
}

func (s RuntimeProducerState) SourceHealthFresh(now time.Time) bool {
	return s.Authoritative &&
		s.SourceHealthStatus == collection.RuntimeSourceHealthHealthy &&
		s.SourceHealthObservedAt != nil && !s.SourceHealthObservedAt.IsZero() &&
		!now.Before(*s.SourceHealthObservedAt) &&
		now.Sub(*s.SourceHealthObservedAt) <= collection.RuntimeSourceHealthMaxGap &&
		s.SourceHealthHealthySince != nil && !s.SourceHealthHealthySince.IsZero()
}

// SourceHealthCovers is the bounded upstream-health primitive. A coverage window
// is absence-eligible only when it lies entirely inside the continuously healthy
// source interval observed for the exact current producer/session.
func (s RuntimeProducerState) SourceHealthCovers(start, end, now time.Time) bool {
	if start.IsZero() || end.IsZero() || end.Before(start) || !s.SourceHealthFresh(now) {
		return false
	}
	return !s.SourceHealthHealthySince.After(start) &&
		s.SourceHealthObservedAt != nil &&
		!s.SourceHealthObservedAt.Before(end)
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
		if !s.SourceHealthFresh(now) {
			return collection.RuntimeProducerNonAuthoritative
		}
		return s.State
	case collection.RuntimeProducerStarting, collection.RuntimeProducerDegraded:
		return s.State
	default:
		return "unknown"
	}
}
