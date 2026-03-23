package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/k8s"
	"github.com/fortuna/core/pkg/models"
)

// ActiveClusterCutoff is how long since last sync to consider a cluster "active" for dashboard display.
// Clusters not synced within this window are excluded from /clusters and dashboard stats (stale data).
const ActiveClusterCutoff = 7 * 24 * time.Hour

// getActiveAgentCutoff returns how long since last_seen_at to consider an agent "active" for dashboard.
// Configurable via ACTIVE_AGENT_CUTOFF_MINUTES (default 15). Ensures dashboard shows agents that ping regularly.
func getActiveAgentCutoff() time.Duration {
	if m := os.Getenv("ACTIVE_AGENT_CUTOFF_MINUTES"); m != "" {
		if n, err := strconv.Atoi(m); err == nil && n > 0 {
			return time.Duration(n) * time.Minute
		}
	}
	return 15 * time.Minute
}

// getClustersForAPI returns clusters for API responses (active by default; optional includeStale).
// Only clusters with source IN ('auto','env') are returned so the dashboard shows agent-synced
// clusters only; legacy rows (e.g. id=kubernetes with empty source) are excluded to avoid duplicate display.
// Single source for cluster list query so GetClusters and GetClustersStats stay in sync.
func getClustersForAPI(db *gorm.DB, c *gin.Context) ([]models.Cluster, error) {
	var clusters []models.Cluster
	query := db.Model(&models.Cluster{}).
		Where("source IN ?", []string{"auto", "env"}).
		Where("EXISTS (SELECT 1 FROM pods p WHERE p.cluster_id = clusters.id AND p.deleted_at IS NULL)")
	if c.Query("includeStale") != "true" {
		cutoff := time.Now().Add(-ActiveClusterCutoff)
		query = query.Where("last_sync >= ?", cutoff)
	}
	if err := query.Order("last_sync DESC").Find(&clusters).Error; err != nil {
		return nil, err
	}
	return clusters, nil
}

// GetClusters returns clusters that have synced recently (within ActiveClusterCutoff).
// Stale clusters (no sync in 7 days) are excluded so dashboard only shows current environment.
func GetClusters(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusters, err := getClustersForAPI(db, c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"clusters": clusters})
	}
}

// GetCluster returns a specific cluster
func GetCluster(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var cluster models.Cluster
		if err := db.First(&cluster, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Cluster not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, cluster)
	}
}

// ClusterInfoResponse for GET /cluster/info (cluster domain: list of clusters with basic info).
func ClusterInfoResponseFrom(clusters []models.Cluster) gin.H {
	list := make([]map[string]interface{}, 0, len(clusters))
	for _, c := range clusters {
		list = append(list, map[string]interface{}{
			"id":         c.ID,
			"name":       c.Name,
			"lastSync":   c.LastSync,
			"source":     c.Source,
		})
	}
	return gin.H{"clusters": list}
}

// GetClusterInfo returns cluster list with basic info for GET /cluster/info (infrastructure domain).
func GetClusterInfo(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusters, err := getClustersForAPI(db, c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, ClusterInfoResponseFrom(clusters))
	}
}

// GetClusterNodes returns node names for a cluster for GET /cluster/:id/nodes (infrastructure domain).
func GetClusterNodes(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var cluster models.Cluster
		if err := db.First(&cluster, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Cluster not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		var nodes []string
		db.Model(&models.Pod{}).Where("cluster_id = ? AND deleted_at IS NULL AND node_name IS NOT NULL AND node_name != ''", id).Distinct("node_name").Pluck("node_name", &nodes)
		if nodes == nil {
			nodes = []string{}
		}
		c.JSON(http.StatusOK, gin.H{"nodes": nodes})
	}
}

// ClusterOverviewResponse for GET /clusters/:id/overview
type ClusterOverviewResponse struct {
	PodCount       int64 `json:"podCount"`
	NodeCount      int64 `json:"nodeCount"`
	NamespaceCount int64 `json:"namespaceCount"`
}

// GetClusterOverview returns node count, namespace count, pod count for a cluster (from pods table).
func GetClusterOverview(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var cluster models.Cluster
		if err := db.First(&cluster, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Cluster not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		var podCount int64
		db.Raw("SELECT COUNT(DISTINCT uid) FROM pods WHERE cluster_id = ? AND deleted_at IS NULL", id).Scan(&podCount)
		var nodeCount int64
		db.Raw("SELECT COUNT(DISTINCT node_name) FROM pods WHERE cluster_id = ? AND deleted_at IS NULL AND node_name IS NOT NULL AND node_name != ''", id).Scan(&nodeCount)
		var namespaceCount int64
		db.Raw("SELECT COUNT(DISTINCT namespace) FROM pods WHERE cluster_id = ? AND deleted_at IS NULL", id).Scan(&namespaceCount)
		c.JSON(http.StatusOK, ClusterOverviewResponse{PodCount: podCount, NodeCount: nodeCount, NamespaceCount: namespaceCount})
	}
}

