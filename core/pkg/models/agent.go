package models

import (
	"time"

	"gorm.io/gorm"
)

// Agent represents a connected agent instance.
// Agent identity is cluster-qualified: the same agent/node name may legitimately
// exist in different clusters, so AgentID must never be treated as globally unique.
type Agent struct {
	gorm.Model
	ClusterID    string `gorm:"size:255;not null;default:'';uniqueIndex:idx_agents_cluster_agent,priority:1;index:idx_agents_cluster_node,priority:1"`
	AgentID      string `gorm:"uniqueIndex:idx_agents_cluster_agent,priority:2;size:255;not null"`
	NodeName     string `gorm:"size:255;index:idx_agents_cluster_node,priority:2"`
	Version      string `gorm:"size:100"`
	Status       string `gorm:"size:50"`
	Capabilities string `gorm:"type:jsonb"`
	LastSeenAt   *time.Time
}
