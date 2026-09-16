package models

import "time"

// AttackPath persists a computed attack path so it can be read by the Unified
// Scorer (Layer 4) and displayed in the Dashboard without re-computing on every
// request.
//
// Paths are identified by (ClusterID, PodUID, PathID). Existing write paths are
// migrated to the cluster-qualified key in C3c query migration.
type AttackPath struct {
	ID          uint    `gorm:"primaryKey" json:"id"`
	ClusterID   string  `gorm:"type:varchar(255);index" json:"clusterId,omitempty"`
	PodUID      string  `gorm:"type:varchar(255);not null;index" json:"podUid"`
	PathID      string  `gorm:"type:varchar(255);not null;index" json:"pathId"`
	Nodes       string  `gorm:"type:jsonb;not null;default:'[]'" json:"nodes"`
	Edges       string  `gorm:"type:jsonb;not null;default:'[]'" json:"edges"`
	TotalRisk   float64 `gorm:"type:float;not null;default:0" json:"totalRisk"`
	Difficulty  float64 `gorm:"type:float;not null;default:1" json:"difficulty"`
	Impact      float64 `gorm:"type:float;not null;default:0" json:"impact"`
	Length      int     `gorm:"not null;default:0" json:"length"`
	Description string  `gorm:"type:text" json:"description"`
	// EnrichedFromPCE is true when capability/attack-step edges were added
	// during Attack Path enrichment (Phase 1.2 / PCE-1).
	EnrichedFromPCE bool      `gorm:"not null;default:false" json:"enrichedFromPce"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// TableName overrides GORM's default table name.
func (AttackPath) TableName() string {
	return "attack_paths"
}
