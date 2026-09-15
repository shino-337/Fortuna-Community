package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/k8s"
	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/graph"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/risk"
)

// ActiveClusterCutoff is how long since last sync to consider a cluster "active" for dashboard display.
// Clusters not synced within this window are excluded from /clusters and dashboard stats (stale data).
const ActiveClusterCutoff = 7 * 24 * time.Hour

// hasTable checks schema cache first, falls back to live GORM introspection.
func hasTable(db *gorm.DB, name string) bool {
	if sc := GetSchemaCache(); sc != nil {
		return sc.Has(name)
	}
	return db.Migrator().HasTable(name)
}

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
	if ids, restricted := middleware.ScopedClusterIDs(c); restricted {
		query = query.Where("id IN ?", ids)
	}
	if c.Query("includeStale") != "true" {
		cutoff := time.Now().Add(-ActiveClusterCutoff)
		query = query.Where("last_sync >= ?", cutoff)
	}
	if err := query.Order("last_sync DESC").Find(&clusters).Error; err != nil {
		return nil, err
	}
	return clusters, nil
}

// GetDefaultActiveClusterID returns a cluster_id for graph endpoints when the client omits ?cluster_id=
// (newest active agent-synced cluster with pods; if none, any cluster that already has pod inventory).
func GetDefaultActiveClusterID(db *gorm.DB, ctx context.Context) (string, error) {
	var cluster models.Cluster
	cutoff := time.Now().Add(-ActiveClusterCutoff)
	err := db.WithContext(ctx).Model(&models.Cluster{}).
		Where("source IN ?", []string{"auto", "env"}).
		Where("EXISTS (SELECT 1 FROM pods p WHERE p.cluster_id = clusters.id AND p.deleted_at IS NULL)").
		Where("last_sync >= ?", cutoff).
		Order("last_sync DESC").
		First(&cluster).Error
	if err == nil {
		return cluster.ID, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return "", err
	}
	var fallback models.Cluster
	err2 := db.WithContext(ctx).Model(&models.Cluster{}).
		Where("EXISTS (SELECT 1 FROM pods p WHERE p.cluster_id = clusters.id AND p.deleted_at IS NULL)").
		Order("last_sync DESC NULLS LAST").
		First(&fallback).Error
	if err2 != nil {
		if err2 == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("no cluster with pod inventory; pass cluster_id or wait for agent sync")
		}
		return "", err2
	}
	return fallback.ID, nil
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
			"id":       c.ID,
			"name":     c.Name,
			"lastSync": c.LastSync,
			"source":   c.Source,
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
		if !hasTable(db, "agents") {
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
		if hasTable(db, "pod_capabilities") {
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
			uids := make([]string, 0, len(pods))
			for _, p := range pods {
				uids = append(uids, p.UID)
			}
			riskByUID := make(map[string]int64, len(pods))
			if len(uids) > 0 && hasTable(db, "insights") {
				var rows []struct {
					ResourceUID string `gorm:"column:resource_uid"`
					Count       int64  `gorm:"column:count"`
				}
				db.Model(&models.Insight{}).
					Select("resource_uid, COUNT(*) as count").
					Where("deleted_at IS NULL AND (status = 'active' OR status IS NULL) AND resource_uid IN ?", uids).
					Group("resource_uid").Scan(&rows)
				for _, r := range rows {
					riskByUID[r.ResourceUID] = r.Count
				}
			}
			resp.Pods = make([]map[string]interface{}, 0, len(pods))
			for _, p := range pods {
				resp.Pods = append(resp.Pods, map[string]interface{}{
					"id":        p.ID,
					"uid":       p.UID,
					"name":      p.Name,
					"namespace": p.Namespace,
					"riskCount": riskByUID[p.UID],
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

		clusterIDs := make([]string, 0, len(clusters))
		for _, cl := range clusters {
			clusterIDs = append(clusterIDs, cl.ID)
		}

		type clusterCount struct {
			ClusterID string `gorm:"column:cluster_id"`
			Cnt       int64  `gorm:"column:cnt"`
		}
		toMap := func(rows []clusterCount) map[string]int64 {
			m := make(map[string]int64, len(rows))
			for _, r := range rows {
				m[r.ClusterID] = r.Cnt
			}
			return m
		}

		var saCounts, roleCounts, crCounts, rbCounts, crbCounts, podCounts, deplCounts, riskCounts []clusterCount

		db.Model(&models.ServiceAccount{}).Select("cluster_id, COUNT(*) as cnt").Where("cluster_id IN ?", clusterIDs).Group("cluster_id").Scan(&saCounts)
		db.Model(&models.Role{}).Select("cluster_id, COUNT(*) as cnt").Where("cluster_id IN ?", clusterIDs).Group("cluster_id").Scan(&roleCounts)
		db.Model(&models.ClusterRole{}).Select("cluster_id, COUNT(*) as cnt").Where("cluster_id IN ?", clusterIDs).Group("cluster_id").Scan(&crCounts)
		db.Model(&models.RoleBinding{}).Select("cluster_id, COUNT(*) as cnt").Where("cluster_id IN ?", clusterIDs).Group("cluster_id").Scan(&rbCounts)
		db.Model(&models.ClusterRoleBinding{}).Select("cluster_id, COUNT(*) as cnt").Where("cluster_id IN ?", clusterIDs).Group("cluster_id").Scan(&crbCounts)
		db.Model(&models.Pod{}).Select("cluster_id, COUNT(DISTINCT uid) AS cnt").Where("cluster_id IN ? AND deleted_at IS NULL", clusterIDs).Group("cluster_id").Scan(&podCounts)
		db.Model(&models.Deployment{}).Select("cluster_id, COUNT(*) as cnt").Where("cluster_id IN ?", clusterIDs).Group("cluster_id").Scan(&deplCounts)
		db.Table("insights i").
			Select("p.cluster_id, COUNT(*) AS cnt").
			Joins("INNER JOIN pods p ON p.uid = i.resource_uid AND p.deleted_at IS NULL").
			Where("i.deleted_at IS NULL AND (i.status = 'active' OR i.status IS NULL) AND p.cluster_id IN ?", clusterIDs).
			Group("p.cluster_id").Scan(&riskCounts)

		saMap, roleMap, crMap, rbMap, crbMap := toMap(saCounts), toMap(roleCounts), toMap(crCounts), toMap(rbCounts), toMap(crbCounts)
		podMap, deplMap, riskMap := toMap(podCounts), toMap(deplCounts), toMap(riskCounts)

		type agentRow struct {
			ClusterID string     `gorm:"column:cluster_id"`
			Cnt       int64      `gorm:"column:cnt"`
			MaxSeen   *time.Time `gorm:"column:max_seen"`
		}
		agentByCluster := make(map[string]agentRow)
		if hasTable(db, "agents") {
			var agentRows []agentRow
			db.Table("agents a").
				Select("p.cluster_id, COUNT(DISTINCT a.id) AS cnt, MAX(a.last_seen_at) AS max_seen").
				Joins("INNER JOIN pods p ON p.node_name = a.node_name AND p.deleted_at IS NULL AND p.node_name IS NOT NULL AND p.node_name != ''").
				Where("a.deleted_at IS NULL AND (a.status = 'ready' OR a.status IS NULL) AND p.cluster_id IN ?", clusterIDs).
				Group("p.cluster_id").Scan(&agentRows)
			for _, r := range agentRows {
				agentByCluster[r.ClusterID] = r
			}
		}

		stats := make([]ClusterStats, 0, len(clusters))
		for _, cluster := range clusters {
			stat := ClusterStats{
				Cluster:                 cluster,
				ServiceAccountCount:     saMap[cluster.ID],
				RoleCount:               roleMap[cluster.ID],
				ClusterRoleCount:        crMap[cluster.ID],
				RoleBindingCount:        rbMap[cluster.ID],
				ClusterRoleBindingCount: crbMap[cluster.ID],
				PodCount:                podMap[cluster.ID],
				DeploymentCount:         deplMap[cluster.ID],
				RiskCount:               riskMap[cluster.ID],
				AgentVersion:            "v1.0.0",
			}

			if ar, ok := agentByCluster[cluster.ID]; ok {
				stat.AgentCount = ar.Cnt
			}

			if cluster.Status == "error" {
				stat.ConnectionStatus = "disconnected"
			} else {
				timeSinceSync := time.Since(cluster.LastSync)
				if cluster.LastSync.IsZero() {
					stat.ConnectionStatus = "disconnected"
				} else if timeSinceSync < 15*time.Minute {
					stat.ConnectionStatus = "connected"
				} else if timeSinceSync < 2*time.Hour {
					stat.ConnectionStatus = "degraded"
				} else {
					stat.ConnectionStatus = "disconnected"
				}
				if stat.ConnectionStatus != "connected" {
					if ar, ok := agentByCluster[cluster.ID]; ok && ar.MaxSeen != nil {
						age := time.Since(*ar.MaxSeen)
						if age < 15*time.Minute {
							stat.ConnectionStatus = "connected"
						} else if age < 2*time.Hour && stat.ConnectionStatus == "disconnected" {
							stat.ConnectionStatus = "degraded"
						}
					}
				}
			}
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
		scope, ok := resolveRiskGovernanceScope(db, c)
		if !ok {
			return
		}
		serviceAccounts := make([]models.ServiceAccount, 0)
		query := db.WithContext(c.Request.Context()).Model(&models.ServiceAccount{})
		if scope.restricted {
			query = query.Where("cluster_id IN ?", scope.clusterIDs)
		}
		// GORM automatically filters soft-deleted records (deleted_at IS NULL)

		// Filter by cluster
		if clusterID := scope.clusterID; clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}

		// Filter by namespace
		if namespace := c.Query("namespace"); namespace != "" {
			query = query.Where("namespace = ?", namespace)
		}

		// Pagination
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		if page < 1 {
			page = 1
		}
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

		offset := 0

		var total int64
		if err := query.Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count service accounts"})
			return
		}
		if !fetchAll {
			if total == 0 || int64(page-1) > (total-1)/int64(pageSize) {
				c.JSON(http.StatusOK, gin.H{"serviceAccounts": serviceAccounts, "total": total, "page": page, "pageSize": responsePageSize})
				return
			}
			offset = (page - 1) * pageSize
		}

		// Build query with pagination
		resultQuery := query.Order("id ASC")
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
		if !authorizeServiceAccount(db, c, &sa) {
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
		if !authorizeServiceAccount(db, c, &sa) {
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

		if clusterID := c.Query("cluster"); clusterID != "" {
			query = query.Where("cluster_id = ?", clusterID)
		}
		if namespace := c.Query("namespace"); namespace != "" {
			query = query.Where("namespace = ?", namespace)
		}

		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
		if page < 1 {
			page = 1
		}
		if pageSize < 1 {
			pageSize = 50
		}
		if pageSize > 1000 {
			pageSize = 1000
		}
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

		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
		if page < 1 {
			page = 1
		}
		if pageSize < 1 {
			pageSize = 50
		}
		if pageSize > 1000 {
			pageSize = 1000
		}
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
		if !authorizeServiceAccount(db, c, &sa) {
			return
		}

		updateData, ok := serviceAccountMetadataUpdate(c)
		if !ok {
			return
		}

		if err := db.Model(&sa).Updates(updateData).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Log audit
		userID := auditUserID(c)
		username := c.GetString("username")
		auditLog := models.AuditLog{
			ClusterID:  sa.ClusterID,
			UserID:     userID,
			Action:     "update",
			Resource:   "serviceaccount",
			ResourceID: strconv.Itoa(int(sa.ID)),
			User:       username,
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
		if !authorizeServiceAccount(db, c, &sa) {
			return
		}

		// Try to delete from Kubernetes cluster first
		if sa.Cluster.Kubeconfig == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cluster-specific Kubernetes credentials are required for deletion"})
			return
		}
		k8sClient, k8sErr := k8s.NewClientFromKubeconfig(sa.Cluster.Kubeconfig)
		if k8sErr != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to initialize the target cluster client"})
			return
		}
		k8sErr = k8sClient.DeleteServiceAccount(sa.Namespace, sa.Name)
		if k8sErr != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Kubernetes deletion failed; inventory record retained"})
			return
		}

		userID := auditUserID(c)
		username := c.GetString("username")
		auditLog := models.AuditLog{
			ClusterID:  sa.ClusterID,
			UserID:     userID,
			Action:     "delete",
			Resource:   "serviceaccount",
			ResourceID: strconv.Itoa(int(sa.ID)),
			User:       username,
			IP:         c.ClientIP(),
		}

		// Add K8s deletion result to audit details
		auditLog.Details = `{"k8s_deletion":"success"}`
		db.Create(&auditLog)

		// Delete from database
		if err := db.Delete(&sa).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Both Kubernetes and database deletion completed.
		message := "ServiceAccount deleted from Kubernetes cluster and database"
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
		if !authorizeServiceAccount(db, c, &sa) {
			return
		}

		updateData, ok := serviceAccountMetadataUpdate(c)
		if !ok {
			return
		}

		if err := db.Model(&sa).Updates(updateData).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		userID := auditUserID(c)
		username := c.GetString("username")
		auditLog := models.AuditLog{
			ClusterID:  sa.ClusterID,
			UserID:     userID,
			Action:     "update",
			Resource:   "serviceaccount",
			ResourceID: strconv.Itoa(int(sa.ID)),
			User:       username,
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
		if !authorizeServiceAccount(db, c, &sa) {
			return
		}

		if sa.Cluster.Kubeconfig == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "cluster-specific Kubernetes credentials are required for deletion"})
			return
		}
		k8sClient, k8sErr := k8s.NewClientFromKubeconfig(sa.Cluster.Kubeconfig)
		if k8sErr != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to initialize the target cluster client"})
			return
		}
		k8sErr = k8sClient.DeleteServiceAccount(sa.Namespace, sa.Name)
		if k8sErr != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Kubernetes deletion failed; inventory record retained"})
			return
		}

		userID := auditUserID(c)
		username := c.GetString("username")
		auditLog := models.AuditLog{
			ClusterID:  sa.ClusterID,
			UserID:     userID,
			Action:     "delete",
			Resource:   "serviceaccount",
			ResourceID: strconv.Itoa(int(sa.ID)),
			User:       username,
			IP:         c.ClientIP(),
		}
		auditLog.Details = `{"k8s_deletion":"success"}`
		db.Create(&auditLog)

		if err := db.Delete(&sa).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		message := "ServiceAccount deleted from Kubernetes cluster and database"
		c.JSON(http.StatusOK, gin.H{"message": message})
	}
}

