package models

import "time"

// PodAttackStep represents a minimal attack step for a pod
type PodAttackStep struct {
	ClusterID   string    `gorm:"type:varchar(255);index" json:"clusterId,omitempty"`
	PodUID      string    `gorm:"primaryKey;type:varchar(255)" json:"podUid"`
	StepID      string    `gorm:"primaryKey;type:varchar(100);index" json:"stepId"`
	Description string    `gorm:"type:text" json:"description,omitempty"`
	Category    string    `gorm:"type:varchar(50);not null;index" json:"category"`
	Confidence  float64   `gorm:"type:float;default:0.5;check:confidence >= 0 AND confidence <= 1" json:"confidence"`
	Evidence    string    `gorm:"type:jsonb" json:"evidence,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// TableName overrides table name
func (PodAttackStep) TableName() string {
	return "pod_attack_steps"
}
