package models

import "time"

// RuntimeSignal represents a semantic runtime behavior signal
type RuntimeSignal struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ClusterID    string    `gorm:"type:varchar(255);index" json:"clusterId,omitempty"`
	PodUID       string    `gorm:"type:varchar(255);not null;index" json:"podUid"`
	SignalType   string    `gorm:"type:varchar(100);not null;index" json:"signalType"`
	Category     string    `gorm:"type:varchar(50);not null;index" json:"category"`
	Confidence   float64   `gorm:"type:float;default:0.5;check:confidence >= 0 AND confidence <= 1" json:"confidence"`
	Evidence     string    `gorm:"type:jsonb;not null" json:"evidence"`
	EvidenceRefs string    `gorm:"type:jsonb;column:evidence_refs" json:"evidenceRefs,omitempty"`
	Count        int       `gorm:"default:1" json:"count"`
	FirstSeenAt  *string   `gorm:"type:timestamp with time zone;column:first_seen_at;index" json:"firstSeenAt,omitempty"`
	LastSeenAt   *string   `gorm:"type:timestamp with time zone;column:last_seen_at;index" json:"lastSeenAt,omitempty"`
	CreatedAt    time.Time `gorm:"default:CURRENT_TIMESTAMP;index" json:"createdAt"`
}

// TableName overrides table name
func (RuntimeSignal) TableName() string {
	return "runtime_signals"
}
