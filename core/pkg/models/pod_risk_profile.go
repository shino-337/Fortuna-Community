package models

import (
	"time"

	"github.com/lib/pq"
)

// PodRiskProfile stores static/runtime risk signals for a pod.
type PodRiskProfile struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	ClusterID    string         `gorm:"type:varchar(255);index" json:"clusterId,omitempty"`
	PodUID       string         `gorm:"type:varchar(255);not null;index" json:"podUid"`
	Namespace    string         `gorm:"type:varchar(255);not null;index" json:"namespace"`
	StaticRisk   int            `gorm:"not null;default:0" json:"staticRisk"`
	RuntimeScore int            `gorm:"not null;default:0" json:"runtimeScore"`
	Capabilities pq.StringArray `gorm:"type:text[];column:capabilities" json:"capabilities"`
	LastEventAt  *time.Time     `json:"lastEventAt"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
}

// TableName overrides table name.
func (PodRiskProfile) TableName() string {
	return "pod_risk_profiles"
}
