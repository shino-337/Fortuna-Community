package models

import "time"

// RuntimeSourceHealthReceipt retains independently signed immutable evidence.
type RuntimeSourceHealthReceipt struct {
	ClusterID       string `gorm:"primaryKey;size:255"`
	AgentID         string `gorm:"primaryKey;size:255"`
	ProducerID      string `gorm:"primaryKey;size:128"`
	SessionID       string `gorm:"primaryKey;size:128"`
	SourceSessionID string `gorm:"primaryKey;size:128"`
	Sequence        int64  `gorm:"primaryKey"`
	KeyID           string `gorm:"size:128"`
	PayloadHash     string `gorm:"size:64"`
	Signature       string `gorm:"size:128"`
	Payload         string `gorm:"type:text"`
	ReceivedAt      time.Time
}
