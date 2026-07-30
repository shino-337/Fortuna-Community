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
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/graph"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/securityaudit"
)

var errGraphRequestAborted = errors.New("graph request aborted")

// GetGraph returns graph data (fallback to relational if AGE not available)
func GetGraph(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try to get graph engine
		graphEngine, err := graph.NewAgeGraphEngine(db)
		if err != nil || !graphEngine.IsEnabled() {
			// Fallback to relational query
			viewerGraphJSON(c, http.StatusOK, gin.H{
				"message": "Graph engine not available, using relational data",
				"data":    getRelationalGraphData(db),
			})
			return
		}

		// Use graph engine
		viewerGraphJSON(c, http.StatusOK, gin.H{
			"message": "Graph engine available",
			"data":    getGraphData(graphEngine),
		})
	}
}

// GetBlastRadius returns blast radius for a resource
func GetBlastRadius(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		resourceID := c.Param("uid")
		maxDepthStr := c.DefaultQuery("max_depth", "3")
		maxDepth, _ := strconv.Atoi(maxDepthStr)
		maxDepth = authorization.MaxGraphTraversalDepth(middleware.GrantedPermissions(c), maxDepth)

		graphEngine, err := graph.NewAgeGraphEngine(db)
		if err != nil || !graphEngine.IsEnabled() {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Graph engine not available",
			})
			return
		}

		resources, err := graphEngine.GetBlastRadius(c.Request.Context(), resourceID, maxDepth)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		truncated := false
		if len(resources) > authorization.GraphTraversalMaxResultNodes {
			resources = resources[:authorization.GraphTraversalMaxResultNodes]
			truncated = true
		}

		viewerJSON(c, http.StatusOK, gin.H{
			"source_id": resourceID,
			"max_depth": maxDepth,
			"resources": resources,
			"count":     len(resources),
			"truncated": truncated,
		})
	}
}

// GetShortestPath finds shortest path between two resources
func GetShortestPath(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		fromID := c.Query("from")
		toID := c.Query("to")

		if fromID == "" || toID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "from and to parameters are required",
			})
			return
		}

		graphEngine, err := graph.NewAgeGraphEngine(db)
		if err != nil || !graphEngine.IsEnabled() {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Graph engine not available",
			})
			return
		}

		path, err := graphEngine.ShortestPath(c.Request.Context(), fromID, toID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		viewerJSON(c, http.StatusOK, gin.H{
			"from": fromID,
			"to":   toID,
			"path": path,
		})
	}
}

// GetAccessibleResources returns resources accessible by a ServiceAccount
func GetAccessibleResources(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		saID := c.Param("uid")
		resourceType := c.DefaultQuery("type", "secrets")

		graphEngine, err := graph.NewAgeGraphEngine(db)
		if err != nil || !graphEngine.IsEnabled() {
			// Fallback to relational query
			viewerJSON(c, http.StatusOK, gin.H{
				"message": "Graph engine not available, using relational data",
				"data":    getRelationalAccessibleResources(db, saID, resourceType),
			})
			return
		}

		if resourceType == "secrets" {
			secrets, err := graphEngine.GetAccessibleSecrets(c.Request.Context(), saID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": err.Error(),
				})
				return
			}

			viewerJSON(c, http.StatusOK, gin.H{
				"service_account_id": saID,
				"type":               resourceType,
				"resources":          secrets,
				"count":              len(secrets),
			})
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "unsupported resource type",
			})
		}
	}
}