// ClusterInventoryResponse for GET /clusters/:id/inventory
type ClusterInventoryResponse struct {
	Nodes      []string `json:"nodes"`
	Namespaces []string `json:"namespaces"`
}

// GetClusterInventory returns distinct node names and namespaces from pods for the cluster.
func GetClusterInventory(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var cluster models.Cluster
		if err := db.First(&cluster, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Cluster not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		var nodes []string
		db.Model(&models.Pod{}).Where("cluster_id = ? AND deleted_at IS NULL AND node_name IS NOT NULL AND node_name != ''", id).Distinct("node_name").Pluck("node_name", &nodes)
		var namespaces []string
		db.Model(&models.Pod{}).Where("cluster_id = ? AND deleted_at IS NULL", id).Distinct("namespace").Pluck("namespace", &namespaces)
		c.JSON(http.StatusOK, ClusterInventoryResponse{Nodes: nodes, Namespaces: namespaces})
	}
}

// GetClusterAgents returns agents whose node_name appears in pods of the given cluster.
func GetClusterAgents(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var cluster models.Cluster
		if err := db.First(&cluster, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Cluster not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !db.Migrator().HasTable("agents") {
			c.JSON(http.StatusOK, gin.H{"agents": []map[string]interface{}{}, "total": 0})
			return
		}
		var nodeNames []string
		db.Raw("SELECT DISTINCT node_name FROM pods WHERE cluster_id = ? AND deleted_at IS NULL AND node_name IS NOT NULL AND node_name != ''", id).Scan(&nodeNames)
		if len(nodeNames) == 0 {
			c.JSON(http.StatusOK, gin.H{"agents": []map[string]interface{}{}, "total": 0})
			return
		}
		var agents []models.Agent
		db.Where("deleted_at IS NULL AND (status = ? OR status IS NULL) AND node_name IN ?", "ready", nodeNames).Order("last_seen_at DESC NULLS LAST").Find(&agents)
		list := make([]map[string]interface{}, 0, len(agents))
		for _, a := range agents {
			status := "healthy"
			if a.LastSeenAt != nil && time.Since(*a.LastSeenAt) > 5*time.Minute {
				status = "slow"
			} else if a.LastSeenAt != nil && time.Since(*a.LastSeenAt) > 15*time.Minute {
				status = "disconnected"
			}
			lastHB := time.Time{}
			if a.LastSeenAt != nil {
				lastHB = *a.LastSeenAt
			}
			list = append(list, map[string]interface{}{
				"agentId":       a.AgentID,
				"nodeName":      a.NodeName,
				"status":        status,
				"lastHeartbeat": lastHB,
				"version":       a.Version,
			})
		}
		c.JSON(http.StatusOK, gin.H{"agents": list, "total": len(list)})
	}
}

// ClusterSecuritySummaryResponse for GET /clusters/:id/security-summary
type ClusterSecuritySummaryResponse struct {
	RiskBySeverity  map[string]int64 `json:"riskBySeverity"`
	CapabilityCount int64            `json:"capabilityCount"`
	CriticalCount   int64            `json:"criticalCount"`
	HighCount       int64            `json:"highCount"`
	MediumCount     int64            `json:"mediumCount"`
	LowCount        int64            `json:"lowCount"`
}

// GetClusterSecuritySummary returns risk counts by severity and capability exposure for the cluster.
func GetClusterSecuritySummary(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var cluster models.Cluster
		if err := db.First(&cluster, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Cluster not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		var severityRows []struct {
			Severity string
			Count    int64
		}
		db.Raw(`
			SELECT LOWER(i.severity) as severity, COUNT(*) as count
			FROM insights i
			INNER JOIN pods p ON p.uid = i.resource_uid AND p.cluster_id = ? AND p.deleted_at IS NULL
			WHERE i.deleted_at IS NULL AND (i.status = 'active' OR i.status IS NULL)
			GROUP BY LOWER(i.severity)
		`, id).Scan(&severityRows)
		riskBySeverity := make(map[string]int64)
		var critical, high, medium, low int64
		for _, r := range severityRows {
			riskBySeverity[r.Severity] = r.Count
			switch r.Severity {
			case "critical":
				critical = r.Count
			case "high":
				high = r.Count
			case "medium":
				medium = r.Count
			case "low":
				low = r.Count
			}
		}
		var capabilityCount int64
		if db.Migrator().HasTable("pod_capabilities") {
			// pod_capabilities has no deleted_at (see migration 041); filter soft-deleted pods only via p.deleted_at
			db.Raw(`
				SELECT COUNT(DISTINCT pc.id) FROM pod_capabilities pc
				INNER JOIN pods p ON p.uid = pc.pod_uid AND p.cluster_id = ? AND p.deleted_at IS NULL
			`, id).Scan(&capabilityCount)
		}
		c.JSON(http.StatusOK, ClusterSecuritySummaryResponse{
			RiskBySeverity:  riskBySeverity,
			CapabilityCount: capabilityCount,
			CriticalCount:   critical,
			HighCount:       high,
			MediumCount:     medium,
			LowCount:        low,
		})
	}
}

