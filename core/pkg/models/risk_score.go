package models

import (
	"time"

	"gorm.io/gorm"
)

// RiskScore represents a calculated risk score for a resource
type RiskScore struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	ResourceType string `gorm:"not null;index" json:"resourceType"`
	ResourceUID  string `gorm:"not null;index" json:"resourceUID"`
	ResourceName string `gorm:"index" json:"resourceName"`
	Namespace    string `gorm:"index" json:"namespace"`
	ClusterID    string `gorm:"not null;index" json:"clusterId"`

	// Scores (0-100)
	TotalScore       float64 `gorm:"type:decimal(5,2);not null;default:0.0;index" json:"totalScore"`
	BaseScore        float64 `gorm:"type:decimal(5,2);not null;default:0.0" json:"baseScore"`
	SeverityWeight   float64 `gorm:"type:decimal(3,2);not null;default:1.0" json:"severityWeight"`   // V1 only
	ImpactMultiplier float64 `gorm:"type:decimal(3,2);not null;default:1.0" json:"impactMultiplier"` // V1 only
	TimeDecay        float64 `gorm:"type:decimal(3,2);not null;default:1.0" json:"timeDecay"`

	// V2 Scoring fields
	ExploitabilityScore float64 `gorm:"type:decimal(5,2);default:0.0" json:"exploitabilityScore"` // V2: 0-30
	BusinessImpactScore float64 `gorm:"type:decimal(5,2);default:0.0" json:"businessImpactScore"` // V2: 0-30
	ScorerVersion       string  `gorm:"type:varchar(10);default:'v1'" json:"scorerVersion"`       // "v1" or "v2"

	// Context
	Factors         string `gorm:"type:jsonb;default:'{}'" json:"factors"` // JSON string
	InsightsCount   int    `gorm:"default:0" json:"insightsCount"`
	HighestSeverity string `gorm:"type:varchar(20)" json:"highestSeverity"`
	PriorityLevel   string `gorm:"type:varchar(10);index" json:"priorityLevel"` // P0, P1, P2, P3, P4

	// Metadata
	CalculatedAt time.Time      `gorm:"index" json:"calculatedAt"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for RiskScore
func (RiskScore) TableName() string {
	return "risk_scores"
}

// PriorityLevel constants
const (
	PriorityP0 = "P0" // Critical: 80-100 (V2), 90-100 (V1)
	PriorityP1 = "P1" // High: 60-79 (V2), 70-89 (V1)
	PriorityP2 = "P2" // Medium: 35-59 (V2), 40-69 (V1)
	PriorityP3 = "P3" // Low: 10-34 (V2), 0-39 (V1)
	PriorityP4 = "P4" // Minimal: 0-9 (V2 only)
)

// GetPriorityLevel returns priority level based on score (V1 thresholds)
func GetPriorityLevel(score float64) string {
	if score >= 90 {
		return PriorityP0
	} else if score >= 70 {
		return PriorityP1
	} else if score >= 40 {
		return PriorityP2
	}
	return PriorityP3
}

// GetPriorityLevelV2 returns priority level based on score (V2 thresholds)
func GetPriorityLevelV2(score float64) string {
	if score >= 80.0 {
		return PriorityP0 // Critical
	} else if score >= 60.0 {
		return PriorityP1 // High
	} else if score >= 35.0 {
		return PriorityP2 // Medium
	} else if score >= 10.0 {
		return PriorityP3 // Low
	}
	return PriorityP4 // Minimal
}
