package models

import (
	"time"

	"gorm.io/gorm"
)

// ExceptionPolicy represents a user-created false-positive / suppress rule.
// Suppression is cluster-qualified: ResourceUID is only meaningful together with
// ClusterID, otherwise duplicate Kubernetes UIDs across clusters could suppress
// unrelated findings.
type ExceptionPolicy struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	ClusterID   string         `gorm:"type:varchar(255);index;not null;default:''" json:"clusterId"`
	ResourceUID string         `gorm:"type:varchar(255);index" json:"resourceUid"`
	CVEID       string         `gorm:"type:varchar(100);index" json:"cveId"`
	InsightType string         `gorm:"type:varchar(50);index" json:"insightType"`
	Reason      string         `gorm:"type:text" json:"reason"`
	ExpiresAt   *time.Time     `json:"expiresAt,omitempty"`
	CreatedBy   string         `gorm:"type:varchar(255)" json:"createdBy"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides table name
func (ExceptionPolicy) TableName() string {
	return "exception_policies"
}
