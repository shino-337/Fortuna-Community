package models

import "time"

// SecurityActivityLog is append-only governance telemetry (RBAC v2 / investigation).
// Application code must never UPDATE or DELETE rows; DB triggers enforce immutability where supported.
type SecurityActivityLog struct {
	ID uint `gorm:"primaryKey" json:"id"`

	EventID string `gorm:"size:36;uniqueIndex:idx_sal_event_id" json:"eventId"`

	ActorUserID   uint   `gorm:"index" json:"actorUserId"`
	ActorUsername string `gorm:"size:255;index" json:"actorUsername"`
	ActorRole     string `gorm:"size:64;index" json:"actorRole"`

	PermissionsJSON string `gorm:"type:jsonb" json:"permissionsJson"`

	Action         string `gorm:"size:128;index" json:"action"`
	Resource       string `gorm:"size:128;index" json:"resource"` // legacy / coarse bucket (e.g. http, findings)
	ResourceType   string `gorm:"size:128;index" json:"resourceType"`
	ResourceID     string `gorm:"size:512;index" json:"resourceId"`
	Result         string `gorm:"size:32;index" json:"result"` // success | deny | error
	Severity       string `gorm:"size:32;index" json:"severity"`
	TargetUserID   *uint  `gorm:"index" json:"targetUserId,omitempty"`

	RequestID     string `gorm:"size:128;index" json:"requestId"`
	SessionID     string `gorm:"size:128;index" json:"sessionId"`
	CorrelationID string `gorm:"size:128;index" json:"correlationId"`
	SourceIP      string `gorm:"size:64" json:"sourceIp"`
	UserAgent     string `gorm:"size:512" json:"userAgent"`
	AuthMethod    string `gorm:"size:32;index" json:"authMethod"`

	BeforeStateJSON string `gorm:"type:jsonb" json:"beforeStateJson"`
	AfterStateJSON  string `gorm:"type:jsonb" json:"afterStateJson"`
	DetailsJSON     string `gorm:"type:jsonb" json:"detailsJson"`

	CreatedAt time.Time `json:"createdAt"`
}

func (SecurityActivityLog) TableName() string {
	return "security_activity_logs"
}
