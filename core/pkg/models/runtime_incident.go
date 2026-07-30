package models

import "time"

// RuntimeIncident stores correlated, stateful runtime conclusions.
// In P0 this table is introduced first; synthesizers are added incrementally.
type RuntimeIncident struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	IncidentID   string    `gorm:"type:varchar(64);not null;uniqueIndex" json:"incidentId"`
	PodUID       string    `gorm:"type:varchar(255);not null;index" json:"podUid"`
	Namespace    string    `gorm:"type:varchar(255);not null;index" json:"namespace"`
	IncidentType string    `gorm:"type:varchar(100);not null;index" json:"incidentType"`
	SeverityHint string    `gorm:"type:varchar(20);not null;index" json:"severityHint"`
	Confidence   float64   `gorm:"type:float;default:0.5;check:confidence >= 0 AND confidence <= 1" json:"confidence"`
	FirstSeenAt  time.Time `gorm:"index" json:"firstSeenAt"`
	LastSeenAt   time.Time `gorm:"index" json:"lastSeenAt"`
	Window       string    `gorm:"type:varchar(20)" json:"window,omitempty"` // e.g. "5m"
	EvidenceRefs string    `gorm:"type:jsonb;not null" json:"evidenceRefs"`
	Metadata     string    `gorm:"type:jsonb;not null" json:"metadata"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (RuntimeIncident) TableName() string {
	return "runtime_incidents"
}
