package models

import (
	"time"

	"gorm.io/gorm"
)

// SBOM represents a Software Bill of Materials for a container image
type SBOM struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	ImageName         string         `gorm:"type:varchar(255);not null;index" json:"imageName"`
	ImageTag          string         `gorm:"type:varchar(255);not null;index" json:"imageTag"`
	ImageDigest       string         `gorm:"type:varchar(255);not null;uniqueIndex" json:"imageDigest"` // SHA256, immutable!
	SBOMFormat        string         `gorm:"type:varchar(50);not null;default:'cyclonedx-json'" json:"sbomFormat"`
	SBOMContent       string         `gorm:"type:jsonb;not null" json:"sbomContent"` // CycloneDX JSON
	ComponentCount    int            `gorm:"not null;default:0" json:"componentCount"`
	OSPackages        int            `gorm:"default:0" json:"osPackages"`
	LanguagePackages  int            `gorm:"default:0" json:"languagePackages"`
	Generator         string         `gorm:"type:varchar(100);default:'syft'" json:"generator"`
	GeneratorVersion  string         `gorm:"type:varchar(50)" json:"generatorVersion"`
	GeneratedAt       time.Time      `gorm:"not null;index" json:"generatedAt"`
	LastUsedAt        time.Time      `gorm:"not null;index" json:"lastUsedAt"`
	UseCount          int            `gorm:"default:1" json:"useCount"`
	CreatedAt         time.Time      `json:"createdAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`

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
	ID              uint           `gorm:"primaryKey" json:"id"`
	SBOMID          uint           `gorm:"not null;index" json:"sbomId"`
	ComponentType  string         `gorm:"type:varchar(50);not null" json:"componentType"` // library, application, os
	ComponentName   string         `gorm:"type:varchar(255);not null;index" json:"componentName"`
	ComponentVersion string        `gorm:"type:varchar(255);not null" json:"componentVersion"`
	PURL             string         `gorm:"type:varchar(512);column:purl;index" json:"purl"` // Package URL (standard)
	Licenses         string         `gorm:"type:jsonb" json:"licenses"`         // JSON array
	Supplier         string         `gorm:"type:varchar(255)" json:"supplier"`
	CreatedAt        time.Time      `json:"createdAt"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	SBOM      SBOM       `gorm:"foreignKey:SBOMID" json:"sbom,omitempty"`
	CVEMatches []CVEMatch `gorm:"foreignKey:ComponentID" json:"cveMatches,omitempty"`
}

// TableName specifies the table name for SBOMComponent
func (SBOMComponent) TableName() string {
	return "sbom_components"
}

// CVE is a minimal CVE reference for agent-side CVEMatch relation.
// Full CVE data lives in Core; this is only for GORM relation/foreign key.
type CVE struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	CVEID  string `gorm:"type:varchar(100);uniqueIndex;not null" json:"cveId"`
}

// TableName specifies the table name for CVE
func (CVE) TableName() string {
	return "cves"
}

// CVEMatch represents a CVE matched to an SBOM component
type CVEMatch struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	SBOMID        uint           `gorm:"not null;index" json:"sbomId"`
	ComponentID   uint           `gorm:"not null;index" json:"componentId"`
	CVEID         string         `gorm:"type:varchar(20);not null;index" json:"cveId"`
	Severity      string         `gorm:"type:varchar(20);not null;index" json:"severity"`
	CVSSScore     *float64       `gorm:"type:decimal(3,1)" json:"cvssScore"`
	FixedVersion  string         `gorm:"type:varchar(255)" json:"fixedVersion"`
	MatchedAt     time.Time      `gorm:"not null;default:CURRENT_TIMESTAMP" json:"matchedAt"`
	Matcher       string         `gorm:"type:varchar(50);default:'grype'" json:"matcher"`
	DBVersion     string         `gorm:"type:varchar(50)" json:"dbVersion"` // Grype DB version used
	NeedsRecheck  bool           `gorm:"default:false;index" json:"needsRecheck"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	SBOM      SBOM          `gorm:"foreignKey:SBOMID" json:"sbom,omitempty"`
	Component SBOMComponent `gorm:"foreignKey:ComponentID" json:"component,omitempty"`
	CVE       CVE           `gorm:"foreignKey:CVEID;references:CVEID" json:"cve,omitempty"`
}

// TableName specifies the table name for CVEMatch
func (CVEMatch) TableName() string {
	return "cve_matches"
}


