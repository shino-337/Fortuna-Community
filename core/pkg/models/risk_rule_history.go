package models

import "time"

// RiskRuleHistory stores a versioned snapshot of a risk rule before each update (Phase 3 rule versioning).
type RiskRuleHistory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	RuleID    string    `gorm:"type:varchar(128);not null;index:idx_risk_rules_history_rule_id" json:"ruleId"`
	Version   int       `gorm:"not null" json:"version"`
	Snapshot  string    `gorm:"type:jsonb;not null" json:"snapshot"` // JSON of rule state before update
	CreatedAt time.Time `json:"createdAt"`
}

// TableName returns the table name for RiskRuleHistory.
func (RiskRuleHistory) TableName() string {
	return "risk_rules_history"
}