// GetGraph is now in graph_handlers.go

// GetAuditLogs returns audit logs.
// Platform-wide listing and arbitrary filters require system.audit.read (admin).
// Callers with only findings.read may list rows for a single insight: GET /audit/logs?resource=insight&resource_id=<id>
// (prevents cross-finding enumeration / IDOR).

func GetAuditLogs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		granted := middleware.GrantedPermissions(c)
		hasSys := authorization.HasPermission(granted, authorization.PermissionSystemAuditRead)
		hasFindingsRead := authorization.HasPermission(granted, authorization.PermissionFindingsRead)
		if !hasSys && !hasFindingsRead {
			c.JSON(http.StatusForbidden, gin.H{
				"error":               "forbidden",
				"required_permission": string(authorization.PermissionSystemAuditRead),
				"hint":                "insight-scoped audit requires findings.read with resource=insight&resource_id=<insight_id>",
			})
			return
		}

		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
		if page < 1 {
			page = 1
		}
		if pageSize < 1 {
			pageSize = 50
		}
		if pageSize > 1000 {
			pageSize = 1000
		}
		offset := (page - 1) * pageSize

		var logs []models.AuditLog
		var query *gorm.DB

		if hasSys {
			query = db.Model(&models.AuditLog{})
			if clusterID := strings.TrimSpace(c.Query("cluster")); clusterID != "" {
				query = query.Where("cluster_id = ?", clusterID)
			}
			if resource := strings.TrimSpace(c.Query("resource")); resource != "" {
				query = query.Where("resource = ?", resource)
			}
			if action := strings.TrimSpace(c.Query("action")); action != "" {
				query = query.Where("action = ?", action)
			}
			if resourceID := strings.TrimSpace(c.Query("resource_id")); resourceID != "" {
				query = query.Where("resource_id = ?", resourceID)
			}
		} else {
			res := strings.ToLower(strings.TrimSpace(c.Query("resource")))
			rid := strings.TrimSpace(c.Query("resource_id"))
			if res != "insight" || rid == "" {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "forbidden",
					"hint":  "non-admin callers must pass resource=insight and a non-empty resource_id for the target finding",
				})
				return
			}
			var cnt int64
			if err := db.Model(&models.Insight{}).Where("id = ? AND deleted_at IS NULL", rid).Count(&cnt).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			if cnt == 0 {
				c.JSON(http.StatusNotFound, gin.H{"error": "insight not found"})
				return
			}
			query = db.Model(&models.AuditLog{}).
				Where("resource = ?", "insight").
				Where("resource_id = ?", rid)
		}

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

