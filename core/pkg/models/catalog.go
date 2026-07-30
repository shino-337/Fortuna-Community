package models

import (
	"time"

	"gorm.io/gorm"
)

// CatalogGeneration records an authoritative load generation for reference data.
type CatalogGeneration struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	CatalogType       string         `gorm:"type:varchar(32);not null;index:idx_catalog_generation_active,priority:1" json:"catalogType"` // cve | malware
	SourceName        string         `gorm:"type:varchar(128);not null;index" json:"sourceName"`
	SourceURL         string         `gorm:"type:text" json:"sourceUrl,omitempty"`
	SourceDigest      string         `gorm:"type:varchar(128);index" json:"sourceDigest,omitempty"`
	Status            string         `gorm:"type:varchar(32);not null;index" json:"status"` // running | active | failed | retired
	RecordCounts      string         `gorm:"type:jsonb;default:'{}'" json:"recordCounts"`
	ErrorSummary      string         `gorm:"type:text" json:"errorSummary,omitempty"`
	StartedAt         time.Time      `gorm:"index" json:"startedAt"`
	CompletedAt       *time.Time     `gorm:"index" json:"completedAt,omitempty"`
	ActivatedAt       *time.Time     `gorm:"index:idx_catalog_generation_active,priority:2" json:"activatedAt,omitempty"`
	DurationMillis    int64          `json:"durationMillis"`
	MirrorVersion     string         `gorm:"type:varchar(128);index" json:"mirrorVersion,omitempty"`
	LoaderVersion     string         `gorm:"type:varchar(64)" json:"loaderVersion,omitempty"`
	ValidationStatus  string         `gorm:"type:varchar(32)" json:"validationStatus,omitempty"`
	ValidationSummary string         `gorm:"type:text" json:"validationSummary,omitempty"`
	CreatedAt         time.Time      `json:"createdAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

func (CatalogGeneration) TableName() string { return "catalog_generations" }
