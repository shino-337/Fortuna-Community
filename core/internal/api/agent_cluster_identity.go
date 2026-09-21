package api

import (
	"context"
	"github.com/fortuna/core/pkg/agentidentity"
	"github.com/fortuna/core/pkg/models"
	"gorm.io/gorm"
)

func upsertAgentClusterIdentity(ctx context.Context, db *gorm.DB, clusterID string, agent *AgentPayload) error {
	if agent == nil || agent.AgentID == "" {
		return nil
	}
	return agentidentity.UpsertRecord(ctx, db, &models.Agent{ClusterID: clusterID, AgentID: agent.AgentID, NodeName: agent.NodeName, Version: agent.Version})
}
