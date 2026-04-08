package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetNetworkActivity lists network connection data across workloads for a cluster.
// Reuses pod_network_connections (Pod Detail ingest) joined to pods for display names.
//
// Query:
//   - cluster (required): cluster id
//   - view: "connections" (default) | "pods" — flat rows vs one row per pod (aggregated)
//   - namespace: filter
//   - q: search pod name, namespace, dest IP/port (connections view); pod name / namespace (pods view)
//   - sinceMinutes: only rows with observed_at >= now - sinceMinutes
//   - page, pageSize: pagination (default page=1, pageSize=50, max 200)
func GetNetworkActivity(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID := strings.TrimSpace(c.Query("cluster"))
		if clusterID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cluster query parameter is required"})
			return
		}
		clusterID = NormalizeClusterID(db, clusterID)

		view := strings.ToLower(strings.TrimSpace(c.DefaultQuery("view", "connections")))
		if view != "connections" && view != "pods" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "view must be connections or pods"})
			return
		}

		namespace := strings.TrimSpace(c.Query("namespace"))
		q := strings.TrimSpace(c.Query("q"))
		sinceMin, _ := strconv.Atoi(strings.TrimSpace(c.Query("sinceMinutes")))
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
		if page < 1 {
			page = 1
		}
		if pageSize < 1 {
			pageSize = 50
		}
		if pageSize > 200 {
			pageSize = 200
		}
		offset := (page - 1) * pageSize

		var since *time.Time
		if sinceMin > 0 {
			t := time.Now().Add(-time.Duration(sinceMin) * time.Minute)
			since = &t
		}

		if view == "pods" {
			handleNetworkActivityPodsView(c, db, clusterID, namespace, q, since, page, pageSize, offset)
			return
		}
		handleNetworkActivityConnectionsView(c, db, clusterID, namespace, q, since, page, pageSize, offset)
	}
}

func handleNetworkActivityConnectionsView(c *gin.Context, db *gorm.DB, clusterID, namespace, q string, since *time.Time, page, pageSize, offset int) {
	base := db.Table("pod_network_connections AS n").
		Where("n.cluster_id = ?", clusterID)
	if namespace != "" {
		base = base.Where("n.namespace = ?", namespace)
	}
	if since != nil {
		base = base.Where("n.observed_at >= ?", *since)
	}
	if q != "" {
		like := "%" + strings.ToLower(q) + "%"
		base = base.Joins("LEFT JOIN pods p ON p.uid = n.pod_uid AND p.cluster_id = n.cluster_id AND p.deleted_at IS NULL").
			Where(`(LOWER(COALESCE(p.name,'')) LIKE ? OR LOWER(n.namespace) LIKE ? OR LOWER(COALESCE(n.dest_ip,'')) LIKE ? OR CAST(n.dest_port AS TEXT) LIKE ? OR CAST(n.source_port AS TEXT) LIKE ?)`,
				like, like, like, like, like)
	}

	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type row struct {
		ID            uint      `json:"id" gorm:"column:id"`
		PodUID        string    `json:"podUid" gorm:"column:pod_uid"`
		ClusterID     string    `json:"clusterId" gorm:"column:cluster_id"`
		Namespace     string    `json:"namespace" gorm:"column:namespace"`
		ContainerName string    `json:"containerName" gorm:"column:container_name"`
		SourceIP      string    `json:"sourceIp" gorm:"column:source_ip"`
		SourcePort    int       `json:"sourcePort" gorm:"column:source_port"`
		DestIP        string    `json:"destIp" gorm:"column:dest_ip"`
		DestPort      int       `json:"destPort" gorm:"column:dest_port"`
		Protocol      string    `json:"protocol" gorm:"column:protocol"`
		State         string    `json:"state" gorm:"column:state"`
		BytesSent     int64     `json:"bytesSent" gorm:"column:bytes_sent"`
		BytesRecv     int64     `json:"bytesRecv" gorm:"column:bytes_recv"`
		ObservedAt    time.Time `json:"observedAt" gorm:"column:observed_at"`
		CreatedAt     time.Time `json:"createdAt" gorm:"column:created_at"`
		RuntimeSource string    `json:"runtimeSource,omitempty" gorm:"column:runtime_source"`
		PodName       string    `json:"podName,omitempty" gorm:"column:pod_name"`
		OwnerKind     string    `json:"ownerKind,omitempty" gorm:"column:owner_kind"`
		OwnerName     string    `json:"ownerName,omitempty" gorm:"column:owner_name"`
		NodeName      string    `json:"nodeName,omitempty" gorm:"column:node_name"`
	}

	qb := db.Table("pod_network_connections AS n").
		Select(`n.id, n.pod_uid, n.cluster_id, n.namespace, n.container_name, n.source_ip, n.source_port, n.dest_ip, n.dest_port, n.protocol, n.state, n.bytes_sent, n.bytes_recv, n.observed_at, n.created_at, n.runtime_source,
			p.name AS pod_name, p.owner_kind, p.owner_name, p.node_name`).
		Joins("LEFT JOIN pods p ON p.uid = n.pod_uid AND p.cluster_id = n.cluster_id AND p.deleted_at IS NULL").
		Where("n.cluster_id = ?", clusterID)
	if namespace != "" {
		qb = qb.Where("n.namespace = ?", namespace)
	}
	if since != nil {
		qb = qb.Where("n.observed_at >= ?", *since)
	}
	if q != "" {
		like := "%" + strings.ToLower(q) + "%"
		qb = qb.Where(`(LOWER(COALESCE(p.name,'')) LIKE ? OR LOWER(n.namespace) LIKE ? OR LOWER(COALESCE(n.dest_ip,'')) LIKE ? OR CAST(n.dest_port AS TEXT) LIKE ? OR CAST(n.source_port AS TEXT) LIKE ?)`,
			like, like, like, like, like)
	}

	var items []row
	if err := qb.Order("n.observed_at DESC").Offset(offset).Limit(pageSize).Scan(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"view":     "connections",
		"clusterId": clusterID,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
		"items":    items,
	})
}