// NodeDetailResponse for GET /clusters/:id/nodes/:nodeName (Node Detail page).
// When node exists in nodes table: full metadata; otherwise derived from pods (nodeName, podCount, pods).
type NodeDetailResponse struct {
	ClusterID      string                   `json:"clusterId"`
	NodeName       string                   `json:"nodeName"`
	IP             string                   `json:"ip,omitempty"`
	KubeletVersion string                   `json:"kubeletVersion,omitempty"`
	Role           string                   `json:"role,omitempty"`
	OS             string                   `json:"os,omitempty"`
	Runtime        string                   `json:"runtime,omitempty"`
	LastSeen       *time.Time               `json:"lastSeen,omitempty"`
	PodCount       int64                    `json:"podCount"`
	Pods           []map[string]interface{} `json:"pods,omitempty"` // List of pods on this node (for Node Detail Workloads tab)
}

// GetClusterNode returns node metadata and pod list for Node Detail page.
// If node exists in nodes table (agent sync), returns metadata; otherwise derives from pods (node_name).
func GetClusterNode(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID := c.Param("id")
		nodeName := c.Param("nodeName")
		if clusterID == "" || nodeName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cluster id and node name required"})
			return
		}
		var cluster models.Cluster
		if err := db.First(&cluster, "id = ?", clusterID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Cluster not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		resp := NodeDetailResponse{ClusterID: clusterID, NodeName: nodeName}
		var node models.Node
		err := db.Where("cluster_id = ? AND node_name = ?", clusterID, nodeName).First(&node).Error
		if err == nil {
			resp.IP = node.IP
			resp.KubeletVersion = node.KubeletVersion
			resp.Role = node.Role
			resp.OS = node.OS
			resp.Runtime = node.Runtime
			resp.LastSeen = node.LastSeen
		}
		var podCount int64
		db.Model(&models.Pod{}).Where("cluster_id = ? AND node_name = ? AND deleted_at IS NULL", clusterID, nodeName).Count(&podCount)
		resp.PodCount = podCount
		includePods := c.Query("pods") == "true" || c.Query("pods") == "1"
		if includePods && podCount > 0 {
			var pods []models.Pod
			db.Where("cluster_id = ? AND node_name = ? AND deleted_at IS NULL", clusterID, nodeName).Find(&pods)
			resp.Pods = make([]map[string]interface{}, 0, len(pods))
			for _, p := range pods {
				var riskCount int64
				db.Model(&models.Insight{}).Where("resource_type = ? AND resource_uid = ? AND deleted_at IS NULL AND (status = 'active' OR status IS NULL)", "Pod", p.UID).Count(&riskCount)
				resp.Pods = append(resp.Pods, map[string]interface{}{
					"id":        p.ID,
					"uid":       p.UID,
					"name":      p.Name,
					"namespace": p.Namespace,
					"riskCount": riskCount,
				})
			}
		}
		c.JSON(http.StatusOK, resp)
	}
}

// ClusterStats represents cluster statistics and status
type ClusterStats struct {
	models.Cluster
	ServiceAccountCount     int64  `json:"serviceAccountCount"`
	RoleCount               int64  `json:"roleCount"`
	ClusterRoleCount        int64  `json:"clusterRoleCount"`
	RoleBindingCount        int64  `json:"roleBindingCount"`
	ClusterRoleBindingCount int64  `json:"clusterRoleBindingCount"`
	PodCount                int64  `json:"podCount"`
	DeploymentCount         int64  `json:"deploymentCount"`
	RiskCount               int64  `json:"riskCount"`        // Active insights for resources in this cluster
	AgentCount              int64  `json:"agentCount"`       // Agents on nodes belonging to this cluster
	ConnectionStatus        string `json:"connectionStatus"` // connected, disconnected, unknown
	AgentVersion            string `json:"agentVersion,omitempty"`
}

