package models

import "time"

// RuntimeEvent stores raw runtime probe events (sensor/audit).
type RuntimeEvent struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	PodUID     string    `gorm:"type:varchar(255);not null;index" json:"podUid"`
	Namespace  string    `gorm:"type:varchar(255);not null;index" json:"namespace"`
	Syscall    string    `gorm:"type:varchar(100);not null;index" json:"syscall"`
	TargetPath string    `gorm:"type:varchar(500)" json:"targetPath"`
	Capability string    `gorm:"type:varchar(100)" json:"capability"`
	CreatedAt  time.Time `json:"createdAt"`
}

// TableName overrides table name.
func (RuntimeEvent) TableName() string {
	return "runtime_events"
}