// ExecuteGraphQuery executes a custom Cypher query
func ExecuteGraphQuery(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Query  string                 `json:"query" binding:"required"`
			Params map[string]interface{} `json:"params,omitempty"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": err.Error(),
			})
			return
		}

		graphEngine, err := graph.NewAgeGraphEngine(db)
		if err != nil || !graphEngine.IsEnabled() {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Graph engine not available",
			})
			return
		}

		granted := middleware.GrantedPermissions(c)
		advanced := authorization.IsGraphAdvanced(granted)
		if !advanced {
			if sc := authorization.GraphQueryComplexityScore(request.Query); sc > authorization.GraphQueryMaxComplexityScore {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "query complexity exceeds policy for non-advanced graph mode",
					"score": sc,
				})
				return
			}
			if !authorization.ValidateSafeGraphQuery(request.Query) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "query not permitted in safe graph mode"})
				return
			}
		}

		ctx, cancel := authorization.ContextForGraphQuery(c.Request.Context(), advanced)
		defer cancel()

		results, err := graphEngine.ExecuteCypher(ctx, request.Query, request.Params)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		truncated := false
		if len(results) > authorization.GraphTraversalMaxResultNodes {
			results = results[:authorization.GraphTraversalMaxResultNodes]
			truncated = true
		}
		if !advanced && len(results) > authorization.GraphSafeMaxResultRows {
			results = results[:authorization.GraphSafeMaxResultRows]
			truncated = true
		}

		mode := "safe"
		if advanced {
			mode = "advanced"
		}
		ev := securityaudit.FromRequest(
			c,
			authorization.ToStrings(granted),
			c.GetString(middleware.CtxJWTSessionID),
			"graph_cypher_query",
			"graph",
			"query",
			"success",
			"medium",
			"jwt",
			nil,
			map[string]any{"mode": mode, "row_count": len(results), "truncated": truncated, "query_len": len(request.Query)},
			map[string]any{"params_keys": keysOfParams(request.Params)},
			nil,
		)
		securityaudit.Append(db, &ev)
		viewerJSON(c, http.StatusOK, gin.H{
			"results":   results,
			"count":     len(results),
			"truncated": truncated,
			"mode":      mode,
		})
	}
}

func graphClusterIDOrDefault(db *gorm.DB, c *gin.Context) (string, error) {
	if id := strings.TrimSpace(c.Query("cluster_id")); id != "" {
		id = NormalizeClusterID(db, id)
		if !middleware.ClusterAllowed(c, id) {
			middleware.AbortClusterScopeDenied(db, c, id)
			return "", errGraphRequestAborted
		}
		return id, nil
	}
	if ids, restricted := middleware.ScopedClusterIDs(c); restricted {
		if len(ids) == 1 {
			return NormalizeClusterID(db, ids[0]), nil
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error":               "forbidden",
			"reason":              "cluster_scope",
			"required_cluster_id": "cluster_id query parameter",
		})
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

		var pod models.Pod
		if err := db.WithContext(c.Request.Context()).
			Where("uid = ? AND deleted_at IS NULL", podUID).
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
		if !middleware.ClusterAllowed(c, pod.ClusterID) {
			middleware.AbortClusterScopeDenied(db, c, pod.ClusterID)
			return
		}

		builder := graph.NewRelationalPathBuilder(db)
		allPaths, err := builder.BuildAllPaths(c.Request.Context(), pod.ClusterID, false)
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
			"pod_uid": podUID,
			"paths":   paths,
			"count":   len(paths),
		})
	}
}

// GetServiceAccountPermissionsGraph returns all permissions for a service account via graph
func GetServiceAccountPermissionsGraph(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		saUID := c.Param("uid")

		queryService, err := graph.NewQueryService(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to create query service",
			})
			return
		}

		permissions, err := queryService.GetServiceAccountPermissions(c.Request.Context(), saUID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		viewerJSON(c, http.StatusOK, gin.H{
			"service_account_uid": saUID,
			"permissions":         permissions,
			"count":               len(permissions),
		})
	}
}

// GetRiskyPods returns pods with privilege escalation risk
func GetRiskyPods(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		queryService, err := graph.NewQueryService(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to create query service",
			})
			return
		}

		pods, err := queryService.GetPodsWithEscalationRisk(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		viewerJSON(c, http.StatusOK, gin.H{
			"risky_pods": pods,
			"count":      len(pods),
		})
	}
}

func keysOfParams(pm map[string]interface{}) []string {
	if len(pm) == 0 {
		return nil
	}
	out := make([]string, 0, len(pm))
	for k := range pm {
		out = append(out, k)
	}
	return out
}