// GetClustersStats returns clusters (with recent sync by default) and their statistics.
func GetClustersStats(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusters, err := getClustersForAPI(db, c)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		stats := make([]ClusterStats, 0, len(clusters))

		for _, cluster := range clusters {
			stat := ClusterStats{
				Cluster: cluster,
			}

			// Count resources for this cluster (GORM automatically filters deleted_at IS NULL)
			db.Model(&models.ServiceAccount{}).Where("cluster_id = ?", cluster.ID).Count(&stat.ServiceAccountCount)
			db.Model(&models.Role{}).Where("cluster_id = ?", cluster.ID).Count(&stat.RoleCount)
			db.Model(&models.ClusterRole{}).Where("cluster_id = ?", cluster.ID).Count(&stat.ClusterRoleCount)
			db.Model(&models.RoleBinding{}).Where("cluster_id = ?", cluster.ID).Count(&stat.RoleBindingCount)
			db.Model(&models.ClusterRoleBinding{}).Where("cluster_id = ?", cluster.ID).Count(&stat.ClusterRoleBindingCount)
			// Count distinct UIDs to avoid duplicates (GORM automatically filters deleted_at IS NULL)
			var podCount int64
			db.Raw("SELECT COUNT(DISTINCT uid) FROM pods WHERE cluster_id = ? AND deleted_at IS NULL", cluster.ID).Scan(&podCount)
			stat.PodCount = podCount
			db.Model(&models.Deployment{}).Where("cluster_id = ?", cluster.ID).Count(&stat.DeploymentCount)

			// Risk count: active insights whose resource_uid is a pod in this cluster
			db.Raw(`
				SELECT COUNT(*) FROM insights i
				INNER JOIN pods p ON p.uid = i.resource_uid AND p.cluster_id = ? AND p.deleted_at IS NULL
				WHERE i.deleted_at IS NULL AND (i.status = 'active' OR i.status IS NULL)
			`, cluster.ID).Scan(&stat.RiskCount)
			// Agent count: agents whose node_name appears in pods of this cluster
			if db.Migrator().HasTable("agents") {
				if errAgent := db.Raw(`
					SELECT COUNT(*) FROM agents a
					WHERE a.deleted_at IS NULL AND (a.status = 'ready' OR a.status IS NULL)
					AND a.node_name IN (SELECT DISTINCT node_name FROM pods WHERE cluster_id = ? AND deleted_at IS NULL AND node_name IS NOT NULL AND node_name != '')
				`, cluster.ID).Scan(&stat.AgentCount).Error; errAgent != nil {
					log.Printf("[GetClustersStats] agent count query failed for cluster %q: %v", cluster.ID, errAgent)
					stat.AgentCount = 0
				}
			}

			// Determine connection status: LastSync (cluster sync from agent) and optionally latest agent heartbeat.
			// Use wider windows so clusters that sync every 5–15 min stay "connected".
			if cluster.Status == "error" {
				stat.ConnectionStatus = "disconnected"
			} else {
				timeSinceSync := time.Since(cluster.LastSync)
				var statusFromSync string
				if cluster.LastSync.IsZero() {
					statusFromSync = "disconnected"
				} else if timeSinceSync < 15*time.Minute {
					statusFromSync = "connected"
				} else if timeSinceSync < 2*time.Hour {
					statusFromSync = "degraded"
				} else {
					statusFromSync = "disconnected"
				}
				stat.ConnectionStatus = statusFromSync
				// If cluster sync is stale but an agent for this cluster was seen recently, upgrade to connected
				if statusFromSync != "connected" && db.Migrator().HasTable("agents") {
					var latestAgentSeen *time.Time
					errAgent := db.Raw(`
						SELECT MAX(a.last_seen_at) FROM agents a
						WHERE a.deleted_at IS NULL
						AND a.node_name IN (SELECT DISTINCT node_name FROM pods WHERE cluster_id = ? AND deleted_at IS NULL AND node_name IS NOT NULL AND node_name != '')
					`, cluster.ID).Scan(&latestAgentSeen).Error
					if errAgent == nil && latestAgentSeen != nil {
						if time.Since(*latestAgentSeen) < 15*time.Minute {
							stat.ConnectionStatus = "connected"
						} else if time.Since(*latestAgentSeen) < 2*time.Hour && stat.ConnectionStatus == "disconnected" {
							stat.ConnectionStatus = "degraded"
						}
					}
				}
			}

			// Agent version could be stored in cluster metadata in future
			// For now, leaving it empty
			stat.AgentVersion = "v1.0.0"

			stats = append(stats, stat)
		}

		c.JSON(http.StatusOK, gin.H{
			"clusters": stats,
			"total":    len(stats),
		})
	}
}

