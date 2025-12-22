package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/ksam/core/pkg/graph"
)

// GetGraph returns graph data (fallback to relational if AGE not available)
func GetGraph(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try to get graph engine
		graphEngine, err := graph.NewAgeGraphEngine(db)
		if err != nil || !graphEngine.IsEnabled() {
			// Fallback to relational query
			c.JSON(http.StatusOK, gin.H{
				"message": "Graph engine not available, using relational data",
				"data":    getRelationalGraphData(db),
			})
			return
		}

		// Use graph engine
		c.JSON(http.StatusOK, gin.H{
			"message": "Graph engine available",
			"data":    getGraphData(graphEngine),
		})
	}
}

// GetBlastRadius returns blast radius for a resource
func GetBlastRadius(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		resourceID := c.Param("id")
		maxDepthStr := c.DefaultQuery("max_depth", "3")
		maxDepth, _ := strconv.Atoi(maxDepthStr)

		if maxDepth < 1 || maxDepth > 10 {
			maxDepth = 3
		}

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

		c.JSON(http.StatusOK, gin.H{
			"source_id": resourceID,
			"max_depth": maxDepth,
			"resources": resources,
			"count":     len(resources),
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

		c.JSON(http.StatusOK, gin.H{
			"from": fromID,
			"to":   toID,
			"path": path,
		})
	}
}

// GetAccessibleResources returns resources accessible by a ServiceAccount
func GetAccessibleResources(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		saID := c.Param("id")
		resourceType := c.DefaultQuery("type", "secrets")

		graphEngine, err := graph.NewAgeGraphEngine(db)
		if err != nil || !graphEngine.IsEnabled() {
			// Fallback to relational query
			c.JSON(http.StatusOK, gin.H{
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

			c.JSON(http.StatusOK, gin.H{
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

		results, err := graphEngine.ExecuteCypher(c.Request.Context(), request.Query, request.Params)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"results": results,
			"count":   len(results),
		})
	}
}

// Helper functions for fallback relational queries
func getRelationalGraphData(db *gorm.DB) map[string]interface{} {
	// Return basic relational data structure
	return map[string]interface{}{
		"nodes": []interface{}{},
		"edges": []interface{}{},
	}
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

// GetAttackPaths returns attack paths from a pod
func GetAttackPaths(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("uid")
		maxDepthStr := c.DefaultQuery("max_depth", "5")
		maxDepth, _ := strconv.Atoi(maxDepthStr)

		if maxDepth < 1 || maxDepth > 10 {
			maxDepth = 5
		}

		queryService, err := graph.NewQueryService(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to create query service",
			})
			return
		}

		paths, err := queryService.GetAttackPath(c.Request.Context(), podUID, maxDepth)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
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

		c.JSON(http.StatusOK, gin.H{
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

		c.JSON(http.StatusOK, gin.H{
			"risky_pods": pods,
			"count":      len(pods),
		})
	}
}
