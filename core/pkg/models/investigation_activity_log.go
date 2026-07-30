package models

import "time"

// InvestigationActivityLog is append-only per-case timeline (immutable at DB layer where supported).
type InvestigationActivityLog struct {
	ID uint `gorm:"primaryKey" json:"id"`

	CaseID string `gorm:"size:64;index;not null" json:"caseId"`
	EventID string `gorm:"size:36;uniqueIndex" json:"eventId"`

	ActorUserID   uint   `gorm:"index" json:"actorUserId"`
	ActorUsername string `gorm:"size:255;index" json:"actorUsername"`

	EventType string `gorm:"size:64;index;not null" json:"eventType"`
	Summary   string `gorm:"size:1024" json:"summary"`

	BeforeJSON string `gorm:"type:jsonb" json:"-"`
	AfterJSON  string `gorm:"type:jsonb" json:"-"`
	DetailsJSON string `gorm:"type:jsonb" json:"-"`

	CorrelationID string `gorm:"size:128;index" json:"correlationId,omitempty"`
	RequestID     string `gorm:"size:128;index" json:"requestId,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}

func (InvestigationActivityLog) TableName() string {
	return "investigation_activity_logs"
}
