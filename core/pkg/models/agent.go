package models

import (
	"time"

	"gorm.io/gorm"
)

// Agent identity is the exact (cluster_id, agent_id) pair. Unowned legacy rows
// remain quarantined and are never implicitly adopted by a credential.
type Agent struct {
	gorm.Model
	ClusterID    string `gorm:"size:255;not null;default:'';index:idx_agents_cluster_node,priority:1;uniqueIndex:idx_agents_cluster_agent,priority:1"`
	AgentID      string `gorm:"uniqueIndex:idx_agents_cluster_agent,priority:2;size:255;not null"`
	NodeName     string `gorm:"size:255;index:idx_agents_cluster_node,priority:2"`
	Version      string `gorm:"size:100"`
	Status       string `gorm:"size:50"`
	Capabilities string `gorm:"type:jsonb"`
	LastSeenAt   *time.Time
}
