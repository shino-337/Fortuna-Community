package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/fortuna/core/internal/ingest"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/internal/service"
	"github.com/fortuna/core/pkg/agentidentity"
	"github.com/fortuna/core/pkg/capability"
	"github.com/fortuna/core/pkg/worker"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ClusterPayload is the agent → core contract (SSOT).
type ClusterPayload struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Source       string `json:"source"` // "auto" | "env"
	K8sVersion   string `json:"k8s_version,omitempty"`
	Distribution string `json:"distribution,omitempty"`
}

type AgentPayload struct {
	AgentID  string `json:"agentId"`
	NodeName string `json:"nodeName"`
	Version  string `json:"version,omitempty"`
}

func validateScopedSyncClaims(c *gin.Context, legacyClusterID string, cluster *ClusterPayload, agent *AgentPayload) error {
	principal, scoped := middleware.AgentPrincipal(c)
	if !scoped {
		return nil // Explicit legacy mode selected by route middleware.
	}
	legacyClusterID = strings.TrimSpace(legacyClusterID)
	clusterObjectID := ""
	if cluster != nil {
		clusterObjectID = strings.TrimSpace(cluster.ID)
	}
	if legacyClusterID != "" && clusterObjectID != "" && legacyClusterID != clusterObjectID {
		return errors.New("conflicting cluster identity aliases")
	}
	clusterID := clusterObjectID
	if clusterID == "" {
		clusterID = legacyClusterID
	}
	if clusterID == "" || agent == nil || strings.TrimSpace(agent.AgentID) == "" {
		return errors.New("scoped sync requires cluster and agent identity claims")
	}
	if err := principal.CheckClaims(clusterID, strings.TrimSpace(agent.AgentID)); err != nil {
		return agentidentity.ErrIdentityMismatch
	}
	return nil
}

// SyncDataFromAgent handles data sync from agent via HTTP.
// Accepts cluster object (id, name, source, k8s_version, distribution) or legacy clusterId/clusterName.
// When clusterLimiter is non-nil, enforces per-cluster rate limit (Finding #6).
func SyncDataFromAgent(db *gorm.DB, clusterLimiter *ingest.ClusterRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ClusterID   string                 `json:"clusterId"`
			ClusterName string                 `json:"clusterName"`
			Cluster     *ClusterPayload        `json:"cluster"`
			Agent       *AgentPayload          `json:"agent"`
			Data        map[string]interface{} `json:"data" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := validateScopedSyncClaims(c, req.ClusterID, req.Cluster, req.Agent); err != nil {
			if errors.Is(err, agentidentity.ErrIdentityMismatch) {
				c.JSON(http.StatusForbidden, gin.H{"error": "authenticated agent identity does not match sync payload", "code": "agent_identity_mismatch"})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "code": "invalid_agent_identity_claims"})
			return
		}

		clusterID := strings.TrimSpace(req.ClusterID)
		clusterName := req.ClusterName
		source := ""
		k8sVersion := ""
		distribution := ""
		if req.Cluster != nil {
			clusterID = strings.TrimSpace(req.Cluster.ID)
			clusterName = req.Cluster.Name
			source = req.Cluster.Source
			k8sVersion = req.Cluster.K8sVersion
			distribution = req.Cluster.Distribution
		}
		if clusterID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "clusterId or cluster.id required"})
			return
		}
		if clusterName == "" {
			clusterName = clusterID
		}
		// Normalize cluster_id only after scoped identity has been validated. This prevents
		// an alias from being normalized into a cluster the authenticated principal did not claim.
		clusterID = NormalizeClusterID(db, clusterID)

		if clusterLimiter != nil && !clusterLimiter.AllowSync(clusterID) {
			log.Printf("[AgentAPI] rate limit exceeded for cluster=%s", clusterID)
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded for cluster", "cluster_id": clusterID})
			return
		}

		_, hasDelta := req.Data["isDeltaSync"]
		_, hasFull := req.Data["isFullSync"]
		log.Printf("[AgentAPI] cluster=%s name=%s source=%s hasDelta=%v hasFull=%v",
			clusterID, clusterName, source, hasDelta, hasFull)

		if err := upsertAgentClusterIdentity(c.Request.Context(), db, clusterID, req.Agent); err != nil {
			if errors.Is(err, errAgentClusterConflict) {
				c.JSON(409, gin.H{"error": "agent ID is assigned to another cluster"})
				return
			}
			c.JSON(500, gin.H{"error": "Unable to persist agent identity"})
			return
		}

		traceID := c.GetHeader("X-Correlation-ID")
		if traceID == "" {
			traceID = c.GetHeader("x-correlation-id")
		}
		agentService := service.NewAgentService(db)
		if err := agentService.SyncData(clusterID, clusterName, source, k8sVersion, distribution, req.Data, traceID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if isFull, ok := req.Data["isFullSync"].(bool); ok && isFull {
			go func(cid string) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()
				log.Printf("[AgentAPI] Triggering historical risk evaluation for cluster=%s", cid)
				evaluator := worker.NewHistoricalRiskEvaluator(db)
				if err := evaluator.EvaluateAllResources(ctx); err != nil {
					log.Printf("[AgentAPI] Risk evaluation failed: %v", err)
				}
			}(clusterID)

			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()
				log.Printf("[AgentAPI] Triggering PodCapabilityEngine evaluation")
				if err := capability.EvaluateAllPods(ctx, db); err != nil {
					log.Printf("[AgentAPI] PCE evaluation failed: %v", err)
				}
			}()
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Data synced successfully",
		})
	}
}
