package models

import (
	"time"

	"gorm.io/gorm"
)

// ErrorLog represents an error log entry (core/agent/system).
type ErrorLog struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Source    string         `gorm:"size:100;not null" json:"source"` // core, agent, worker, etc.
	Level     string         `gorm:"size:20;not null" json:"level"`   // ERROR, WARN, INFO
	Message   string         `gorm:"type:text;not null" json:"message"`
	CreatedAt time.Time      `json:"time"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides table name
func (ErrorLog) TableName() string {
	return "error_logs"
}