// escapeLikePattern escapes % and _ for use inside ILIKE patterns (PostgreSQL).
func escapeLikePattern(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

// GetPods returns all pods with optional filters
func GetPods(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := db.Model(&models.Pod{})

		clusterFilter := strings.TrimSpace(c.Query("cluster"))
		if clusterFilter != "" {
			query = query.Where("pods.cluster_id = ?", clusterFilter)
			if !middleware.ClusterAllowed(c, clusterFilter) {
				middleware.AbortClusterScopeDenied(db, c, clusterFilter)
				return
			}
		} else if ids, restricted := middleware.ScopedClusterIDs(c); restricted {
			query = query.Where("pods.cluster_id IN ?", ids)
		}
		if namespace := c.Query("namespace"); namespace != "" {
			query = query.Where("pods.namespace = ?", namespace)
		}
		if serviceAccount := c.Query("serviceAccount"); serviceAccount != "" {
			query = query.Where("pods.service_account = ?", serviceAccount)
		}
		if nodeName := c.Query("node"); nodeName != "" {
			query = query.Where("pods.node_name = ?", nodeName)
		}
		if rawSearch := strings.TrimSpace(c.Query("search")); rawSearch != "" {
			pat := "%" + escapeLikePattern(rawSearch) + "%"
			query = query.Where(
				`(pods.name ILIKE ? ESCAPE '\' OR pods.namespace ILIKE ? ESCAPE '\' OR pods.uid ILIKE ? ESCAPE '\' OR COALESCE(pods.node_name, '') ILIKE ? ESCAPE '\' )`,
				pat, pat, pat, pat,
			)
		}

		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
		if page < 1 {
			page = 1
		}
		if pageSize < 1 {
			pageSize = 50
		}
		if pageSize > 1000 {
			pageSize = 1000
		}
		if limStr := strings.TrimSpace(c.Query("limit")); limStr != "" {
			if n, err := strconv.Atoi(limStr); err == nil && n > 0 {
				pageSize = n
				if pageSize > 1000 {
					pageSize = 1000
				}
			}
		}
		if offStr := strings.TrimSpace(c.Query("offset")); offStr != "" {
			if off, err := strconv.Atoi(offStr); err == nil && off >= 0 && pageSize > 0 {
				page = off/pageSize + 1
				if page < 1 {
					page = 1
				}
			}
		}
		offset := (page - 1) * pageSize

		var total int64
		if err := query.Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		sortBy := strings.TrimSpace(c.Query("sortBy"))
		ctx := c.Request.Context()

		// Attack-path–aware ordering: has_attack_path DESC, max_impact DESC, risk_score DESC (spec §8).
		// Requires a cluster scope + graph.Build; load matching pods (capped), sort in Go, then paginate.
		if clusterFilter != "" && sortBy == "risk_desc" {
			var allPods []models.Pod
			qAll := query.Session(&gorm.Session{})
			if err := qAll.Order("pods.name ASC").Limit(maxPodsForAttackPathSort).Find(&allPods).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			riskByUID := loadActiveInsightCountsByPodUID(db, allPods)
			scores := loadLatestV3RiskScoresByPod(db, allPods)
			chainCache := make(map[string]*clusterChainsCacheEntry)
			rows := buildPodRowsWithRiskSignals(ctx, db, allPods, riskByUID, scores, chainCache)
			sortPodRowsByAttackPathPriority(rows)
			end := offset + pageSize
			if offset > len(rows) {
				offset = len(rows)
			}
			if end > len(rows) {
				end = len(rows)
			}
			pageRows := rows[offset:end]
			out := make([]interface{}, 0, len(pageRows))
			for i := range pageRows {
				out = append(out, pageRows[i])
			}
			c.JSON(http.StatusOK, gin.H{
				"pods":     out,
				"total":    total,
				"page":     page,
				"pageSize": pageSize,
			})
			return
		}

		var pods []models.Pod
		var findErr error
		switch sortBy {
		case "name_asc":
			findErr = query.Offset(offset).Limit(pageSize).Order("pods.name ASC, pods.namespace ASC").Find(&pods).Error
		case "namespace_asc":
			findErr = query.Offset(offset).Limit(pageSize).Order("pods.namespace ASC, pods.name ASC").Find(&pods).Error
		case "risk_desc":
			q2 := query.Session(&gorm.Session{})
			orderSQL := "pods.created_at DESC, pods.name ASC"
			if hasTable(db, "risk_scores") {
				q2 = q2.Joins(`LEFT JOIN (
					SELECT resource_uid, cluster_id, MAX(total_score) AS max_score
					FROM risk_scores
					WHERE LOWER(resource_type) = 'pod' AND deleted_at IS NULL AND LOWER(TRIM(COALESCE(scorer_version, ''))) = 'v3'
					GROUP BY resource_uid, cluster_id
				) _rs ON _rs.resource_uid = pods.uid AND _rs.cluster_id = pods.cluster_id`)
				orderSQL = "COALESCE(_rs.max_score, 0) DESC, pods.name ASC"
				if hasTable(db, "insights") {
					q2 = q2.Joins(`LEFT JOIN (
						SELECT resource_uid, COUNT(*) AS insight_cnt
						FROM insights
						WHERE deleted_at IS NULL AND (status = 'active' OR status IS NULL)
						GROUP BY resource_uid
					) _ins ON _ins.resource_uid = pods.uid`)
					orderSQL = "COALESCE(_rs.max_score, 0) DESC, COALESCE(_ins.insight_cnt, 0) DESC, pods.name ASC"
				}
			} else if hasTable(db, "insights") {
				orderSQL = `(SELECT COUNT(*) FROM insights WHERE insights.resource_uid = pods.uid AND insights.deleted_at IS NULL AND (insights.status = 'active' OR insights.status IS NULL)) DESC, pods.name ASC`
			}
			findErr = q2.Offset(offset).Limit(pageSize).Order(orderSQL).Find(&pods).Error
		case "created_desc":
			findErr = query.Offset(offset).Limit(pageSize).Order("pods.created_at DESC, pods.name ASC").Find(&pods).Error
		default:
			findErr = query.Offset(offset).Limit(pageSize).Order("pods.created_at DESC, pods.name ASC").Find(&pods).Error
		}
		if findErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": findErr.Error()})
			return
		}

		riskByUID := loadActiveInsightCountsByPodUID(db, pods)
		scores := loadLatestV3RiskScoresByPod(db, pods)
		chainCache := make(map[string]*clusterChainsCacheEntry)
		rows := buildPodRowsWithRiskSignals(ctx, db, pods, riskByUID, scores, chainCache)
		if sortBy == "risk_desc" {
			sortPodRowsByAttackPathPriority(rows)
		}
		out := make([]interface{}, 0, len(rows))
		for i := range rows {
			out = append(out, rows[i])
		}

		c.JSON(http.StatusOK, gin.H{
			"pods":     out,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		})
	}
}

