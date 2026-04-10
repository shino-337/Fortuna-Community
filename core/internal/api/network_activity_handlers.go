package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/networkbucket"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetNetworkActivity lists network connection data across workloads for a cluster.
// Reuses pod_network_connections (Pod Detail ingest) INNER JOIN pods so chỉ pod còn trong inventory (deleted_at NULL) — đồng bộ với Resources.
//
// Query:
//   - cluster (required): cluster id
//   - view: "connections" (default) | "pods" | "destinations" | "talkers" | "edges" — edges = aggregated (pod_uid, dest_ip, dest_port, protocol) for topology (fewer rows, more pods)
//   - namespace: filter source pod namespace (case-insensitive exact; suffix * = prefix match, e.g. kube*)
//   - q: search pod name, namespace, dest IP/port (connections view); pod name / namespace (pods view)
//   - podUid: optional exact filter on source pod UID (AND with namespace/q); index-friendly drill from dashboard
//   - sinceMinutes: only rows with bucket_5m >= floor5m(now - sinceMinutes) (aligned with stored buckets)
//   - page, pageSize: pagination (default page=1, pageSize=50; max pageSize: standard views NETWORK_ACTIVITY_MAX_PAGE_SIZE default 200 cap 500; view=edges NETWORK_ACTIVITY_TOPOLOGY_EDGES_MAX default 2500 cap 8000)
func GetNetworkActivity(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		clusterID := strings.TrimSpace(c.Query("cluster"))
		if clusterID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cluster query parameter is required"})
			return
		}
		clusterID = NormalizeClusterID(db, clusterID)

		view := strings.ToLower(strings.TrimSpace(c.DefaultQuery("view", "connections")))
		if view != "connections" && view != "pods" && view != "destinations" && view != "talkers" && view != "edges" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "view must be connections, pods, destinations, talkers, or edges"})
			return
		}

		namespace := strings.TrimSpace(c.Query("namespace"))
		q := strings.TrimSpace(c.Query("q"))
		podUID := strings.TrimSpace(c.Query("podUid"))
		sinceMin, _ := strconv.Atoi(strings.TrimSpace(c.Query("sinceMinutes")))
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
		if page < 1 {
			page = 1
		}
		if pageSize < 1 {
			pageSize = 50
		}
		maxPS := networkActivityPageSizeCap(view)
		if pageSize > maxPS {
			pageSize = maxPS
		}
		offset := (page - 1) * pageSize

		var sinceBucket *time.Time
		if sinceMin > 0 {
			sw := time.Now().UTC().Add(-time.Duration(sinceMin) * time.Minute)
			sb := networkbucket.FloorBucket5MUTC(sw)
			sinceBucket = &sb
		}

		if view == "pods" {
			handleNetworkActivityPodsView(c, db, clusterID, namespace, q, podUID, sinceBucket, page, pageSize, offset)
			return
		}
		if view == "destinations" {
			handleNetworkActivityDestinationsView(c, db, clusterID, namespace, q, podUID, sinceBucket, page, pageSize, offset)
			return
		}
		if view == "talkers" {
			handleNetworkActivityTalkersView(c, db, clusterID, namespace, q, podUID, sinceBucket, page, pageSize, offset)
			return
		}
		if view == "edges" {
			handleNetworkActivityEdgesView(c, db, clusterID, namespace, q, podUID, sinceBucket, page, pageSize, offset)
			return
		}
		handleNetworkActivityConnectionsView(c, db, clusterID, namespace, q, podUID, sinceBucket, page, pageSize, offset)
	}
}

