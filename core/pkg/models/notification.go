package models

import (
	"time"

	"gorm.io/gorm"
)

// Notification represents a system notification (alerts, events).
type Notification struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Title        string         `gorm:"size:255;not null" json:"title"`
	Message      string         `gorm:"type:text" json:"message"`
	Severity     string         `gorm:"size:50" json:"severity"` // info, warning, error, critical
	ReadAt       *time.Time     `json:"readAt,omitempty"`
	Source       string         `gorm:"size:100" json:"source"` // core, agent, policy, etc.
	Category     string         `gorm:"size:64;index" json:"category,omitempty"`
	Route        string         `gorm:"size:255" json:"route,omitempty"`
	DedupeKey    string         `gorm:"size:255" json:"-"`
	ClusterID    string         `gorm:"size:255;index" json:"clusterId,omitempty"`
	ResourceUID  string         `gorm:"size:255;index" json:"resourceUid,omitempty"`
	ResourceName string         `gorm:"size:255;index" json:"resourceName,omitempty"`
	CreatedAt    time.Time      `json:"timestamp"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides table name
func (Notification) TableName() string {
	return "notifications"
}

// NotificationRead records that one user has read one notification. Read state
// is per user: one operator clearing the bell must not clear it for everyone.
type NotificationRead struct {
	NotificationID uint      `gorm:"primaryKey;autoIncrement:false" json:"notificationId"`
	UserID         uint      `gorm:"primaryKey;autoIncrement:false;index" json:"userId"`
	ReadAt         time.Time `gorm:"not null" json:"readAt"`
}

// TableName overrides table name
func (NotificationRead) TableName() string {
	return "notification_reads"
}
