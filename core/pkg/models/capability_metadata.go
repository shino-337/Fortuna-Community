package models

import (
	"time"
)

// CapabilityMetadata provides semantic layer for capabilities.
// Preconditions and ProducesAttackSteps are JSONB in DB; use JSONBStringArray.
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
	CreatedAt                time.Time       `json:"createdAt"`
	UpdatedAt                time.Time       `json:"updatedAt"`
}

func (CapabilityMetadata) TableName() string {
	return "capability_metadata"
}
