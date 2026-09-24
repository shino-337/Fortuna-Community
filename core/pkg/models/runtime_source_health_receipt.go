package models

import "time"

// RuntimeSourceHealthReceipt is immutable evidence that an authenticated Agent
// observed an approved upstream source-health probe result. It is separate from
// runtime coverage: event-window completeness cannot manufacture source health.
type RuntimeSourceHealthReceipt struct {
	ClusterID        string    `gorm:"primaryKey;size:255" json:"clusterId"`
	AgentID          string    `gorm:"primaryKey;size:255" json:"agentId"`
	ProducerID       string    `gorm:"primaryKey;size:128" json:"producerId"`
	SessionID        string    `gorm:"primaryKey;size:128;index" json:"sessionId"`
	HealthID         string    `gorm:"primaryKey;size:128" json:"healthId"`
	SourceKind       string    `gorm:"size:64;index" json:"sourceKind"`
	ProbeKind        string    `gorm:"size:64;index" json:"probeKind"`
	SourceInstanceID string    `gorm:"size:255;index" json:"sourceInstanceId,omitempty"`
	Status           string    `gorm:"size:32;index" json:"status"`
	ObservedAt       time.Time `gorm:"index" json:"observedAt"`
	ReceivedAt       time.Time `gorm:"index" json:"receivedAt"`
	Reason           string    `gorm:"size:512" json:"reason,omitempty"`
}
