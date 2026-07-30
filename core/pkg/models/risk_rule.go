package models

import (
	"time"

	"gorm.io/gorm"
)

// RiskRule stores a risk evaluation rule (CRUD from DB; engine loads from here when present).
// Maps to riskengine.Rule for evaluation.
type RiskRule struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	RuleID      string         `gorm:"uniqueIndex;size:128;not null" json:"ruleId"` // logical id (e.g. esc-priv-pod)
	Name        string         `gorm:"size:256;not null" json:"name"`
	Category    string         `gorm:"size:64;index" json:"category"`
	Severity    string         `gorm:"size:32;index" json:"severity"`
	Description string         `gorm:"type:text" json:"description"`
	Enabled     bool           `gorm:"default:true;index" json:"enabled"`
	Conditions  string         `gorm:"type:text;not null" json:"conditions"`  // JSON array of condition objects
	Aggregation string         `gorm:"size:32;default:AND" json:"aggregation"`
	BaseScore   float64        `gorm:"type:decimal(4,2);default:0" json:"baseScore"`
	Tags        string         `gorm:"type:text" json:"tags"` // JSON array of strings
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name for RiskRule.
func (RiskRule) TableName() string {
	return "risk_rules"
}
