package models

import (
	"time"

	"gorm.io/gorm"
)

// PolicyViolation represents a policy violation detected during resource evaluation
type PolicyViolation struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Policy reference
	InstanceID   uint   `gorm:"not null;index" json:"instanceId"`
	InstanceName string `gorm:"not null" json:"instanceName"`
	TemplateID   string `gorm:"not null" json:"templateId"`
	TemplateName string `gorm:"not null" json:"templateName"`

	// Resource info
	ResourceType string `gorm:"not null;index:idx_violations_resource" json:"resourceType"`
	ResourceUID  string `gorm:"not null;index:idx_violations_resource" json:"resourceUid"`
	ResourceName string `json:"resourceName"`
	Namespace    string `json:"namespace"`
	ClusterID    string `gorm:"not null;index" json:"clusterId"`

	// Violation details
	Severity string `gorm:"not null;check:severity IN ('low', 'medium', 'high', 'critical')" json:"severity"`
	Action   string `gorm:"not null;check:action IN ('alert', 'block', 'audit', 'remediate')" json:"action"`
	Status   string `gorm:"default:'active';check:status IN ('active', 'resolved', 'dismissed')" json:"status"`
	Message  string `gorm:"type:text" json:"message"`

	// Enforcement
	EnforcedAt        *time.Time `json:"enforcedAt,omitempty"`
	EnforcementResult string     `gorm:"type:text" json:"enforcementResult,omitempty"`

	// Metadata
	DetectedAt *time.Time `gorm:"default:now();index" json:"detectedAt"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`

	// Relations
	Instance PolicyInstance `gorm:"foreignKey:InstanceID" json:"instance,omitempty"`
}

// TableName specifies the table name for PolicyViolation
func (PolicyViolation) TableName() string {
	return "policy_violations"
}

