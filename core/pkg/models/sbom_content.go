package models

import (
	"fmt"
	"gorm.io/gorm"
	"time"
)

// SBOMImageContent is an immutable, resource-independent snapshot. Its key includes
// extraction provenance and package content, not merely an image digest.
// Ownership, lifecycle, match runs and findings remain on the SBOM observation.
type SBOMImageContent struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ContentHash string    `gorm:"size:64;not null;uniqueIndex" json:"contentHash"`
	Payload     string    `gorm:"type:text;not null" json:"-"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (SBOMImageContent) TableName() string { return "sbom_image_contents" }
func (*SBOMImageContent) BeforeUpdate(*gorm.DB) error {
	return fmt.Errorf("SBOM image content is immutable")
}
