package models

import "time"

// UserSession binds interactive JWTs to server-side revocation and idle tracking (RBAC governance).
type UserSession struct {
	ID        string `gorm:"primaryKey;type:char(36)" json:"id"`
	UserID    uint   `gorm:"not null;index:idx_user_sessions_user" json:"userId"`
	IssuedAt  time.Time
	ExpiresAt time.Time
	// RevokedAt non-nil means session is invalid (logout / admin revoke).
	RevokedAt *time.Time `gorm:"column:revoked_at" json:"revokedAt,omitempty"`

	LastActivityAt time.Time

	SourceIP          string `gorm:"size:64"`
	UserAgent         string `gorm:"size:512"`
	DeviceFingerprint string `gorm:"size:128;index"`
	AuthMethod        string `gorm:"size:32"`
}

func (UserSession) TableName() string {
	return "user_sessions"
}
