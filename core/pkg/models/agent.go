package models

import (
	"time"

	"gorm.io/gorm"
)

// Agent represents a connected agent instance.
type Agent struct {
	gorm.Model
	AgentID      string `gorm:"uniqueIndex;size:255;not null"`
	NodeName     string `gorm:"size:255"`
	Version      string `gorm:"size:100"`
	Status       string `gorm:"size:50"`
	Capabilities string `gorm:"type:jsonb"`
	LastSeenAt   *time.Time
}