// GetServiceAccounts returns all service accounts with optional filters
func GetServiceAccounts(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var serviceAccounts []models.ServiceAccount
		query := db.Model(&models.ServiceAccount{})
		// GORM automatically filters soft-deleted records (deleted_at IS NULL)

		// Filter by cluster
		if clusterID := c.Query("cluster"); clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		// Filter by namespace
		if namespace := c.Query("namespace"); namespace != "" {
			query = query.Where("namespace = ?", namespace)
		}

		// Pagination
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSizeParam, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))

		// Special handling: pageSize=-1 means return all records
		// For security, set a maximum limit (1000) when fetching all
		// Note: pageSize=0 is not supported as GORM treats Limit(0) specially
		fetchAll := pageSizeParam == -1

		var pageSize int
		var responsePageSize int
		if fetchAll {
			// When fetching all, use a large limit (1000) but don't apply offset
			pageSize = 1000      // Max limit for "all" requests
			responsePageSize = 0 // Will be set to actual count later
		} else {
			pageSize = pageSizeParam
			// Validate pageSize (max 1000 for normal pagination)
			// pageSize=0 is treated as invalid and defaults to 50
			if pageSize < 1 {
				pageSize = 50 // Default if invalid (including 0)
			}
			if pageSize > 1000 {
				pageSize = 1000 // Max limit
			}
			responsePageSize = pageSize
		}

		offset := (page - 1) * pageSize

		var total int64
		query.Count(&total)

		// Build query with pagination
		resultQuery := query
		if fetchAll {
			// For "all", apply limit but no offset (start from beginning)
			resultQuery = resultQuery.Limit(pageSize)
		} else {
			// Normal pagination with offset and limit
			resultQuery = resultQuery.Offset(offset).Limit(pageSize)
		}

		if err := resultQuery.Find(&serviceAccounts).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// If fetching all, update responsePageSize to actual returned count
		if fetchAll {
			responsePageSize = len(serviceAccounts)
		}

		c.JSON(http.StatusOK, gin.H{
			"serviceAccounts": serviceAccounts,
			"total":           total,
			"page":            page,
			"pageSize":        responsePageSize,
		})
	}
}

// GetServiceAccount returns a specific service account
func GetServiceAccount(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var sa models.ServiceAccount
		if err := db.Preload("Cluster").First(&sa, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "ServiceAccount not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, sa)
	}
}

// GetServiceAccountByUID returns a service account by UID (inventory domain).
func GetServiceAccountByUID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		saUID := c.Param("uid")
		if saUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "uid is required"})
			return
		}
		var sa models.ServiceAccount
		if err := db.Preload("Cluster").Where("uid = ?", saUID).First(&sa).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "ServiceAccount not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, sa)
	}
}

// GetDeployments returns all deployments with filtering and pagination
func GetDeployments(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var deployments []models.Deployment
		query := db.Model(&models.Deployment{})
		// GORM automatically filters soft-deleted records (deleted_at IS NULL)

		// Filter by cluster
		if clusterID := c.Query("cluster"); clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		// Filter by namespace
		if namespace := c.Query("namespace"); namespace != "" {
			query = query.Where("namespace = ?", namespace)
		}

		// Pagination
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
		offset := (page - 1) * pageSize

		var total int64
		query.Count(&total)

		if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&deployments).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"deployments": deployments,
			"total":       total,
			"page":        page,
			"pageSize":    pageSize,
		})
	}
}

// GetDeployment returns a specific deployment
func GetDeployment(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var deployment models.Deployment
		if err := db.Preload("Cluster").First(&deployment, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Deployment not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Parse JSON fields for frontend
		var containers []interface{}
		var labels map[string]string
		var annotations map[string]string
		var selector map[string]string
		var conditions []interface{}
		var template map[string]interface{}

		json.Unmarshal([]byte(deployment.Template), &template)
		json.Unmarshal([]byte(deployment.Labels), &labels)
		json.Unmarshal([]byte(deployment.Annotations), &annotations)
		json.Unmarshal([]byte(deployment.Selector), &selector)
		json.Unmarshal([]byte(deployment.Conditions), &conditions)

		// Extract containers from template
		if template != nil {
			if containersList, ok := template["containers"]; ok {
				containers, _ = containersList.([]interface{})
			}
		}

		response := gin.H{
			"id":                  deployment.ID,
			"clusterId":           deployment.ClusterID,
			"uid":                 deployment.UID,
			"name":                deployment.Name,
			"namespace":           deployment.Namespace,
			"replicasDesired":     deployment.Replicas,
			"replicasReady":       deployment.ReadyReplicas,
			"replicasAvailable":   deployment.AvailableReplicas,
			"replicasUnavailable": deployment.UnavailableReplicas,
			"replicasUpdated":     deployment.UpdatedReplicas,
			"strategy":            deployment.Strategy,
			"containers":          containers,
			"labels":              labels,
			"annotations":         annotations,
			"selector":            selector,
			"conditions":          conditions,
			"createdAt":           deployment.CreatedAt,
			"updatedAt":           deployment.UpdatedAt,
		}

		c.JSON(http.StatusOK, response)
	}
}

