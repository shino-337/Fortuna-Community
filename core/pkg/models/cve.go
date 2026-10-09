package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// ImageScanResult stores scan results from Trivy or other scanners
type ImageScanResult struct {
	ID                   uint           `gorm:"primaryKey" json:"id"`
	ImageName            string         `gorm:"type:varchar(255);not null;index" json:"imageName"`
	ImageTag             string         `gorm:"type:varchar(50);not null;index" json:"imageTag"`
	ImageDigest          string         `gorm:"type:varchar(71);index" json:"imageDigest"`
	Registry             string         `gorm:"type:varchar(255)" json:"registry"`
	FullImageRef         string         `gorm:"type:text" json:"fullImageRef"`
	ScannedAt            time.Time      `gorm:"index" json:"scannedAt"`
	ScannerName          string         `gorm:"type:varchar(50);default:'trivy'" json:"scannerName"`
	ScannerVersion       string         `gorm:"type:varchar(50)" json:"scannerVersion"`
	ScanDurationSeconds  float64        `gorm:"type:decimal(10,2)" json:"scanDurationSeconds"`
	OSFamily             string         `gorm:"type:varchar(50)" json:"osFamily"`
	OSName               string         `gorm:"type:varchar(100)" json:"osName"`
	OSVersion            string         `gorm:"type:varchar(50)" json:"osVersion"`
	TotalVulnerabilities int            `gorm:"default:0" json:"totalVulnerabilities"`
	CriticalCount        int            `gorm:"default:0;index" json:"criticalCount"`
	HighCount            int            `gorm:"default:0;index" json:"highCount"`
	MediumCount          int            `gorm:"default:0" json:"mediumCount"`
	LowCount             int            `gorm:"default:0" json:"lowCount"`
	UnknownCount         int            `gorm:"default:0" json:"unknownCount"`
	Vulnerabilities      string         `gorm:"type:jsonb" json:"vulnerabilities"`                          // Full JSON array
	Packages             string         `gorm:"type:jsonb" json:"packages"`                                 // All packages found
	Status               string         `gorm:"type:varchar(20);default:'in_progress';index" json:"status"` // in_progress, completed, failed
	ErrorMessage         string         `gorm:"type:text" json:"errorMessage"`
	CacheKey             string         `gorm:"type:varchar(100);index" json:"cacheKey"`
	ExpiresAt            *time.Time     `gorm:"index" json:"expiresAt"`
	CreatedAt            time.Time      `json:"createdAt"`
	UpdatedAt            time.Time      `json:"updatedAt"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	PodImageScans []PodImageScan `gorm:"foreignKey:ScanResultID" json:"podImageScans,omitempty"`
}

// ToJSONBString converts a Go value to JSONB string for PostgreSQL
func ToJSONBString(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// TableName specifies the table name for ImageScanResult
func (ImageScanResult) TableName() string {
	return "image_scan_results"
}

// PodImageScan maps pods to their image scan results
type PodImageScan struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	PodUID         string         `gorm:"type:varchar(255);not null;index" json:"podUid"`
	PodName        string         `gorm:"type:varchar(255);not null" json:"podName"`
	PodNamespace   string         `gorm:"type:varchar(255);not null;index" json:"podNamespace"`
	ClusterID      string         `gorm:"type:varchar(255);not null;index" json:"clusterId"`
	ContainerName  string         `gorm:"type:varchar(255);not null" json:"containerName"`
	ContainerImage string         `gorm:"type:text;not null" json:"containerImage"`
	ImageName      string         `gorm:"type:varchar(255);index" json:"imageName"`
	ImageTag       string         `gorm:"type:varchar(50);index" json:"imageTag"`
	ImageRegistry  string         `gorm:"type:varchar(255)" json:"imageRegistry"`
	ScanResultID   *uint          `gorm:"index" json:"scanResultId"`
	SBOMID         *uint          `gorm:"index" json:"sbomId"` // Reference to SBOM
	PodPhase       string         `gorm:"type:varchar(20)" json:"podPhase"`
	PodCreatedAt   *time.Time     `json:"podCreatedAt"`
	PodDeletedAt   *time.Time     `json:"podDeletedAt"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	// Relationships
	ScanResult ImageScanResult `gorm:"foreignKey:ScanResultID" json:"scanResult,omitempty"`
}

// TableName specifies the table name for PodImageScan
func (PodImageScan) TableName() string {
	return "pod_image_scans"
}
