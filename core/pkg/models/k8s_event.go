package models

import "time"

// K8sEvent represents a Kubernetes cluster event (Event Service).
type K8sEvent struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	ClusterID      string     `gorm:"type:varchar(255);not null;index" json:"clusterId"`
	EventUID       string     `gorm:"type:varchar(255);not null" json:"eventUid"`
	Namespace      string     `gorm:"type:varchar(255);not null" json:"namespace"`
	EventName      string     `gorm:"type:varchar(255);not null" json:"eventName"`
	InvolvedKind   string     `gorm:"type:varchar(64);not null;index" json:"involvedKind"`   // Pod, Deployment, Node, ...
	InvolvedUID    string     `gorm:"type:varchar(255);not null;index" json:"involvedUid"`
	InvolvedName   string     `gorm:"type:varchar(255);not null" json:"involvedName"`
	Reason         string     `gorm:"type:varchar(128);not null" json:"reason"`
	Message        string     `gorm:"type:text" json:"message"`
	EventType      string     `gorm:"type:varchar(32);default:Normal" json:"eventType"` // Normal, Warning
	Count          int        `gorm:"default:1" json:"count"`
	FirstTimestamp *time.Time `gorm:"column:first_timestamp" json:"firstTimestamp,omitempty"`
	LastTimestamp  *time.Time `gorm:"column:last_timestamp" json:"lastTimestamp,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

// TableName overrides table name.
func (K8sEvent) TableName() string {
	return "k8s_events"
}
