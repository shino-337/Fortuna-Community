package models

import (
	"time"

	"gorm.io/gorm"
)

// InvestigationCase is a persisted SOC investigation workspace (entities/notes/remediation stored as JSON).
type InvestigationCase struct {
	ID              string         `gorm:"primaryKey;size:64" json:"id"`
	Title           string         `gorm:"size:512;not null" json:"title"`
	Status          string         `gorm:"size:32;index;not null" json:"status"`
	Owner           string         `gorm:"size:255;index" json:"owner"`
	ClusterID       *string        `gorm:"size:64;index" json:"clusterId,omitempty"`
	CreatedByUserID uint           `gorm:"index;not null" json:"createdByUserId"`
	EntitiesJSON    string         `gorm:"type:jsonb" json:"-"`
	NotesJSON       string         `gorm:"type:jsonb" json:"-"`
	RemediationJSON     string         `gorm:"type:jsonb" json:"-"`
	CollaborationJSON   string         `gorm:"type:jsonb" json:"-"`
	SLADueAt            *time.Time     `json:"slaDueAt,omitempty"`
	ArchivedAt          *time.Time     `gorm:"index" json:"archivedAt,omitempty"`
	RetentionUntil      *time.Time     `gorm:"index" json:"retentionUntil,omitempty"`
	CreatedAt           time.Time      `json:"createdAt"`
	UpdatedAt           time.Time      `json:"updatedAt"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`
}

func (InvestigationCase) TableName() string {
	return "investigation_cases"
}
