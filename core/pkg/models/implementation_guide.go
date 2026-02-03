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
	Role           string     `json:"role,omitempty"`    // control-plane, worker, etc.
	OS             string     `json:"os,omitempty"`      // OS image (e.g. Ubuntu 22.04)
	Runtime        string     `json:"runtime,omitempty"` // containerd, docker, etc.
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
// Insight represents a security insight or finding
// Updated for Agent-Based architecture with direct resource references
type Insight struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// Resource context (direct references, no JSONB)
	ResourceType      string `gorm:"type:varchar(50);not null;index" json:"resourceType"` // Pod, Node, ServiceAccount, etc.
	ResourceNamespace string `gorm:"type:varchar(255);index" json:"resourceNamespace"`
	ResourceName      string `gorm:"type:varchar(255);not null;index" json:"resourceName"`
	ResourceUID       string `gorm:"type:varchar(255);not null;index" json:"resourceUid"` // Unique identifier

	// Insight info
	InsightType    string `gorm:"type:varchar(50);not null;index" json:"insightType"` // vulnerability, misconfiguration, rbac_risk, etc.
	Severity       string `gorm:"type:varchar(20);not null;index" json:"severity"`    // low, medium, high, critical
	Title          string `gorm:"type:varchar(500);not null" json:"title"`
	Description    string `gorm:"type:text;not null" json:"description"`
	Recommendation string `gorm:"type:text" json:"recommendation"`

	// CVE-specific fields (nullable, only for vulnerability insights)
	CVEID             string  `gorm:"type:varchar(20);index" json:"cveId,omitempty"`
	AffectedComponent string  `gorm:"type:varchar(255);index" json:"affectedComponent,omitempty"` // Package name
	AffectedVersion   string  `gorm:"type:varchar(100)" json:"affectedVersion,omitempty"`
	FixedVersion      string  `gorm:"type:varchar(100)" json:"fixedVersion,omitempty"`
	CVSS              float32 `gorm:"type:decimal(4,1)" json:"cvss,omitempty"` // Changed from *float64

	// Risk Detail: evidence and violated rules (optional; populated by risk engine when available)
	Evidence      string `gorm:"type:jsonb" json:"evidence,omitempty"`
	ViolatedRules string `gorm:"type:jsonb" json:"violatedRules,omitempty"`

	// Status & Timestamps
	Status     string         `gorm:"type:varchar(20);default:active;index" json:"status"` // active, resolved, dismissed
	DetectedAt time.Time      `gorm:"not null;index" json:"detectedAt"`
	ResolvedAt *time.Time     `json:"resolvedAt,omitempty"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
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
