package models

import (
	"time"

	"gorm.io/gorm"
)

// SBOM represents a Software Bill of Materials for a container image
// Updated for Agent-Based architecture
type SBOM struct {
	ID             uint              `gorm:"primaryKey" json:"id"`
	ImageName      string            `gorm:"type:varchar(255);not null;index" json:"imageName"`
	ImageTag       string            `gorm:"type:varchar(255);not null;index" json:"imageTag"`
	ImageDigest    string            `gorm:"type:varchar(255);not null;index" json:"imageDigest"` // SHA256; one row per pod (multiple pods can share same image)
	PodUID         string            `gorm:"type:varchar(255);index" json:"podUid"`
	PodName        string            `gorm:"type:varchar(255)" json:"podName"`
	Namespace      string            `gorm:"type:varchar(255);index" json:"namespace"`
	ContainerName  string            `gorm:"type:varchar(255)" json:"containerName"`
	OSName         string            `gorm:"type:varchar(100)" json:"osName"`
	OSVersion      string            `gorm:"type:varchar(100)" json:"osVersion"`
	OSArchitecture string            `gorm:"type:varchar(50)" json:"osArchitecture"`
	GoVersion      string            `gorm:"type:varchar(50)" json:"goVersion"` // buildinfo.GoVersion for Go stdlib CVE matcher
	PackageCount   int               `gorm:"default:0" json:"packageCount"`
	SBOMFormat     string            `gorm:"type:varchar(50);default:'fortuna-agent'" json:"sbomFormat"`
	SBOMContent    string            `gorm:"type:jsonb" json:"sbomContent"` // Optional JSON content
	GeneratedAt    time.Time         `gorm:"index" json:"generatedAt"`
	AgentID        string            `gorm:"type:varchar(255);index" json:"agentId"`
	NodeID         string            `gorm:"type:varchar(255);index" json:"nodeId"`
	Labels         map[string]string `gorm:"type:jsonb;serializer:json" json:"labels"`
	Annotations    map[string]string `gorm:"type:jsonb;serializer:json" json:"annotations"`
	LastUsedAt     time.Time         `gorm:"index" json:"lastUsedAt"`
	UseCount       int               `gorm:"default:1" json:"useCount"`
	SbomSource     string            `gorm:"type:varchar(64)" json:"sbomSource"`     // Finding #8.4: parsers | distroless-heuristic | label-metadata
	Confidence     string            `gorm:"type:varchar(32)" json:"confidence"`    // Finding #8.4: low | medium | high
	Status         string            `gorm:"type:varchar(20);default:'pending';index" json:"status"` // pending | finalized
	Version        int               `gorm:"type:integer;default:1" json:"version"`                  // SBOM snapshot version
	CreatedAt      time.Time         `json:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt"`
	DeletedAt      gorm.DeletedAt    `gorm:"index" json:"-"`

	// Relationships
	Components []SBOMComponent `gorm:"foreignKey:SBOMID" json:"components,omitempty"`
	CVEMatches []CVEMatch      `gorm:"foreignKey:SBOMID" json:"cveMatches,omitempty"`
}

// TableName specifies the table name for SBOM
func (SBOM) TableName() string {
	return "sboms"
}

// SBOMComponent represents a component (package) extracted from SBOM
type SBOMComponent struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	SBOMID           uint           `gorm:"not null;index" json:"sbomId"`
	ComponentType    string         `gorm:"type:varchar(50);not null" json:"componentType"` // library, application, os
	ComponentName    string         `gorm:"type:varchar(255);not null;index" json:"componentName"`
	ComponentVersion string         `gorm:"type:varchar(255);not null" json:"componentVersion"`
	PURL             string         `gorm:"type:varchar(512);column:purl;index" json:"purl"` // Package URL (standard)
	Licenses         string         `gorm:"type:text" json:"licenses"`                       // Comma-separated licenses
	Source           string         `gorm:"type:varchar(500)" json:"source"`
	Description      string         `gorm:"type:text" json:"description"`
	Homepage         string         `gorm:"type:varchar(500)" json:"homepage"`
	Maintainer       string         `gorm:"type:varchar(255)" json:"maintainer"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	SBOM SBOM `gorm:"foreignKey:SBOMID" json:"sbom,omitempty"`
}

// TableName specifies the table name for SBOMComponent
func (SBOMComponent) TableName() string {
	return "sbom_components"
}

// CVEMatch represents a CVE matched to an SBOM component
// Updated for Agent-Based architecture - contains package info directly
type CVEMatch struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	SBOMID uint `gorm:"not null;index" json:"sbomId"`

	// Pod context (from Agent)
	PodUID        string `gorm:"type:varchar(255);index" json:"podUid"`
	ContainerName string `gorm:"type:varchar(255)" json:"containerName"`

	// CVE info
	CVEID string `gorm:"type:varchar(100);not null;index" json:"cveId"`

	// Package info (direct from Agent, no ComponentID needed)
	PackageName    string `gorm:"type:varchar(255);not null;index" json:"packageName"`
	PackageVersion string `gorm:"type:varchar(100);not null" json:"packageVersion"`
	PURL           string `gorm:"type:varchar(500)" json:"purl"`

	// Severity & Fix
	Severity     string  `gorm:"type:varchar(20);not null;index" json:"severity"`
	CVSS         float32 `gorm:"type:decimal(4,1)" json:"cvss"` // Changed from *float64
	FixedVersion string  `gorm:"type:varchar(255)" json:"fixedVersion"`
	MatchedBy    string  `gorm:"type:varchar(255)" json:"matchedBy"` // Version range that matched

	// Timestamps
	MatchedAt time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP" json:"matchedAt"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	SBOM SBOM `gorm:"foreignKey:SBOMID" json:"sbom,omitempty"`
	CVE  CVE  `gorm:"foreignKey:CVEID;references:CVEID" json:"cve,omitempty"`
}

// TableName specifies the table name for CVEMatch
func (CVEMatch) TableName() string {
	return "cve_matches"
}
