package models

import "time"

// RuntimeSignal represents a semantic runtime behavior signal
type RuntimeSignal struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	PodUID     string    `gorm:"type:varchar(255);not null;index" json:"podUid"`
	SignalType string    `gorm:"type:varchar(100);not null;index" json:"signalType"`
	Category   string    `gorm:"type:varchar(50);not null;index" json:"category"`
	Confidence float64   `gorm:"type:float;default:0.5;check:confidence >= 0 AND confidence <= 1" json:"confidence"`
	Evidence   string    `gorm:"type:jsonb;not null" json:"evidence"`
	CreatedAt  time.Time `gorm:"default:CURRENT_TIMESTAMP;index" json:"createdAt"`
}

// TableName overrides table name
func (RuntimeSignal) TableName() string {
	return "runtime_signals"
}
