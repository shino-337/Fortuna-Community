package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// resolvePodUIDByID returns pod UID for GET by numeric id; 404 if not found.
func resolvePodUIDByID(db *gorm.DB, idStr string) (string, int, error) {
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return "", 0, gorm.ErrRecordNotFound
	}
	var pod models.Pod
	if err := db.Where("id = ?", id).First(&pod).Error; err != nil {
		return "", 0, err
	}
	return pod.UID, int(id), nil
}

// --- GET: runtime-metrics ---

func GetPodRuntimeMetrics(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		uid, _, err := resolvePodUIDByID(db, id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		getPodRuntimeMetricsByUID(c, db, uid)
	}
}

func GetPodRuntimeMetricsByUID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("uid")
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid is required"})
			return
		}
		getPodRuntimeMetricsByUID(c, db, podUID)
	}
}

func getPodRuntimeMetricsByUID(c *gin.Context, db *gorm.DB, podUID string) {
	var list []models.PodRuntimeMetrics
	if err := db.Where("pod_uid = ?", podUID).Order("last_observed_at DESC").Limit(500).Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"podUid": podUID, "items": list})
}

// --- GET: processes ---

func GetPodProcesses(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		uid, _, err := resolvePodUIDByID(db, id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		getPodProcessesByUID(c, db, uid)
	}
}

func GetPodProcessesByUID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("uid")
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid is required"})
			return
		}
		getPodProcessesByUID(c, db, podUID)
	}
}

func getPodProcessesByUID(c *gin.Context, db *gorm.DB, podUID string) {
	var list []models.PodProcess
	if err := db.Where("pod_uid = ?", podUID).Order("observed_at DESC").Limit(1000).Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	decryptProcessList(list)
	c.JSON(http.StatusOK, gin.H{"podUid": podUID, "items": list})
}

// --- GET: network-connections ---

func GetPodNetworkConnections(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		uid, _, err := resolvePodUIDByID(db, id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		getPodNetworkConnectionsByUID(c, db, uid)
	}
}

func GetPodNetworkConnectionsByUID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("uid")
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid is required"})
			return
		}
		getPodNetworkConnectionsByUID(c, db, podUID)
	}
}

func getPodNetworkConnectionsByUID(c *gin.Context, db *gorm.DB, podUID string) {
	var list []models.PodNetworkConnection
	if err := db.Where("pod_uid = ?", podUID).Order("observed_at DESC").Limit(500).Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"podUid": podUID, "items": list})
}

// --- GET: events (K8s events where involved_uid = pod UID) ---

func GetPodEvents(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		uid, _, err := resolvePodUIDByID(db, id)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		getPodEventsByUID(c, db, uid)
	}
}

func GetPodEventsByUID(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		podUID := c.Param("uid")
		if podUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid is required"})
			return
		}
		getPodEventsByUID(c, db, podUID)
	}
}

func getPodEventsByUID(c *gin.Context, db *gorm.DB, podUID string) {
	var list []models.K8sEvent
	if err := db.Where("involved_uid = ?", podUID).Order("last_timestamp DESC NULLS LAST").Limit(200).Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"podUid": podUID, "items": list})
}

// --- POST ingest (agent, no auth) ---

func rejectPodUid(uid string) bool {
	return uid == "" || uid == "0"
}