// GetReplicaSets returns all replicasets with filters and pagination
func GetReplicaSets(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var replicasets []models.ReplicaSet
		query := db.Model(&models.ReplicaSet{})
		// GORM automatically filters soft-deleted records (deleted_at IS NULL)

		// Filter by cluster
		if clusterID := c.Query("cluster"); clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		// Filter by namespace
		if namespace := c.Query("namespace"); namespace != "" {
			query = query.Where("namespace = ?", namespace)
		}

		// Filter by owner
		if ownerKind := c.Query("ownerKind"); ownerKind != "" {
			query = query.Where("owner_kind = ?", ownerKind)
		}
		if ownerName := c.Query("ownerName"); ownerName != "" {
			query = query.Where("owner_name = ?", ownerName)
		}

		// Pagination
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
		offset := (page - 1) * pageSize

		var total int64
		query.Count(&total)

		if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&replicasets).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"replicasets": replicasets,
			"total":       total,
			"page":        page,
			"pageSize":    pageSize,
		})
	}
}

// GetReplicaSet returns a specific replicaset
func GetReplicaSet(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var replicaset models.ReplicaSet
		if err := db.Preload("Cluster").First(&replicaset, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "ReplicaSet not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Parse JSON fields for frontend
		var containers []interface{}
		var labels map[string]string
		var annotations map[string]string
		var selector map[string]string
		var conditions []interface{}
		var template map[string]interface{}

		json.Unmarshal([]byte(replicaset.Template), &template)
		json.Unmarshal([]byte(replicaset.Labels), &labels)
		json.Unmarshal([]byte(replicaset.Annotations), &annotations)
		json.Unmarshal([]byte(replicaset.Selector), &selector)
		json.Unmarshal([]byte(replicaset.Conditions), &conditions)

		// Extract containers from template
		if template != nil {
			if containersList, ok := template["containers"]; ok {
				containers, _ = containersList.([]interface{})
			}
		}

		response := gin.H{
			"id":                   replicaset.ID,
			"clusterId":            replicaset.ClusterID,
			"uid":                  replicaset.UID,
			"name":                 replicaset.Name,
			"namespace":            replicaset.Namespace,
			"replicas":             replicaset.Replicas,
			"readyReplicas":        replicaset.ReadyReplicas,
			"availableReplicas":    replicaset.AvailableReplicas,
			"fullyLabeledReplicas": replicaset.FullyLabeledReplicas,
			"ownerKind":            replicaset.OwnerKind,
			"ownerName":            replicaset.OwnerName,
			"ownerUid":             replicaset.OwnerUID,
			"containers":           containers,
			"labels":               labels,
			"annotations":          annotations,
			"selector":             selector,
			"conditions":           conditions,
			"createdAt":            replicaset.CreatedAt,
			"updatedAt":            replicaset.UpdatedAt,
		}

		c.JSON(http.StatusOK, response)
	}
}

