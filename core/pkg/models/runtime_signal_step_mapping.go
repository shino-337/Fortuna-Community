package models

import "time"

// RuntimeSignalStepMapping maps runtime signal_type -> attack step_id.
// This allows runtime-step progress correlation to be configured without code changes.
type RuntimeSignalStepMapping struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	SignalType    string     `gorm:"type:varchar(100);not null;index;uniqueIndex:uidx_signal_step_map" json:"signalType"`
	StepID        string     `gorm:"type:varchar(100);not null;index;uniqueIndex:uidx_signal_step_map" json:"stepId"`
	Enabled       bool       `gorm:"not null;default:true;index" json:"enabled"`
	EffectiveFrom *time.Time `gorm:"index" json:"effectiveFrom,omitempty"`
	CreatedBy     string     `gorm:"type:varchar(100);default:''" json:"createdBy,omitempty"`
	UpdatedBy     string     `gorm:"type:varchar(100);default:''" json:"updatedBy,omitempty"`
	CreatedAt     time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
	UpdatedAt     time.Time  `gorm:"default:CURRENT_TIMESTAMP" json:"updatedAt"`
}

func (RuntimeSignalStepMapping) TableName() string {
	return "runtime_signal_step_mappings"
}