// IngestPodRuntimeMetricsPayload accepts POST from agent.
func IngestPodRuntimeMetricsPayload(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			PodUID     string                      `json:"podUid" binding:"required"`
			ClusterID  string                      `json:"clusterId" binding:"required"`
			Namespace  string                      `json:"namespace" binding:"required"`
			Metrics    []models.PodRuntimeMetrics  `json:"metrics"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if rejectPodUid(req.PodUID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid required and must not be 0 or empty"})
			return
		}
		now := time.Now()
		for i := range req.Metrics {
			req.Metrics[i].PodUID = req.PodUID
			req.Metrics[i].ClusterID = req.ClusterID
			req.Metrics[i].Namespace = req.Namespace
			req.Metrics[i].LastObservedAt = now
			req.Metrics[i].CreatedAt = now
			req.Metrics[i].UpdatedAt = now
		}
		if len(req.Metrics) > 0 {
			if err := dbIngestWithRetry(db, func(tx *gorm.DB) error { return tx.CreateInBatches(req.Metrics, 50).Error }); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			BroadcastPodDetailUpdate(req.PodUID, "metrics")
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "count": len(req.Metrics)})
	}
}

// IngestPodProcessesPayload accepts POST from agent.
func IngestPodProcessesPayload(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			PodUID        string               `json:"podUid" binding:"required"`
			ClusterID     string               `json:"clusterId" binding:"required"`
			Namespace     string               `json:"namespace" binding:"required"`
			RuntimeSource string               `json:"runtimeSource"` // optional: "host" | "exec" for UI indicator
			Processes     []models.PodProcess  `json:"processes"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if rejectPodUid(req.PodUID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid required and must not be 0 or empty"})
			return
		}
		runtimeSource := req.RuntimeSource
		if runtimeSource != "host" && runtimeSource != "exec" {
			runtimeSource = "exec"
		}
		now := time.Now()
		for i := range req.Processes {
			req.Processes[i].PodUID = req.PodUID
			req.Processes[i].ClusterID = req.ClusterID
			req.Processes[i].Namespace = req.Namespace
			req.Processes[i].ObservedAt = now
			req.Processes[i].CreatedAt = now
			req.Processes[i].RuntimeSource = runtimeSource
			// Phase 4.2: encrypt sensitive fields at-rest when POD_DETAIL_ENCRYPTION_KEY is set
			req.Processes[i].Command = EncryptSensitive(req.Processes[i].Command)
			req.Processes[i].BinaryPath = EncryptSensitive(req.Processes[i].BinaryPath)
		}
		if len(req.Processes) > 0 {
			if err := dbIngestWithRetry(db, func(tx *gorm.DB) error { return tx.CreateInBatches(req.Processes, 100).Error }); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			BroadcastPodDetailUpdate(req.PodUID, "processes")
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "count": len(req.Processes)})
	}
}

// IngestPodNetworkConnectionsPayload accepts POST from agent.
func IngestPodNetworkConnectionsPayload(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			PodUID        string                        `json:"podUid" binding:"required"`
			ClusterID     string                        `json:"clusterId" binding:"required"`
			Namespace     string                        `json:"namespace" binding:"required"`
			RuntimeSource string                        `json:"runtimeSource"` // optional: "host" | "exec" for UI indicator
			Connections   []models.PodNetworkConnection `json:"connections"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if rejectPodUid(req.PodUID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid required and must not be 0 or empty"})
			return
		}
		runtimeSource := req.RuntimeSource
		if runtimeSource != "host" && runtimeSource != "exec" {
			runtimeSource = "exec"
		}
		now := time.Now()
		for i := range req.Connections {
			req.Connections[i].PodUID = req.PodUID
			req.Connections[i].ClusterID = req.ClusterID
			req.Connections[i].Namespace = req.Namespace
			req.Connections[i].ObservedAt = now
			req.Connections[i].CreatedAt = now
			req.Connections[i].RuntimeSource = runtimeSource
		}
		if len(req.Connections) > 0 {
			// Use PrepareStmt to reuse INSERT plan; smaller batch (50) to reduce per-statement time and stay under PG param limit.
			session := db.Session(&gorm.Session{PrepareStmt: true})
			if err := dbIngestWithRetry(session, func(tx *gorm.DB) error {
				return tx.Transaction(func(tx2 *gorm.DB) error {
					return tx2.CreateInBatches(req.Connections, 50).Error
				})
			}); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			BroadcastPodDetailUpdate(req.PodUID, "network")
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "count": len(req.Connections)})
	}
}

// IngestPodEventsPayload accepts POST from agent (K8s events; involved_uid can be pod UID).
func IngestPodEventsPayload(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ClusterID string           `json:"clusterId" binding:"required"`
			Events    []models.K8sEvent `json:"events"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		now := time.Now()
		for i := range req.Events {
			req.Events[i].ClusterID = req.ClusterID
			req.Events[i].CreatedAt = now
		}
		if len(req.Events) > 0 {
			// Unique on (cluster_id, event_uid): ignore duplicate when informer resyncs
			if err := dbIngestWithRetry(db, func(tx *gorm.DB) error {
				return tx.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "cluster_id"}, {Name: "event_uid"}},
					DoNothing: true,
				}).CreateInBatches(req.Events, 50).Error
			}); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			// Broadcast for each distinct pod UID in the batch (events can reference different pods)
			seen := make(map[string]struct{})
			for i := range req.Events {
				u := req.Events[i].InvolvedUID
				if u != "" && u != "0" {
					if _, ok := seen[u]; !ok {
						seen[u] = struct{}{}
						BroadcastPodDetailUpdate(u, "events")
					}
				}
			}
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "count": len(req.Events)})
	}
}