// UpdateServiceAccount updates a service account
func UpdateServiceAccount(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var sa models.ServiceAccount
		if err := db.First(&sa, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "ServiceAccount not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var updateData map[string]interface{}
		if err := c.ShouldBindJSON(&updateData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := db.Model(&sa).Updates(updateData).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Log audit
		userID, _ := c.Get("userID")
		username, _ := c.Get("username")
		auditLog := models.AuditLog{
			ClusterID:  sa.ClusterID,
			UserID:     userID.(uint),
			Action:     "update",
			Resource:   "serviceaccount",
			ResourceID: strconv.Itoa(int(sa.ID)),
			User:       username.(string),
			IP:         c.ClientIP(),
		}
		db.Create(&auditLog)

		c.JSON(http.StatusOK, sa)
	}
}

// DeleteServiceAccount deletes a service account
func DeleteServiceAccount(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var sa models.ServiceAccount
		if err := db.Preload("Cluster").First(&sa, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "ServiceAccount not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Try to delete from Kubernetes cluster first
		var k8sErr error
		if sa.Cluster.Kubeconfig != "" {
			// Use kubeconfig from cluster
			k8sClient, err := k8s.NewClientFromKubeconfig(sa.Cluster.Kubeconfig)
			if err == nil {
				k8sErr = k8sClient.DeleteServiceAccount(sa.Namespace, sa.Name)
			}
		} else {
			// Try to use default kubeconfig or in-cluster config
			k8sClient, err := k8s.NewClientFromPath("")
			if err == nil {
				k8sErr = k8sClient.DeleteServiceAccount(sa.Namespace, sa.Name)
			}
		}

		// Log audit before deletion
		userID, _ := c.Get("userID")
		username, _ := c.Get("username")
		auditLog := models.AuditLog{
			ClusterID:  sa.ClusterID,
			UserID:     userID.(uint),
			Action:     "delete",
			Resource:   "serviceaccount",
			ResourceID: strconv.Itoa(int(sa.ID)),
			User:       username.(string),
			IP:         c.ClientIP(),
		}

		// Add K8s deletion result to audit details
		if k8sErr != nil {
			auditLog.Details = fmt.Sprintf(`{"k8s_deletion":"failed","error":"%s"}`, k8sErr.Error())
		} else {
			auditLog.Details = `{"k8s_deletion":"success"}`
		}
		db.Create(&auditLog)

		// Delete from database
		if err := db.Delete(&sa).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Return success even if K8s deletion failed (for backward compatibility)
		message := "ServiceAccount deleted from database"
		if k8sErr == nil {
			message = "ServiceAccount deleted from Kubernetes cluster and database"
		} else {
			message = fmt.Sprintf("ServiceAccount deleted from database, but failed to delete from Kubernetes: %v", k8sErr)
		}

		c.JSON(http.StatusOK, gin.H{"message": message})
	}
}

// UpdateServiceAccountByUID updates a service account by Kubernetes UID (inventory domain).
func UpdateServiceAccountByUID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uid")
		if uid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "uid is required"})
			return
		}
		var sa models.ServiceAccount
		if err := db.Where("uid = ?", uid).First(&sa).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "ServiceAccount not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var updateData map[string]interface{}
		if err := c.ShouldBindJSON(&updateData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := db.Model(&sa).Updates(updateData).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		userID, _ := c.Get("userID")
		username, _ := c.Get("username")
		auditLog := models.AuditLog{
			ClusterID:  sa.ClusterID,
			UserID:     userID.(uint),
			Action:     "update",
			Resource:   "serviceaccount",
			ResourceID: strconv.Itoa(int(sa.ID)),
			User:       username.(string),
			IP:         c.ClientIP(),
		}
		db.Create(&auditLog)

		c.JSON(http.StatusOK, sa)
	}
}

// DeleteServiceAccountByUID deletes a service account by Kubernetes UID (inventory domain).
func DeleteServiceAccountByUID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uid")
		if uid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "uid is required"})
			return
		}
		var sa models.ServiceAccount
		if err := db.Preload("Cluster").Where("uid = ?", uid).First(&sa).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "ServiceAccount not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		var k8sErr error
		if sa.Cluster.Kubeconfig != "" {
			k8sClient, err := k8s.NewClientFromKubeconfig(sa.Cluster.Kubeconfig)
			if err == nil {
				k8sErr = k8sClient.DeleteServiceAccount(sa.Namespace, sa.Name)
			}
		} else {
			k8sClient, err := k8s.NewClientFromPath("")
			if err == nil {
				k8sErr = k8sClient.DeleteServiceAccount(sa.Namespace, sa.Name)
			}
		}

		userID, _ := c.Get("userID")
		username, _ := c.Get("username")
		auditLog := models.AuditLog{
			ClusterID:  sa.ClusterID,
			UserID:     userID.(uint),
			Action:     "delete",
			Resource:   "serviceaccount",
			ResourceID: strconv.Itoa(int(sa.ID)),
			User:       username.(string),
			IP:         c.ClientIP(),
		}
		if k8sErr != nil {
			auditLog.Details = fmt.Sprintf(`{"k8s_deletion":"failed","error":"%s"}`, k8sErr.Error())
		} else {
			auditLog.Details = `{"k8s_deletion":"success"}`
		}
		db.Create(&auditLog)

		if err := db.Delete(&sa).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		message := "ServiceAccount deleted from database"
		if k8sErr == nil {
			message = "ServiceAccount deleted from Kubernetes cluster and database"
		} else {
			message = fmt.Sprintf("ServiceAccount deleted from database, but failed to delete from Kubernetes: %v", k8sErr)
		}
		c.JSON(http.StatusOK, gin.H{"message": message})
	}
}

// GetGraph is now in graph_handlers.go

