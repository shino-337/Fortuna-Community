package models

import "time"

// RuntimeBehaviorFact stores normalized, source-independent behavior primitives extracted
// from runtime_events. This is Layer 2 between raw telemetry and security semantics.
type RuntimeBehaviorFact struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	FactID     string    `gorm:"type:varchar(64);not null;uniqueIndex" json:"factId"` // deterministic id (e.g. event-id + fact-type)
	EventID    uint      `gorm:"not null;index" json:"eventId"`                       // FK-ish link to runtime_events.id
	PodUID     string    `gorm:"type:varchar(255);not null;index" json:"podUid"`
	Namespace  string    `gorm:"type:varchar(255);not null;index" json:"namespace"`
	FactType   string    `gorm:"type:varchar(100);not null;index" json:"factType"` // PROCESS_EXEC/FILE_READ/NETWORK_CONNECT/...
	Domain     string    `gorm:"type:varchar(50);not null;index" json:"domain"`    // execution/filesystem/network/...
	Attributes string    `gorm:"type:jsonb;not null" json:"attributes"`            // compact fact attributes
	SourceRef  string    `gorm:"type:jsonb;not null" json:"sourceRef"`             // source_kind/rule/runtime + event linkage
	ObservedAt time.Time `gorm:"index" json:"observedAt"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (RuntimeBehaviorFact) TableName() string {
	return "runtime_behavior_facts"
}
