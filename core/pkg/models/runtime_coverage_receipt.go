package models

import "time"

// RuntimeCoverageReceipt is the immutable audit record for every accepted
// non-replay runtime coverage window. RuntimeCoverage remains the mutable latest
// projection used for ordering/arbitration.
type RuntimeCoverageReceipt struct {
	ClusterID       string     `gorm:"primaryKey;size:255" json:"clusterId"`
	AgentID         string     `gorm:"primaryKey;size:255" json:"agentId"`
	ProducerID      string     `gorm:"primaryKey;size:128" json:"producerId"`
	SessionID       string     `gorm:"primaryKey;size:128;index" json:"sessionId"`
	SourceSessionID string     `gorm:"size:128" json:"sourceSessionId,omitempty"`
	CoverageID      string     `gorm:"primaryKey;size:128" json:"coverageId"`
	SourceKind      string     `gorm:"size:64;index" json:"sourceKind"`
	Status          string     `gorm:"size:32;index" json:"status"`
	WindowStart     time.Time  `json:"windowStart"`
	WindowEnd       time.Time  `gorm:"index" json:"windowEnd"`
	ReceivedAt      time.Time  `gorm:"index" json:"receivedAt"`
	ContinuousSince *time.Time `json:"continuousSince,omitempty"`
	Emitted         uint64     `json:"emitted"`
	Delivered       uint64     `json:"delivered"`
	Dropped         uint64     `json:"dropped"`
	Invalid         uint64     `json:"invalid"`
	Errors          uint64     `json:"errors"`
	Reason          string     `gorm:"size:512" json:"reason,omitempty"`
}

func RuntimeCoverageReceiptFrom(row RuntimeCoverage) RuntimeCoverageReceipt {
	return RuntimeCoverageReceipt{
		ClusterID:       row.ClusterID,
		AgentID:         row.AgentID,
		ProducerID:      row.ProducerID,
		SessionID:       row.SessionID,
		SourceSessionID: row.SourceSessionID,
		CoverageID:      row.CoverageID,
		SourceKind:      row.SourceKind,
		Status:          row.Status,
		WindowStart:     row.WindowStart,
		WindowEnd:       row.WindowEnd,
		ReceivedAt:      row.ReceivedAt,
		ContinuousSince: row.ContinuousSince,
		Emitted:         row.Emitted,
		Delivered:       row.Delivered,
		Dropped:         row.Dropped,
		Invalid:         row.Invalid,
		Errors:          row.Errors,
		Reason:          row.Reason,
	}
}

func RuntimeCoverageFromReceipt(row RuntimeCoverageReceipt) RuntimeCoverage {
	return RuntimeCoverage{
		ClusterID:       row.ClusterID,
		AgentID:         row.AgentID,
		ProducerID:      row.ProducerID,
		SessionID:       row.SessionID,
		SourceSessionID: row.SourceSessionID,
		CoverageID:      row.CoverageID,
		SourceKind:      row.SourceKind,
		Status:          row.Status,
		WindowStart:     row.WindowStart,
		WindowEnd:       row.WindowEnd,
		ReceivedAt:      row.ReceivedAt,
		ContinuousSince: row.ContinuousSince,
		Emitted:         row.Emitted,
		Delivered:       row.Delivered,
		Dropped:         row.Dropped,
		Invalid:         row.Invalid,
		Errors:          row.Errors,
		Reason:          row.Reason,
	}
}