// GetAuditLogs returns audit logs
func GetAuditLogs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var logs []models.AuditLog
		query := db.Model(&models.AuditLog{})

		// Filter by cluster
		if clusterID := c.Query("cluster"); clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		// Filter by resource
		if resource := c.Query("resource"); resource != "" {
			query = query.Where("resource = ?", resource)
		}

		// Filter by action
		if action := c.Query("action"); action != "" {
			query = query.Where("action = ?", action)
		}

		// Filter by resource_id (e.g. insight ID for Risk Center audit trail)
		if resourceID := c.Query("resource_id"); resourceID != "" {
			query = query.Where("resource_id = ?", resourceID)
		}

		// Pagination
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
		offset := (page - 1) * pageSize

		var total int64
		query.Count(&total)

		if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"logs":     logs,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		})
	}
}

// GetPods returns all pods with optional filters
func GetPods(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var pods []models.Pod
		query := db.Model(&models.Pod{})
		// GORM automatically filters soft-deleted records (deleted_at IS NULL)

		// Filter by cluster
		if clusterID := c.Query("cluster"); clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		// Filter by namespace
		if namespace := c.Query("namespace"); namespace != "" {
			query = query.Where("namespace = ?", namespace)
		}

		// Filter by service account
		if serviceAccount := c.Query("serviceAccount"); serviceAccount != "" {
			query = query.Where("service_account = ?", serviceAccount)
		}

		// Filter by node name (for Node "detail" view: pods on a specific node)
		if nodeName := c.Query("node"); nodeName != "" {
			query = query.Where("node_name = ?", nodeName)
		}

		// Pagination
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
		offset := (page - 1) * pageSize

		var total int64
		query.Count(&total)

		// GORM automatically filters soft-deleted records
		if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&pods).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Risk count per pod (active insights where resource_uid = pod.uid)
		riskByUID := make(map[string]int64)
		if len(pods) > 0 && db.Migrator().HasTable("insights") {
			uids := make([]string, 0, len(pods))
			for _, p := range pods {
				uids = append(uids, p.UID)
			}
			var rows []struct {
				ResourceUID string `gorm:"column:resource_uid"`
				Count       int64  `gorm:"column:count"`
			}
			db.Raw(`
				SELECT resource_uid, COUNT(*) as count FROM insights
				WHERE deleted_at IS NULL AND (status = 'active' OR status IS NULL) AND resource_uid IN ?
				GROUP BY resource_uid
			`, uids).Scan(&rows)
			for _, r := range rows {
				riskByUID[r.ResourceUID] = r.Count
			}
		}
		type podWithRisk struct {
			models.Pod
			RiskCount int64 `json:"riskCount"`
		}
		out := make([]podWithRisk, 0, len(pods))
		for _, p := range pods {
			out = append(out, podWithRisk{Pod: p, RiskCount: riskByUID[p.UID]})
		}

		c.JSON(http.StatusOK, gin.H{
			"pods":     out,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		})
	}
}

// GetPod returns a specific pod by ID, with riskCount (active insights for this pod UID).
func GetPod(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var pod models.Pod
		// GORM automatically filters soft-deleted records
		if err := db.Preload("Cluster").First(&pod, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		var riskCount int64
		if db.Migrator().HasTable("insights") {
			db.Raw(`
				SELECT COUNT(*) FROM insights
				WHERE deleted_at IS NULL AND (status = 'active' OR status IS NULL) AND resource_uid = ?
			`, pod.UID).Scan(&riskCount)
		}
		type podWithRisk struct {
			models.Pod
			RiskCount int64 `json:"riskCount"`
		}
		c.JSON(http.StatusOK, podWithRisk{Pod: pod, RiskCount: riskCount})
	}
}

// GetPodByUID returns a pod by UID (for Risk Detail → Pod Detail link). Same response shape as GetPod.
func GetPodByUID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := c.Param("uid")
		if uid == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "uid is required"})
			return
		}
		var pod models.Pod
		if err := db.Preload("Cluster").Where("uid = ?", uid).First(&pod).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		var riskCount int64
		if db.Migrator().HasTable("insights") {
			db.Raw(`
				SELECT COUNT(*) FROM insights
				WHERE deleted_at IS NULL AND (status = 'active' OR status IS NULL) AND resource_uid = ?
			`, pod.UID).Scan(&riskCount)
		}
		type podWithRisk struct {
			models.Pod
			RiskCount int64 `json:"riskCount"`
		}
		c.JSON(http.StatusOK, podWithRisk{Pod: pod, RiskCount: riskCount})
	}
}

// GetAuditReports returns audit reports
func GetAuditReports(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		type Report struct {
			Resource string `json:"resource"`
			Action   string `json:"action"`
			Count    int64  `json:"count"`
		}

		reports := make([]Report, 0)
		if err := db.Model(&models.AuditLog{}).
			Select("resource, action, COUNT(*) as count").
			Group("resource, action").
			Scan(&reports).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"reports": reports})
	}
}