func handleNetworkActivityPodsView(c *gin.Context, db *gorm.DB, clusterID, namespace, q string, since *time.Time, page, pageSize, offset int) {
	// Subquery for grouping — count distinct pod groups matching filters
	sub := db.Table("pod_network_connections AS n").
		Select("n.pod_uid, n.namespace, n.cluster_id, COUNT(*) AS connection_count, MAX(n.observed_at) AS last_observed_at, MAX(p.name) AS pod_name, MAX(p.owner_kind) AS owner_kind, MAX(p.owner_name) AS owner_name, MAX(p.node_name) AS node_name").
		Joins("LEFT JOIN pods p ON p.uid = n.pod_uid AND p.cluster_id = n.cluster_id AND p.deleted_at IS NULL").
		Where("n.cluster_id = ?", clusterID).
		Group("n.pod_uid, n.namespace, n.cluster_id")
	if namespace != "" {
		sub = sub.Where("n.namespace = ?", namespace)
	}
	if since != nil {
		sub = sub.Where("n.observed_at >= ?", *since)
	}
	if q != "" {
		like := "%" + strings.ToLower(q) + "%"
		sub = sub.Having(`(LOWER(MAX(COALESCE(p.name,''))) LIKE ? OR LOWER(MAX(n.namespace)) LIKE ?)`, like, like)
	}

	// Count groups — wrap subquery
	var total int64
	countSQL := "SELECT COUNT(*) FROM (?) AS t"
	if err := db.Raw(countSQL, sub).Scan(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type podRow struct {
		PodUID           string    `json:"podUid" gorm:"column:pod_uid"`
		Namespace        string    `json:"namespace" gorm:"column:namespace"`
		ClusterID        string    `json:"clusterId" gorm:"column:cluster_id"`
		ConnectionCount  int64     `json:"connectionCount" gorm:"column:connection_count"`
		LastObservedAt   time.Time `json:"lastObservedAt" gorm:"column:last_observed_at"`
		PodName          string    `json:"podName,omitempty" gorm:"column:pod_name"`
		OwnerKind        string    `json:"ownerKind,omitempty" gorm:"column:owner_kind"`
		OwnerName        string    `json:"ownerName,omitempty" gorm:"column:owner_name"`
		NodeName         string    `json:"nodeName,omitempty" gorm:"column:node_name"`
	}

	outer := db.Table("(?) AS w", sub).
		Select("*").
		Order("w.last_observed_at DESC").
		Offset(offset).
		Limit(pageSize)

	var items []podRow
	if err := outer.Scan(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"view":      "pods",
		"clusterId": clusterID,
		"total":     total,
		"page":      page,
		"pageSize":  pageSize,
		"items":     items,
	})
}