func handleNetworkActivityConnectionsView(c *gin.Context, db *gorm.DB, clusterID, namespace, q, podUID string, sinceBucket *time.Time, page, pageSize, offset int) {
	base := joinPodsForNetworkActivity(db.Table("pod_network_connections AS n")).
		Where("n.cluster_id = ?", clusterID)
	base = applyNetworkActivityNamespaceFilter(base, namespace)
	base = applyNetworkActivityPodUidFilter(base, podUID)
	if sinceBucket != nil {
		base = base.Where("n.bucket_5m >= ?", *sinceBucket)
	}
	if q != "" {
		base = applyNetworkActivityConnectionsSearch(base, q)
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
		Bucket5m      time.Time `json:"bucket5m" gorm:"column:bucket_5m"`
		ObservedAt    time.Time `json:"observedAt" gorm:"column:observed_at"`
		CreatedAt     time.Time `json:"createdAt" gorm:"column:created_at"`
		RuntimeSource string    `json:"runtimeSource,omitempty" gorm:"column:runtime_source"`
		PodName       string    `json:"podName,omitempty" gorm:"column:pod_name"`
		OwnerKind     string    `json:"ownerKind,omitempty" gorm:"column:owner_kind"`
		OwnerName     string    `json:"ownerName,omitempty" gorm:"column:owner_name"`
		NodeName      string    `json:"nodeName,omitempty" gorm:"column:node_name"`
	}

	qb := joinPodsForNetworkActivity(db.Table("pod_network_connections AS n")).
		Select(`n.id, n.pod_uid, n.cluster_id, n.namespace, n.container_name, n.source_ip, n.source_port, n.dest_ip, n.dest_port, n.protocol, n.state, n.bytes_sent, n.bytes_recv, n.bucket_5m, n.observed_at, n.created_at, n.runtime_source,
			p.name AS pod_name, p.owner_kind, p.owner_name, p.node_name`).
		Where("n.cluster_id = ?", clusterID)
	qb = applyNetworkActivityNamespaceFilter(qb, namespace)
	qb = applyNetworkActivityPodUidFilter(qb, podUID)
	if sinceBucket != nil {
		qb = qb.Where("n.bucket_5m >= ?", *sinceBucket)
	}
	if q != "" {
		qb = applyNetworkActivityConnectionsSearch(qb, q)
	}

	var items []row
	if err := qb.Order("n.bucket_5m DESC, n.observed_at DESC").Offset(offset).Limit(pageSize).Scan(&items).Error; err != nil {
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

// handleNetworkActivityEdgesView returns one row per (source pod × dest IP/port/protocol), ordered by observation volume.
// Intended for dashboard topology (replaces sampling raw connection rows with the same 200-row cap).
func handleNetworkActivityEdgesView(c *gin.Context, db *gorm.DB, clusterID, namespace, q, podUID string, sinceBucket *time.Time, page, pageSize, offset int) {
	filtered := joinPodsForNetworkActivity(db.Table("pod_network_connections AS n")).
		Where("n.cluster_id = ?", clusterID)
	filtered = applyNetworkActivityNamespaceFilter(filtered, namespace)
	filtered = applyNetworkActivityPodUidFilter(filtered, podUID)
	if sinceBucket != nil {
		filtered = filtered.Where("n.bucket_5m >= ?", *sinceBucket)
	}
	if q != "" {
		filtered = applyNetworkActivityConnectionsSearch(filtered, q)
	}

	grouped := filtered.Session(&gorm.Session{}).
		Select("n.pod_uid, n.namespace, n.cluster_id, n.dest_ip, n.dest_port, n.protocol").
		Group("n.pod_uid, n.namespace, n.cluster_id, n.dest_ip, n.dest_port, n.protocol")

	var total int64
	if err := db.Table("(?) AS g", grouped).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type edgeRow struct {
		PodUID           string    `json:"podUid" gorm:"column:pod_uid"`
		Namespace        string    `json:"namespace" gorm:"column:namespace"`
		ClusterID        string    `json:"clusterId" gorm:"column:cluster_id"`
		DestIP           string    `json:"destIp" gorm:"column:dest_ip"`
		DestPort         int       `json:"destPort" gorm:"column:dest_port"`
		Protocol         string    `json:"protocol" gorm:"column:protocol"`
		ObservationCount int64     `json:"observationCount" gorm:"column:observation_count"`
		LastObservedAt   time.Time `json:"lastObservedAt" gorm:"column:last_observed_at"`
		PodName          string    `json:"podName,omitempty" gorm:"column:pod_name"`
		OwnerKind        string    `json:"ownerKind,omitempty" gorm:"column:owner_kind"`
		OwnerName        string    `json:"ownerName,omitempty" gorm:"column:owner_name"`
		NodeName         string    `json:"nodeName,omitempty" gorm:"column:node_name"`
	}

	qb := filtered.Session(&gorm.Session{}).
		Select(`n.pod_uid, n.namespace, n.cluster_id, n.dest_ip, n.dest_port, n.protocol,
			COUNT(*) AS observation_count,
			MAX(n.observed_at) AS last_observed_at,
			MAX(p.name) AS pod_name,
			MAX(p.owner_kind) AS owner_kind,
			MAX(p.owner_name) AS owner_name,
			MAX(p.node_name) AS node_name`).
		Group("n.pod_uid, n.namespace, n.cluster_id, n.dest_ip, n.dest_port, n.protocol").
		Order("observation_count DESC").
		Offset(offset).
		Limit(pageSize)

	var items []edgeRow
	if err := qb.Scan(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"view":      "edges",
		"clusterId": clusterID,
		"total":     total,
		"page":      page,
		"pageSize":  pageSize,
		"items":     items,
	})
}

func handleNetworkActivityPodsView(c *gin.Context, db *gorm.DB, clusterID, namespace, q, podUID string, sinceBucket *time.Time, page, pageSize, offset int) {
	// Subquery for grouping — count distinct pod groups matching filters
	sub := joinPodsForNetworkActivity(db.Table("pod_network_connections AS n")).
		Select("n.pod_uid, n.namespace, n.cluster_id, COUNT(*) AS connection_count, MAX(n.observed_at) AS last_observed_at, MAX(p.name) AS pod_name, MAX(p.owner_kind) AS owner_kind, MAX(p.owner_name) AS owner_name, MAX(p.node_name) AS node_name").
		Where("n.cluster_id = ?", clusterID).
		Group("n.pod_uid, n.namespace, n.cluster_id")
	sub = applyNetworkActivityNamespaceFilter(sub, namespace)
	sub = applyNetworkActivityPodUidFilter(sub, podUID)
	if sinceBucket != nil {
		sub = sub.Where("n.bucket_5m >= ?", *sinceBucket)
	}
	if q != "" {
		sub = applyNetworkActivityPodsSubSearch(sub, q)
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

// clusterDestinationRow is aggregated remote endpoints across pods in a cluster (same filters as connections view).
type clusterDestinationRow struct {
	DestIP               string    `json:"destIp" gorm:"column:dest_ip"`
	DestPort             int       `json:"destPort" gorm:"column:dest_port"`
	Protocol             string    `json:"protocol" gorm:"column:protocol"`
	ObservationCount     int64     `json:"observationCount" gorm:"column:observation_count"`
	LastObservedAt       time.Time `json:"lastObservedAt" gorm:"column:last_observed_at"`
	DistinctPodCount     int64     `json:"distinctPodCount" gorm:"column:distinct_pod_count"`
	DistinctBucketCount  int64     `json:"distinctBucketCount" gorm:"column:distinct_bucket_count"`
	DestWorkloadName     string    `json:"destWorkloadName,omitempty" gorm:"column:dest_workload_name"`         // pods.pod_ip = dest_ip (pod-to-pod), not K8s Service ClusterIP
	DestWorkloadNS       string    `json:"destWorkloadNamespace,omitempty" gorm:"column:dest_workload_namespace"` // namespace of matched pod, if any
}

func handleNetworkActivityDestinationsView(c *gin.Context, db *gorm.DB, clusterID, namespace, q, podUID string, sinceBucket *time.Time, page, pageSize, offset int) {
	// INNER JOIN source pod (p) so chỉ cộng lưu lượng từ workload còn trong inventory; pdest join riêng cho tên đích (pod-to-pod).
	filtered := joinPodsForNetworkActivity(db.Table("pod_network_connections AS n")).
		Where("n.cluster_id = ?", clusterID)
	filtered = applyNetworkActivityNamespaceFilter(filtered, namespace)
	filtered = applyNetworkActivityPodUidFilter(filtered, podUID)
	if sinceBucket != nil {
		filtered = filtered.Where("n.bucket_5m >= ?", *sinceBucket)
	}
	if q != "" {
		filtered = applyNetworkActivityConnectionsSearch(filtered, q)
	}

	grouped := filtered.Session(&gorm.Session{}).
		Select("n.dest_ip, n.dest_port, n.protocol").
		Group("n.dest_ip, n.dest_port, n.protocol")

	var total int64
	if err := db.Table("(?) AS g", grouped).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	qb := filtered.Session(&gorm.Session{}).
		Joins(`LEFT JOIN pods pdest ON pdest.cluster_id = n.cluster_id AND pdest.pod_ip = n.dest_ip AND pdest.pod_ip <> '' AND pdest.deleted_at IS NULL`).
		Select(`n.dest_ip, n.dest_port, n.protocol,
			COUNT(DISTINCT n.id) AS observation_count,
			MAX(n.observed_at) AS last_observed_at,
			COUNT(DISTINCT n.pod_uid) AS distinct_pod_count,
			COUNT(DISTINCT n.bucket_5m) AS distinct_bucket_count,
			MAX(pdest.name) AS dest_workload_name,
			MAX(pdest.namespace) AS dest_workload_namespace`).
		Group("n.dest_ip, n.dest_port, n.protocol").
		Order("observation_count DESC").
		Offset(offset).
		Limit(pageSize)

	var items []clusterDestinationRow
	if err := qb.Scan(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"view":      "destinations",
		"clusterId": clusterID,
		"total":     total,
		"page":      page,
		"pageSize":  pageSize,
		"items":     items,
	})
}

// clusterTalkerRow is per-pod aggregate: how many observation rows and distinct dest fingerprints this pod produced.
type clusterTalkerRow struct {
	PodUID              string    `json:"podUid" gorm:"column:pod_uid"`
	Namespace           string    `json:"namespace" gorm:"column:namespace"`
	ClusterID           string    `json:"clusterId" gorm:"column:cluster_id"`
	PodName             string    `json:"podName,omitempty" gorm:"column:pod_name"`
	OwnerKind           string    `json:"ownerKind,omitempty" gorm:"column:owner_kind"`
	OwnerName           string    `json:"ownerName,omitempty" gorm:"column:owner_name"`
	NodeName            string    `json:"nodeName,omitempty" gorm:"column:node_name"`
	ObservationCount    int64     `json:"observationCount" gorm:"column:observation_count"`
	LastObservedAt      time.Time `json:"lastObservedAt" gorm:"column:last_observed_at"`
	DistinctBucketCount int64     `json:"distinctBucketCount" gorm:"column:distinct_bucket_count"`
	DistinctDestCount   int64     `json:"distinctDestCount" gorm:"column:distinct_dest_count"`
}

func handleNetworkActivityTalkersView(c *gin.Context, db *gorm.DB, clusterID, namespace, q, podUID string, sinceBucket *time.Time, page, pageSize, offset int) {
	base := joinPodsForNetworkActivity(db.Table("pod_network_connections AS n")).
		Where("n.cluster_id = ?", clusterID)
	base = applyNetworkActivityNamespaceFilter(base, namespace)
	base = applyNetworkActivityPodUidFilter(base, podUID)
	if sinceBucket != nil {
		base = base.Where("n.bucket_5m >= ?", *sinceBucket)
	}
	if q != "" {
		base = applyNetworkActivityConnectionsSearch(base, q)
	}

	grouped := base.Session(&gorm.Session{}).
		Select("n.pod_uid, n.namespace, n.cluster_id").
		Group("n.pod_uid, n.namespace, n.cluster_id")

	var total int64
	if err := db.Table("(?) AS g", grouped).Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	qb := base.Session(&gorm.Session{}).
		Select(`n.pod_uid, n.namespace, n.cluster_id,
			MAX(p.name) AS pod_name,
			MAX(p.owner_kind) AS owner_kind,
			MAX(p.owner_name) AS owner_name,
			MAX(p.node_name) AS node_name,
			COUNT(DISTINCT n.id) AS observation_count,
			MAX(n.observed_at) AS last_observed_at,
			COUNT(DISTINCT n.bucket_5m) AS distinct_bucket_count,
			COUNT(DISTINCT (n.dest_ip || E'\x1f' || n.dest_port::text || E'\x1f' || COALESCE(n.protocol, ''))) AS distinct_dest_count`).
		Group("n.pod_uid, n.namespace, n.cluster_id").
		Order("observation_count DESC").
		Offset(offset).
		Limit(pageSize)

	var items []clusterTalkerRow
	if err := qb.Scan(&items).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"view":      "talkers",
		"clusterId": clusterID,
		"total":     total,
		"page":      page,
		"pageSize":  pageSize,
		"items":     items,
	})
}
