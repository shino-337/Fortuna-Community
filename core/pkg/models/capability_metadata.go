package models

import (
	"time"
)

// CapabilityMetadata provides semantic layer for capabilities.
// Preconditions and ProducesAttackSteps are JSONB in DB; use JSONBStringArray.
// Extended fields (name, summary, full_description, mitre_*, kill_chain_stage, technical_indicators, impact,
// recommended_mitigations, false_positive_considerations, references) come from Capability Specification (MITRE ATT&CK).
type CapabilityMetadata struct {
	CapabilityID             string           `gorm:"primaryKey;type:varchar(100)" json:"capabilityId"`
	Domain                   string          `gorm:"type:varchar(50);not null;index" json:"domain"`
	Category                 string          `gorm:"type:varchar(100);not null;index" json:"category"`
	Description              string          `gorm:"type:text;not null" json:"description"`
	SeverityBase             string          `gorm:"type:varchar(20);not null" json:"severityBase"`
	ConfidenceBase           float64         `gorm:"type:float;default:0.5" json:"confidenceBase"`
	Preconditions            JSONBStringArray `gorm:"type:jsonb" json:"preconditions,omitempty"`
	ProducesAttackSteps      JSONBStringArray `gorm:"type:jsonb" json:"producesAttackSteps,omitempty"`
	ExpiresWithInstance      bool            `gorm:"default:true" json:"expiresWithInstance"`
	SupportsRuntimePromotion bool            `gorm:"default:true" json:"supportsRuntimePromotion"`
	// Extended (spec)
	Name                      string           `gorm:"type:varchar(200)" json:"name,omitempty"`
	Summary                   string          `gorm:"type:text" json:"summary,omitempty"`
	FullDescription           string          `gorm:"column:full_description;type:text" json:"fullDescription,omitempty"`
	MitreTactic               string          `gorm:"column:mitre_tactic;type:varchar(100)" json:"mitreTactic,omitempty"`
	MitreTechnique            string          `gorm:"column:mitre_technique;type:varchar(50)" json:"mitreTechnique,omitempty"`
	MitreSubtechnique         string          `gorm:"column:mitre_subtechnique;type:varchar(50)" json:"mitreSubtechnique,omitempty"`
	KillChainStage            string          `gorm:"column:kill_chain_stage;type:varchar(80)" json:"killChainStage,omitempty"`
	TechnicalIndicators       JSONBStringArray `gorm:"column:technical_indicators;type:jsonb" json:"technicalIndicators,omitempty"`
	Impact                    JSONBStringArray `gorm:"type:jsonb" json:"impact,omitempty"`
	RecommendedMitigations    JSONBStringArray `gorm:"column:recommended_mitigations;type:jsonb" json:"recommendedMitigations,omitempty"`
	FalsePositiveConsiderations JSONBStringArray `gorm:"column:false_positive_considerations;type:jsonb" json:"falsePositiveConsiderations,omitempty"`
	References                JSONBStringArray `gorm:"column:refs;type:jsonb" json:"references,omitempty"`
	CreatedAt                time.Time       `json:"createdAt"`
	UpdatedAt                time.Time       `json:"updatedAt"`
}

func (CapabilityMetadata) TableName() string {
	return "capability_metadata"
}
