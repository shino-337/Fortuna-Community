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

	// Legacy sub-scores kept for backward data compatibility.
	ExploitabilityScore float64 `gorm:"type:decimal(5,2);default:0.0" json:"exploitabilityScore"`
	BusinessImpactScore float64 `gorm:"type:decimal(5,2);default:0.0" json:"businessImpactScore"`
	ScorerVersion       string  `gorm:"type:varchar(10);default:'v3'" json:"scorerVersion"`

	// V3 Scoring dimension fields (Unified Scorer — Phase 2.3)
	CapabilityExposureScore float64 `gorm:"type:float;default:0" json:"capabilityExposureScore"` // V3: 0-15
	AttackPathScore         float64 `gorm:"type:float;default:0" json:"attackPathScore"`         // V3: 0-15
	RuntimeThreatScore      float64 `gorm:"type:float;default:0" json:"runtimeThreatScore"`      // V3: 0-15
	BlastRadiusScore        float64 `gorm:"type:float;default:0" json:"blastRadiusScore"`        // V3: 0-10

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

// PriorityLevel constants (legacy P-band label retained for storage compatibility).
const (
	PriorityP0 = "P0" // Critical: 80-100 (V2), 90-100 (V1)
	PriorityP1 = "P1" // High: 60-79 (V2), 70-89 (V1)
	PriorityP2 = "P2" // Medium: 35-59 (V2), 40-69 (V1)
	PriorityP3 = "P3" // Low: 10-34 (V2), 0-39 (V1)
	PriorityP4 = "P4" // Minimal: 0-9 (V2 only)
)

// GetPriorityLevel returns legacy P0–P3 labels from total_score using historical P-band thresholds (P0≥90, etc.).
// For UI “risk level” bands (low/medium/high/critical from 0–100), use github.com/fortuna/core/pkg/risk.DeriveFinalLevelFromScore instead.
//
// Deprecated for unified UX: do not map this string to severity badges interchangeably with final_level.
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

