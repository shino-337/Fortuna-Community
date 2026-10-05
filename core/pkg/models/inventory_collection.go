package models

import (
	"github.com/fortuna/api/collection"
	"time"
)

// InventoryCollection records the latest accepted inventory attempt per cluster.
// It is not runtime coverage, an authoritative deletion, or resolution permission.
type InventoryCollection struct {
	ClusterID     string    `gorm:"primaryKey;size:255" json:"clusterId"`
	AgentID       string    `gorm:"size:255" json:"agentId"`
	CollectionID  string    `gorm:"size:128" json:"collectionId"`
	Namespace     string    `json:"namespace"`
	Status        string    `json:"status"` // complete, failed, or unknown for legacy credentials
	StartedAt     time.Time `json:"startedAt"`
	ObservedAt    time.Time `json:"observedAt"`
	ReceivedAt    time.Time `json:"receivedAt"`
	PayloadSHA256 string    `gorm:"size:64" json:"payloadSha256"`
	Counts        string    `gorm:"type:text" json:"counts"`
	KindStartedAt string    `gorm:"type:text" json:"kindStartedAt"`
	RoleDigests   string    `gorm:"type:text" json:"-"`
	FailureStage  string    `json:"failureStage,omitempty"`
}

func (c InventoryCollection) EffectiveStatus(now time.Time) string {
	if c.Status != "complete" {
		return c.Status
	}
	if c.ObservedAt.IsZero() || c.ObservedAt.After(now) || now.Sub(c.ObservedAt) > collection.MaxAge {
		return "stale"
	}
	return "complete"
}
