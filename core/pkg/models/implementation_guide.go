package models

import (
	"time"

	"gorm.io/gorm"
)

// Node represents a Kubernetes node
type Node struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	ClusterID      string     `gorm:"not null;index" json:"clusterId"`
	NodeName       string     `gorm:"not null;index" json:"nodeName"`
	IP             string     `json:"ip"`
	KubeletVersion string     `json:"kubeletVersion"`
	LastSeen       *time.Time `json:"lastSeen"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`

	Cluster Cluster `gorm:"foreignKey:ClusterID" json:"cluster,omitempty"`
	Pods    []Pod   `gorm:"foreignKey:NodeID" json:"pods,omitempty"`
}

// Policy represents a generated policy (KubeArmor, NetworkPolicy, etc.)
type Policy struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	Type        string    `gorm:"not null;index" json:"type"` // e.g., "KubeArmor", "NetworkPolicy"
	PolicyYAML  string    `gorm:"type:text;not null" json:"policyYaml"`
	GeneratedBy string    `json:"generatedBy"`                         // e.g., "risk_engine", "policy_engine"
	Status      string    `gorm:"default:pending;index" json:"status"` // pending, approved, applied, rejected
	CreatedBy   string    `json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Insight represents a risk insight or security finding
type Insight struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	Type              string         `gorm:"not null;index" json:"type"` // e.g., "rbac_risk", "network_risk", "runtime_anomaly", "vulnerability"
	Description       string         `gorm:"not null" json:"description"`
	AffectedResources string         `gorm:"type:jsonb" json:"affectedResources"` // JSON array
	Severity          string         `gorm:"not null;index" json:"severity"`      // low, medium, high, critical
	RecommendedAction string         `json:"recommendedAction"`
	Status            string         `gorm:"default:active;index" json:"status"` // active, resolved, dismissed
	CreatedAt         time.Time      `gorm:"index" json:"createdAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"` // Soft delete support

	// Source field to track where insight came from
	Source string `gorm:"type:varchar(50);index" json:"source,omitempty"` // policy-engine, cve-scanner, correlation-engine, manual

	// CVE-specific fields (nullable, only for vulnerability insights)
	CVEID            string   `gorm:"type:varchar(20);index" json:"cveId,omitempty"` // CVE-2021-23017
	CVSSScore        *float64 `gorm:"type:decimal(3,1)" json:"cvssScore,omitempty"`
	CVSSVector       string   `gorm:"type:text" json:"cvssVector,omitempty"`
	ExploitAvailable bool     `gorm:"default:false;index" json:"exploitAvailable,omitempty"`
	PackageName      string   `gorm:"type:varchar(255);index" json:"packageName,omitempty"`
	InstalledVersion string   `gorm:"type:varchar(50)" json:"installedVersion,omitempty"`
	FixedVersion     string   `gorm:"type:varchar(50)" json:"fixedVersion,omitempty"`

	// SBOM references (nullable, only for SBOM-based insights)
	SBOMID     *uint `gorm:"index" json:"sbomId,omitempty"`
	CVEMatchID *uint `gorm:"index" json:"cveMatchId,omitempty"`
}

// EventIndex represents an index entry pointing to raw events in ClickHouse/Timescale
type EventIndex struct {
	EventID   string    `gorm:"primaryKey" json:"eventId"`
	ClusterID string    `gorm:"index" json:"clusterId"`
	NodeID    *uint     `gorm:"index" json:"nodeId"`
	PodUID    string    `gorm:"index" json:"podUid"`
	TS        time.Time `gorm:"not null;index" json:"ts"`
	Summary   string    `gorm:"type:jsonb" json:"summary"` // JSON summary
	CreatedAt time.Time `json:"createdAt"`

	Cluster *Cluster `gorm:"foreignKey:ClusterID" json:"cluster,omitempty"`
	Node    *Node    `gorm:"foreignKey:NodeID" json:"node,omitempty"`
}

// TableName overrides table names
func (Node) TableName() string {
	return "nodes"
}

func (Policy) TableName() string {
	return "policies"
}

func (Insight) TableName() string {
	return "insights"
}

func (EventIndex) TableName() string {
	return "events_index"
}
