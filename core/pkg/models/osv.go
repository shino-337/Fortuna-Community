package models

import "time"

// OSVVulnerability stores high-level OSV vulnerability metadata.
// This is a mirror table for OSV JSON, focused initially on Go ecosystem (P2-7).
type OSVVulnerability struct {
	ID          string    `gorm:"primaryKey;type:varchar(100)" json:"id"` // OSV ID, e.g. GO-2023-1234
	Summary     string    `gorm:"type:text" json:"summary"`
	Details     string    `gorm:"type:text" json:"details"`
	Severity    string    `gorm:"type:varchar(20);index" json:"severity"`
	CVSSScore   float64   `gorm:"type:decimal(4,1)" json:"cvssScore"`
	PublishedAt time.Time `json:"publishedAt"`
	ModifiedAt  time.Time `json:"modifiedAt"`
	Source      string    `gorm:"type:varchar(50);default:'osv'" json:"source"`
	Aliases     string    `gorm:"type:text" json:"aliases"` // optional: JSON array of aliases (CVE, GHSA, etc.)
}

func (OSVVulnerability) TableName() string {
	return "osv_vulnerabilities"
}

// OSVPackage represents (ecosystem, package_name) pairs affected by an OSV vulnerability.
type OSVPackage struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	VulnID      string `gorm:"type:varchar(100);not null;index" json:"vulnId"`
	Ecosystem   string `gorm:"type:varchar(50);not null;index" json:"ecosystem"`
	PackageName string `gorm:"type:varchar(255);not null;index" json:"packageName"`
}

func (OSVPackage) TableName() string {
	return "osv_packages"
}

// OSVRange flattens OSV "ranges[].events" for a given package into a simple introduced/fixed/last_affected row.
type OSVRange struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	PackageID    uint   `gorm:"not null;index" json:"packageId"`
	RangeType    string `gorm:"type:varchar(20);not null" json:"rangeType"` // e.g. "SEMVER"
	Introduced   string `gorm:"type:varchar(64)" json:"introduced"`
	Fixed        string `gorm:"type:varchar(64)" json:"fixed"`
	LastAffected string `gorm:"type:varchar(64)" json:"lastAffected"`
}

func (OSVRange) TableName() string {
	return "osv_ranges"
}

// GoModuleAlias maps an old/alias Go module path to the canonical one used in OSV (P2 alias resolver).
// Example: github.com/coreos/etcd -> go.etcd.io/etcd.
type GoModuleAlias struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	Alias     string `gorm:"type:varchar(512);not null;uniqueIndex:idx_go_module_alias_alias" json:"alias"`
	Canonical string `gorm:"type:varchar(512);not null;index" json:"canonical"`
}

func (GoModuleAlias) TableName() string {
	return "go_module_alias"
}

// MirrorState stores version/epoch for mirror caches (e.g. OSV). When version increments, cache keys
// that include it automatically miss, so no explicit invalidation is needed (multi-worker safe).
type MirrorState struct {
	Name      string    `gorm:"primaryKey;type:varchar(64)" json:"name"`   // e.g. "osv"
	Version   int64     `gorm:"not null;default:0" json:"version"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

func (MirrorState) TableName() string {
	return "mirror_state"
}
