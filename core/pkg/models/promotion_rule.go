package models

import (
	"time"
)

// PromotionRule defines signal -> state promotion rules.
// RequiredCapabilities is JSONB in DB; use JSONBStringArray for correct scan.
type PromotionRule struct {
	ID                   uint              `gorm:"primaryKey" json:"id"`
	CapabilityID         string            `gorm:"type:varchar(100);not null;index" json:"capabilityId"`
	SignalType           string            `gorm:"type:varchar(100);not null;index" json:"signalType"`
	MinOccurrences       int               `gorm:"default:1" json:"minOccurrences"`
	RequiredCapabilities JSONBStringArray  `gorm:"type:jsonb" json:"requiredCapabilities,omitempty"`
	PromoteTo            string            `gorm:"type:varchar(20);not null;index" json:"promoteTo"`
	ConfidenceBoost      float64           `gorm:"default:0.1" json:"confidenceBoost"`
	CreatedAt            time.Time         `json:"createdAt"`
	UpdatedAt            time.Time         `json:"updatedAt"`
}

func (PromotionRule) TableName() string {
	return "promotion_rules"
}
