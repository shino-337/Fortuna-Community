package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/graph"
	"github.com/fortuna/core/pkg/models"
)

var errGraphRequestAborted = errors.New("graph request aborted")

// GetGraph returns relational graph data within the authorized cluster scope.
func GetGraph(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID, err := graphClusterIDOrDefault(db, c)
		if err != nil {
			return
		}
		data, err := graph.NewRelationalPathBuilder(db).BuildGraphData(c.Request.Context(), clusterID)
		if err != nil {
			c.JSON(500, gin.H{"error": "Unable to build graph from inventory"})
			return
		}
		viewerGraphJSON(c, 200, gin.H{"message": "Graph from synchronized inventory", "data": data})
	}
}

func graphClusterIDOrDefault(db *gorm.DB, c *gin.Context) (string, error) {
	scope, ok := resolveRiskGovernanceScope(db, c)
	if !ok {
		return "", errGraphRequestAborted
	}
	legacy := strings.TrimSpace(c.Query("cluster_id"))
	if legacy != "" {
		if scope.clusterID != "" && scope.clusterID != legacy {
			c.AbortWithStatusJSON(400, gin.H{"error": "conflicting cluster filters"})
			return "", errGraphRequestAborted
		}
		if !middleware.ClusterAllowed(c, legacy) {
			middleware.AbortClusterScopeDenied(db, c, legacy)
			return "", errGraphRequestAborted
		}
		scope.clusterID = legacy
	}
	if scope.clusterID != "" {
		return scope.clusterID, nil
	}
	if scope.restricted {
		if len(scope.clusterIDs) == 1 {
			return scope.clusterIDs[0], nil
		}
		c.AbortWithStatusJSON(400, gin.H{"error": "Select one authorized cluster for this graph"})
		return "", errGraphRequestAborted
	}
	return "", nil
}

// GetAttackPathsSummary returns attack path statistics.
func GetAttackPathsSummary(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID, err := graphClusterIDOrDefault(db, c)
		if err != nil {
			if errors.Is(err, errGraphRequestAborted) {
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		builder := graph.NewRelationalPathBuilder(db)
		summary, err := builder.GetSummary(c.Request.Context(), clusterID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		viewerJSON(c, http.StatusOK, gin.H{"data": summary})
	}
}

// GetAttackPathObjectives returns grouped attack objectives for prioritization.
func GetAttackPathObjectives(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID, err := graphClusterIDOrDefault(db, c)
		if err != nil {
			if errors.Is(err, errGraphRequestAborted) {
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		builder := graph.NewRelationalPathBuilder(db)
		objectives, err := builder.GetObjectives(c.Request.Context(), clusterID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		viewerJSON(c, http.StatusOK, gin.H{"data": objectives})
	}
}

// GetAttackChains returns chainable path combinations (MVP rules C1/C2/C5).
func GetAttackChains(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID, err := graphClusterIDOrDefault(db, c)
		if err != nil {
			if errors.Is(err, errGraphRequestAborted) {
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		builder := graph.NewRelationalPathBuilder(db)
		chains, err := builder.GetChains(c.Request.Context(), clusterID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		viewerGraphJSON(c, http.StatusOK, gin.H{"data": chains, "count": len(chains)})
	}
}

// GetAttackPathsBundle returns graph, summary, chains, and objectives in one HTTP round-trip
// (single BuildAllPaths pass on the server).
func GetAttackPathsBundle(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID, err := graphClusterIDOrDefault(db, c)
		if err != nil {
			if errors.Is(err, errGraphRequestAborted) {
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		builder := graph.NewRelationalPathBuilder(db)
		bundle, err := builder.BuildAttackPathsViewBundle(c.Request.Context(), clusterID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		viewerGraphJSON(c, http.StatusOK, gin.H{"data": bundle})
	}
}

// Helper functions for fallback relational queries
func getRelationalGraphData(db *gorm.DB) map[string]interface{} {
	builder := graph.NewRelationalPathBuilder(db)
	data, err := builder.BuildGraphData(context.Background(), "")
	if err != nil {
		return map[string]interface{}{
			"nodes": []interface{}{},
			"links": []interface{}{},
		}
	}
	return data
}

func getRelationalAccessibleResources(db *gorm.DB, saID, resourceType string) []interface{} {
	// Return empty for now - would implement relational query if needed
	return []interface{}{}
}

func getGraphData(graphEngine *graph.AgeGraphEngine) map[string]interface{} {
	// Return graph data structure
	return map[string]interface{}{
		"nodes": []interface{}{},
		"edges": []interface{}{},
	}
}

func attackPathSourcePodUID(path graph.AttackPath) string {
	if len(path.Nodes) == 0 {
		return ""
	}
	return strings.TrimSpace(path.Nodes[0].ID)
}

// GetAttackPaths returns attack paths that originate from a pod.
// Keep this endpoint aligned with /graph/attack-paths/bundle: both use the
// relational builder, cluster scope, and stable p0/p1 path IDs. Resource
// inspector and Attack Paths must not show different path counts for the same
// pod.
func GetAttackPaths(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := strings.TrimSpace(c.Param("uid"))
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "pod UID is required"})
			return
		}

		clusterID, ok := middleware.ResolvedPodClusterID(c)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "resolved pod cluster is required"})
			return
		}

		var pod models.Pod
		if err := db.WithContext(c.Request.Context()).
			Where("cluster_id = ? AND uid = ? AND deleted_at IS NULL", clusterID, podUID).
			First(&pod).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				viewerGraphJSON(c, http.StatusOK, gin.H{
					"pod_uid": podUID,
					"paths":   []graph.AttackPath{},
					"count":   0,
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		builder := graph.NewRelationalPathBuilder(db)
		allPaths, err := builder.BuildAllPaths(c.Request.Context(), clusterID, false)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		paths := make([]graph.AttackPath, 0)
		for i := range allPaths {
			allPaths[i].PathID = "p" + strconv.Itoa(i)
			if strings.EqualFold(attackPathSourcePodUID(allPaths[i]), podUID) {
				paths = append(paths, allPaths[i])
			}
		}

		viewerGraphJSON(c, http.StatusOK, gin.H{
			"cluster_id": clusterID,
			"pod_uid":    podUID,
			"paths":      paths,
			"count":      len(paths),
		})
	}
}
