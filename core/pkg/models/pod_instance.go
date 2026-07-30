package models

import "time"

// PodInstance represents a pod instance with explicit lifecycle
type PodInstance struct {
	PodUID      string     `gorm:"primaryKey;type:varchar(255)" json:"podUid"`
	WorkloadID  *string    `gorm:"type:varchar(255);index" json:"workloadId,omitempty"`
	Namespace   string     `gorm:"type:varchar(255);not null;index" json:"namespace"`
	Name        string     `gorm:"type:varchar(255);not null;index" json:"name"`
	Generation  int        `gorm:"default:1" json:"generation"`
	StartedAt   time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"startedAt"`
	TerminatedAt *time.Time `gorm:"type:timestamp with time zone" json:"terminatedAt,omitempty"`
	Status      string     `gorm:"type:varchar(20);not null;default:active;check:status IN ('active', 'terminated')" json:"status"`
}

// TableName overrides table name
func (PodInstance) TableName() string {
	return "pod_instances"
}
