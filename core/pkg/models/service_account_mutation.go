package models

import "time"

// ServiceAccountMutation retains the immutable preview and durable progress.
// A completed Kubernetes action can be retried after a database/process failure.
type ServiceAccountMutation struct {
	ID               string     `gorm:"primaryKey;size:64" json:"id"`
	ClusterID        string     `gorm:"not null;index" json:"clusterId"`
	ServiceAccountID uint       `json:"serviceAccountId"`
	UID              string     `gorm:"not null" json:"uid"`
	ActorID          uint       `json:"actorId"`
	Actor            string     `json:"actor"`
	Action           string     `json:"action"`
	Plan             string     `gorm:"type:jsonb" json:"-"`
	Digest           string     `gorm:"size:64" json:"digest"`
	Status           string     `gorm:"index" json:"status"`
	NextStep         int        `json:"completedSteps"`
	Attempts         int        `json:"attempts"`
	LastError        string     `json:"lastError,omitempty"`
	LeaseToken       string     `gorm:"size:64" json:"-"`
	LeaseUntil       *time.Time `json:"-"`
	RetryAt          time.Time  `gorm:"index" json:"retryAt"`
	ExpiresAt        time.Time  `json:"expiresAt"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}
