package models

import "time"

// SBOMMatchRun tracks matcher executions for a given SBOM snapshot and mirror version.
// Primary key: (sbom_id, version, mirror_version) for idempotency.
type SBOMMatchRun struct {
	SBOMID       uint      `gorm:"primaryKey;column:sbom_id"`
	Version      int       `gorm:"primaryKey;column:version"`
	MirrorVersion string   `gorm:"primaryKey;type:varchar(128);column:mirror_version"`
	Status       string    `gorm:"type:varchar(20);not null;default:'running'"` // running | succeeded | failed
	ResolverVersion string `gorm:"type:varchar(32);default:''" json:"resolverVersion"` // matcher resolver_version at match time
	MatcherVersion  string `gorm:"type:varchar(32);default:''" json:"matcherVersion"`  // same as ResolverVersion for now
	CreatedAt    time.Time `gorm:"not null;default:CURRENT_TIMESTAMP"`
}

func (SBOMMatchRun) TableName() string {
	return "sbom_match_runs"
}