func loadActiveInsightCountsByPodUID(db *gorm.DB, pods []models.Pod) map[string]int64 {
	riskByUID := make(map[string]int64)
	if len(pods) == 0 || !hasTable(db, "insights") {
		return riskByUID
	}
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
	return riskByUID
}

// podDetailEnvelope is the JSON shape for GET pod by id / by uid (inventory + legacy routes).
type podDetailEnvelope struct {
	models.Pod
	RiskCount     int64                     `json:"riskCount"`
	RiskSignals   graph.ResourceRiskSignals `json:"risk_signals"`
	UnifiedScore  *float64                  `json:"unifiedScore,omitempty"`
	FinalLevel    string                    `json:"finalLevel,omitempty"`
	ScorerVersion string                    `json:"scorerVersion,omitempty"`
}

func newPodDetailResponse(c *gin.Context, db *gorm.DB, pod models.Pod, riskCount int64) podDetailEnvelope {
	ctx := c.Request.Context()
	row := podDetailEnvelope{
		Pod:       pod,
		RiskCount: riskCount,
	}
	uval := 0.0
	if hasTable(db, "risk_scores") {
		var rs models.RiskScore
		if err := db.Where("resource_uid = ? AND cluster_id = ? AND LOWER(resource_type) = 'pod' AND deleted_at IS NULL AND LOWER(TRIM(COALESCE(scorer_version, ''))) = ?", pod.UID, pod.ClusterID, "v3").
			Order("calculated_at DESC, id DESC").First(&rs).Error; err == nil {
			uval = rs.TotalScore
			ts := rs.TotalScore
			row.UnifiedScore = &ts
			row.FinalLevel = risk.DeriveFinalLevelFromScore(ts)
			row.ScorerVersion = "v3"
		}
	}
	row.RiskSignals = podResourceRiskSignalsWithScore(ctx, db, pod, uval)
	return row
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
		if hasTable(db, "insights") {
			db.Raw(`
				SELECT COUNT(*) FROM insights
				WHERE deleted_at IS NULL AND (status = 'active' OR status IS NULL) AND resource_uid = ?
			`, pod.UID).Scan(&riskCount)
		}
		c.JSON(http.StatusOK, newPodDetailResponse(c, db, pod, riskCount))
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
		if hasTable(db, "insights") {
			db.Raw(`
				SELECT COUNT(*) FROM insights
				WHERE deleted_at IS NULL AND (status = 'active' OR status IS NULL) AND resource_uid = ?
			`, pod.UID).Scan(&riskCount)
		}
		c.JSON(http.StatusOK, newPodDetailResponse(c, db, pod, riskCount))
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

		query := db.Model(&models.AuditLog{})
		window := c.DefaultQuery("window", "24h")
		if window != "all" {
			cutoff := time.Now().Add(-24 * time.Hour)
			if since := c.Query("since"); since != "" {
				parsed, err := time.Parse(time.RFC3339, since)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "since must be RFC3339"})
					return
				}
				cutoff = parsed
			} else if hoursParam := c.Query("hours"); hoursParam != "" {
				hours, err := strconv.Atoi(hoursParam)
				if err != nil || hours <= 0 {
					c.JSON(http.StatusBadRequest, gin.H{"error": "hours must be a positive integer"})
					return
				}
				cutoff = time.Now().Add(-time.Duration(hours) * time.Hour)
			}
			query = query.Where("created_at >= ?", cutoff)
		}

		reports := make([]Report, 0)
		if err := query.
			Select("resource, action, COUNT(*) as count").
			Group("resource, action").
			Scan(&reports).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"reports": reports})
	}
}
