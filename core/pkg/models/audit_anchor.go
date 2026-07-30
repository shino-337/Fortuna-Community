package models

import "time"

// AuditAnchor stores periodic hash-chain anchors for tamper-evident audit logs.
type AuditAnchor struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	AnchorHash string    `gorm:"type:varchar(128);not null;index" json:"anchorHash"`
	CreatedAt  time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"createdAt"`
}

func (AuditAnchor) TableName() string {
	return "audit_anchor"
}

