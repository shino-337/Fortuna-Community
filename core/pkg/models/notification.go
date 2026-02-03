package models

import (
	"time"

	"gorm.io/gorm"
)

// Notification represents a system notification (alerts, events).
type Notification struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Title     string         `gorm:"size:255;not null" json:"title"`
	Message   string         `gorm:"type:text" json:"message"`
	Severity  string         `gorm:"size:50" json:"severity"` // info, warning, error, critical
	ReadAt    *time.Time     `json:"readAt,omitempty"`
	Source    string         `gorm:"size:100" json:"source"` // core, agent, policy, etc.
	CreatedAt time.Time      `json:"timestamp"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides table name
func (Notification) TableName() string {
	return "notifications"
}
