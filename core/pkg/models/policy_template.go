package models

import (
	"time"

	"gorm.io/gorm"
)

// PolicyTemplate represents an immutable policy template with CEL expression
type PolicyTemplate struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Template identity
	TemplateID    string `gorm:"uniqueIndex:idx_template_id_version;not null" json:"templateId"`
	Version       string `gorm:"uniqueIndex:idx_template_id_version;not null" json:"version"`

	// Metadata
	Name            string   `gorm:"not null" json:"name"`
	Description     string   `gorm:"type:text" json:"description"`
	Category        string   `gorm:"not null;check:category IN ('security', 'compliance', 'operational', 'governance')" json:"category"`
	DefaultSeverity string   `gorm:"not null" json:"defaultSeverity"`

	// Immutable logic (user CANNOT change)
	CELExpression   string `gorm:"type:text;not null" json:"celExpression"`
	CELProgramCache []byte `gorm:"type:bytea" json:"-"` // Compiled CEL program

	// Default configuration
	DefaultScope  string `gorm:"type:jsonb" json:"defaultScope"` // JSON string
	DefaultAction string `gorm:"default:'alert';check:default_action IN ('alert', 'block', 'audit')" json:"defaultAction"`

	// Remediation
	SupportsRemediation bool   `gorm:"default:false" json:"supportsRemediation"`
	RemediationTemplate string `gorm:"type:jsonb" json:"remediationTemplate"` // JSON string

	// Documentation
	Rationale  string   `gorm:"type:text" json:"rationale"`
	References []string `gorm:"type:text[]" json:"references"`
	Examples   string   `gorm:"type:jsonb" json:"examples"` // JSON string

	// Metadata
	CreatedBy string `gorm:"default:'system'" json:"createdBy"`
	IsSystem  bool   `gorm:"default:true" json:"isSystem"` // Cannot be deleted
}

// TableName specifies the table name for PolicyTemplate
func (PolicyTemplate) TableName() string {
	return "policy_templates"
}

