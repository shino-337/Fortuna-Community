package api

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
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
		if tryDedup(c, "metrics") {
			return
		}
		var req struct {
			PodUID    string                     `json:"podUid" binding:"required"`
			ClusterID string                     `json:"clusterId" binding:"required"`
			Namespace string                     `json:"namespace" binding:"required"`
			Metrics   []models.PodRuntimeMetrics `json:"metrics"`
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
		if tryDedup(c, "processes") {
			return
		}
		var req struct {
			PodUID        string              `json:"podUid" binding:"required"`
			ClusterID     string              `json:"clusterId" binding:"required"`
			Namespace     string              `json:"namespace" binding:"required"`
			RuntimeSource string              `json:"runtimeSource"` // optional: "host" | "exec" for UI indicator
			Processes     []models.PodProcess `json:"processes"`
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
		}
		// R7: process diff detection
		// Compare current snapshot with previous snapshot for the same pod; emit runtime events
		// only for newly appeared processes (container+pid key).
		newProcessEvents, err := buildProcessDiffEvents(db, req.PodUID, req.Namespace, now, req.Processes)
		if err != nil {
			log.Printf("[PodDetail] R7 diff detection skipped for pod %s: %v", req.PodUID, err)
			newProcessEvents = nil
		}
		for i := range req.Processes {
			// Phase 4.2: encrypt sensitive fields at-rest when POD_DETAIL_ENCRYPTION_KEY is set
			req.Processes[i].Command = EncryptSensitive(req.Processes[i].Command)
			req.Processes[i].BinaryPath = EncryptSensitive(req.Processes[i].BinaryPath)
			req.Processes[i].WorkingDir = EncryptSensitive(req.Processes[i].WorkingDir)
		}
		if len(req.Processes) > 0 {
			if err := dbIngestWithRetry(db, func(tx *gorm.DB) error {
				if err := tx.CreateInBatches(req.Processes, 100).Error; err != nil {
					return err
				}
				if len(newProcessEvents) > 0 {
					if err := tx.CreateInBatches(newProcessEvents, 100).Error; err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			BroadcastPodDetailUpdate(req.PodUID, "processes")
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "count": len(req.Processes)})
	}
}

func buildProcessDiffEvents(db *gorm.DB, podUID, namespace string, observedAt time.Time, current []models.PodProcess) ([]models.RuntimeEvent, error) {
	if podUID == "" || len(current) == 0 {
		return nil, nil
	}
	var prevSnapshot models.PodProcess
	if err := db.Table("pod_processes").
		Select("observed_at").
		Where("pod_uid = ? AND observed_at < ?", podUID, observedAt).
		Order("observed_at DESC").
		Limit(1).
		Take(&prevSnapshot).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	// First snapshot for this pod: no diff baseline.
	if prevSnapshot.ObservedAt.IsZero() {
		return nil, nil
	}

	var prev []models.PodProcess
	if err := db.Where("pod_uid = ? AND observed_at = ?", podUID, prevSnapshot.ObservedAt).Find(&prev).Error; err != nil {
		return nil, err
	}
	prevSet := make(map[string]struct{}, len(prev))
	for i := range prev {
		prevSet[processDiffKey(prev[i].ContainerName, prev[i].PID)] = struct{}{}
	}

	events := make([]models.RuntimeEvent, 0, len(current))
	for i := range current {
		k := processDiffKey(current[i].ContainerName, current[i].PID)
		if _, ok := prevSet[k]; ok {
			continue
		}
		target := strings.TrimSpace(current[i].BinaryPath)
		if target == "" {
			target = strings.TrimSpace(current[i].Command)
		}
		// RuntimeEvent.TargetPath is varchar(500)
		if len(target) > 500 {
			target = target[:500]
		}
		events = append(events, models.RuntimeEvent{
			PodUID:     podUID,
			Namespace:  namespace,
			Syscall:    "execve",
			TargetPath: target,
			Capability: "PROCESS_SNAPSHOT_DIFF",
			CreatedAt:  observedAt,
		})
		// Safety cap to avoid spikes on huge churn.
		if len(events) >= 500 {
			break
		}
	}
	return events, nil
}

func processDiffKey(container string, pid int) string {
	return container + "|" + strconv.Itoa(pid)
}

// IngestPodNetworkConnectionsPayload accepts POST from agent.
func IngestPodNetworkConnectionsPayload(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if tryDedup(c, "network") {
			return
		}
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
		newNetworkEvents, err := buildNetworkQueueSpikeEvents(db, req.PodUID, req.Namespace, now, req.Connections)
		if err != nil {
			log.Printf("[PodDetail] R5 network anomaly detection skipped for pod %s: %v", req.PodUID, err)
			newNetworkEvents = nil
		}
		if len(req.Connections) > 0 {
			// Use PrepareStmt to reuse INSERT plan; smaller batch (50) to reduce per-statement time and stay under PG param limit.
			session := db.Session(&gorm.Session{PrepareStmt: true})
			if err := dbIngestWithRetry(session, func(tx *gorm.DB) error {
				return tx.Transaction(func(tx2 *gorm.DB) error {
					if err := tx2.CreateInBatches(req.Connections, 50).Error; err != nil {
						return err
					}
					if len(newNetworkEvents) > 0 {
						if err := tx2.CreateInBatches(newNetworkEvents, 100).Error; err != nil {
							return err
						}
					}
					return nil
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

func buildNetworkQueueSpikeEvents(db *gorm.DB, podUID, namespace string, observedAt time.Time, current []models.PodNetworkConnection) ([]models.RuntimeEvent, error) {
	if podUID == "" || len(current) == 0 {
		return nil, nil
	}
	windowMin := envIntDefault("POD_DETAIL_NET_SPIKE_WINDOW_MINUTES", 30)
	minSamples := envIntDefault("POD_DETAIL_NET_SPIKE_MIN_SAMPLES", 5)
	multiplier := envFloatDefault("POD_DETAIL_NET_SPIKE_MULTIPLIER", 4.0)
	minQueueBytes := int64(envIntDefault("POD_DETAIL_NET_SPIKE_MIN_QUEUE_BYTES", 4096))
	cooldownMinutes := envIntDefault("POD_DETAIL_NET_SPIKE_COOLDOWN_MINUTES", 10)

	type baselineRow struct {
		ContainerName string
		DestIP        string
		DestPort      int
		Protocol      string
		AvgQueue      float64
		Samples       int64
	}
	var baseline []baselineRow
	fromTime := observedAt.Add(-time.Duration(windowMin) * time.Minute)
	if err := db.Table("pod_network_connections").
		Select("container_name, dest_ip, dest_port, protocol, AVG(bytes_sent + bytes_recv) AS avg_queue, COUNT(*) AS samples").
		Where("pod_uid = ? AND observed_at >= ? AND observed_at < ?", podUID, fromTime, observedAt).
		Group("container_name, dest_ip, dest_port, protocol").
		Scan(&baseline).Error; err != nil {
		return nil, err
	}
	baseMap := make(map[string]baselineRow, len(baseline))
	for i := range baseline {
		k := networkAnomalyKey(baseline[i].ContainerName, baseline[i].DestIP, baseline[i].DestPort, baseline[i].Protocol)
		baseMap[k] = baseline[i]
	}

	events := make([]models.RuntimeEvent, 0, len(current))
	emitted := make(map[string]struct{})
	for i := range current {
		c := current[i]
		k := networkAnomalyKey(c.ContainerName, c.DestIP, c.DestPort, c.Protocol)
		if _, ok := emitted[k]; ok {
			continue
		}
		base, ok := baseMap[k]
		if !ok || base.Samples < int64(minSamples) {
			continue
		}
		currentQueue := c.BytesSent + c.BytesRecv
		if currentQueue < minQueueBytes {
			continue
		}
		if base.AvgQueue <= 0 {
			continue
		}
		if float64(currentQueue) < base.AvgQueue*multiplier {
			continue
		}
		ratio := float64(currentQueue) / base.AvgQueue
		if suppressed, err := isNetworkSpikeSuppressed(db, podUID, k, observedAt, cooldownMinutes); err != nil {
			return nil, err
		} else if suppressed {
			continue
		}
		target := "key=" + k + " dst=" + strings.TrimSpace(c.DestIP) + ":" + strconv.Itoa(c.DestPort) +
			" proto=" + strings.ToLower(strings.TrimSpace(c.Protocol)) +
			" q=" + strconv.FormatInt(currentQueue, 10) +
			" avg=" + strconv.FormatFloat(base.AvgQueue, 'f', 0, 64) +
			" ratio=" + strconv.FormatFloat(ratio, 'f', 2, 64) +
			" samples=" + strconv.FormatInt(base.Samples, 10)
		if len(target) > 500 {
			target = target[:500]
		}
		events = append(events, models.RuntimeEvent{
			PodUID:     podUID,
			Namespace:  namespace,
			Syscall:    "connect",
			TargetPath: target,
			Capability: "NETWORK_TXRX_QUEUE_SPIKE",
			CreatedAt:  observedAt,
		})
		emitted[k] = struct{}{}
		if len(events) >= 100 {
			break
		}
	}
	return events, nil
}

func isNetworkSpikeSuppressed(db *gorm.DB, podUID, key string, observedAt time.Time, cooldownMinutes int) (bool, error) {
	if cooldownMinutes <= 0 {
		return false, nil
	}
	from := observedAt.Add(-time.Duration(cooldownMinutes) * time.Minute)
	var cnt int64
	err := db.Model(&models.RuntimeEvent{}).
		Where("pod_uid = ? AND capability = ? AND created_at >= ? AND target_path LIKE ?", podUID, "NETWORK_TXRX_QUEUE_SPIKE", from, "key="+key+" %").
		Count(&cnt).Error
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}

func networkAnomalyKey(container, destIP string, destPort int, proto string) string {
	return container + "|" + destIP + "|" + strconv.Itoa(destPort) + "|" + strings.ToLower(strings.TrimSpace(proto))
}

func envIntDefault(name string, def int) int {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func envFloatDefault(name string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

// IngestPodEventsPayload accepts POST from agent (K8s events; involved_uid can be pod UID).
func IngestPodEventsPayload(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if tryDedup(c, "events") {
			return
		}
		var req struct {
			ClusterID string            `json:"clusterId" binding:"required"`
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
