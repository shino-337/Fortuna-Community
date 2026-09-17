package api

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/networkbucket"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// IngestPodProcessesPayloadScoped keeps process snapshots and process-diff
// RuntimeEvents inside the same canonical cluster-qualified Pod identity.
func IngestPodProcessesPayloadScoped(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if tryDedup(c, "processes") {
			return
		}
		var req struct {
			PodUID        string              `json:"podUid" binding:"required"`
			ClusterID     string              `json:"clusterId" binding:"required"`
			Namespace     string              `json:"namespace" binding:"required"`
			RuntimeSource string              `json:"runtimeSource"`
			Processes     []models.PodProcess `json:"processes"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if rejectPodUid(req.PodUID) || strings.TrimSpace(req.ClusterID) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid and clusterId are required"})
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
		newEvents, err := buildProcessDiffEventsScoped(db, req.ClusterID, req.PodUID, req.Namespace, now, req.Processes)
		if err != nil {
			log.Printf("[PodDetail] scoped process diff skipped for %s/%s: %v", req.ClusterID, req.PodUID, err)
			newEvents = nil
		}
		for i := range req.Processes {
			req.Processes[i].Command = EncryptSensitive(req.Processes[i].Command)
			req.Processes[i].BinaryPath = EncryptSensitive(req.Processes[i].BinaryPath)
			req.Processes[i].WorkingDir = EncryptSensitive(req.Processes[i].WorkingDir)
		}
		if len(req.Processes) > 0 {
			if err := dbIngestWithRetry(db, func(tx *gorm.DB) error {
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(req.Processes, 100).Error; err != nil {
					return err
				}
				if len(newEvents) > 0 {
					return tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(newEvents, 100).Error
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

func buildProcessDiffEventsScoped(db *gorm.DB, clusterID, podUID, namespace string, observedAt time.Time, current []models.PodProcess) ([]models.RuntimeEvent, error) {
	if clusterID == "" || podUID == "" || len(current) == 0 {
		return nil, nil
	}
	var prevSnapshot models.PodProcess
	if err := db.Table("pod_processes").Select("observed_at").
		Where("cluster_id = ? AND pod_uid = ? AND observed_at < ?", clusterID, podUID, observedAt).
		Order("observed_at DESC").Limit(1).Take(&prevSnapshot).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	if prevSnapshot.ObservedAt.IsZero() {
		return nil, nil
	}
	var prev []models.PodProcess
	if err := db.Where("cluster_id = ? AND pod_uid = ? AND observed_at = ?", clusterID, podUID, prevSnapshot.ObservedAt).Find(&prev).Error; err != nil {
		return nil, err
	}
	prevSet := make(map[string]struct{}, len(prev))
	for i := range prev {
		prevSet[processDiffKey(prev[i].ContainerName, prev[i].PID)] = struct{}{}
	}
	events := make([]models.RuntimeEvent, 0, len(current))
	for i := range current {
		if _, ok := prevSet[processDiffKey(current[i].ContainerName, current[i].PID)]; ok {
			continue
		}
		target := strings.TrimSpace(current[i].BinaryPath)
		if target == "" {
			target = strings.TrimSpace(current[i].Command)
		}
		if len(target) > 500 {
			target = target[:500]
		}
		events = append(events, models.RuntimeEvent{
			ClusterID: clusterID, PodUID: podUID, Namespace: namespace, Syscall: "execve",
			TargetPath: target, PayloadJSON: `{}`, Capability: "PROCESS_SNAPSHOT_DIFF",
			CreatedAt: observedAt, Confidence: 0.75,
		})
		if len(events) >= 500 {
			break
		}
	}
	return events, nil
}

// IngestPodNetworkConnectionsPayloadScoped preserves cluster identity for both
// network observations and network-anomaly RuntimeEvents.
func IngestPodNetworkConnectionsPayloadScoped(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if tryDedup(c, "network") {
			return
		}
		var req struct {
			PodUID        string                        `json:"podUid" binding:"required"`
			ClusterID     string                        `json:"clusterId" binding:"required"`
			Namespace     string                        `json:"namespace" binding:"required"`
			RuntimeSource string                        `json:"runtimeSource"`
			Connections   []models.PodNetworkConnection `json:"connections"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if rejectPodUid(req.PodUID) || strings.TrimSpace(req.ClusterID) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "podUid and clusterId are required"})
			return
		}
		runtimeSource := req.RuntimeSource
		if runtimeSource != "host" && runtimeSource != "exec" {
			runtimeSource = "exec"
		}
		now := time.Now().UTC()
		bucket := networkbucket.FloorBucket5MUTC(now)
		normalized := make([]models.PodNetworkConnection, len(req.Connections))
		for i := range req.Connections {
			row := req.Connections[i]
			normalizePodNetworkForUpsert(&row)
			normalized[i] = models.PodNetworkConnection{
				PodUID: req.PodUID, ClusterID: req.ClusterID, Namespace: req.Namespace,
				ContainerName: row.ContainerName, SourceIP: row.SourceIP, SourcePort: row.SourcePort,
				DestIP: row.DestIP, DestPort: row.DestPort, Protocol: row.Protocol, State: row.State,
				BytesSent: row.BytesSent, BytesRecv: row.BytesRecv, ObservedAt: now, CreatedAt: now,
				RuntimeSource: runtimeSource, Bucket5m: bucket,
			}
		}
		normalized = dedupePodNetworkConnectionsForUpsert(normalized)
		newEvents, err := buildNetworkQueueSpikeEventsScoped(db, req.ClusterID, req.PodUID, req.Namespace, now, normalized)
		if err != nil {
			log.Printf("[PodDetail] scoped network anomaly detection skipped for %s/%s: %v", req.ClusterID, req.PodUID, err)
			newEvents = nil
		}
		if len(normalized) > 0 {
			upsert := clause.OnConflict{
				Columns: []clause.Column{{Name:"cluster_id"},{Name:"pod_uid"},{Name:"namespace"},{Name:"container_name"},{Name:"source_ip"},{Name:"source_port"},{Name:"dest_ip"},{Name:"dest_port"},{Name:"protocol"},{Name:"state"},{Name:"bucket_5m"}},
				DoUpdates: clause.Assignments(map[string]interface{}{
					"observed_at": gorm.Expr("GREATEST(pod_network_connections.observed_at, EXCLUDED.observed_at)"),
					"bytes_sent": gorm.Expr("CASE WHEN EXCLUDED.observed_at >= pod_network_connections.observed_at THEN EXCLUDED.bytes_sent ELSE pod_network_connections.bytes_sent END"),
					"bytes_recv": gorm.Expr("CASE WHEN EXCLUDED.observed_at >= pod_network_connections.observed_at THEN EXCLUDED.bytes_recv ELSE pod_network_connections.bytes_recv END"),
					"runtime_source": gorm.Expr("CASE WHEN EXCLUDED.observed_at >= pod_network_connections.observed_at THEN EXCLUDED.runtime_source ELSE pod_network_connections.runtime_source END"),
				}),
			}
			session := db.Session(&gorm.Session{PrepareStmt: false})
			if err := dbIngestWithRetry(session, func(tx *gorm.DB) error {
				return tx.Transaction(func(tx2 *gorm.DB) error {
					if err := tx2.Clauses(upsert).CreateInBatches(normalized, 50).Error; err != nil {
						return err
					}
					if len(newEvents) > 0 {
						return tx2.Clauses(clause.OnConflict{DoNothing:true}).CreateInBatches(newEvents, 100).Error
					}
					return nil
				})
			}); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			BroadcastPodDetailUpdate(req.PodUID, "network")
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "count": len(normalized)})
	}
}

func buildNetworkQueueSpikeEventsScoped(db *gorm.DB, clusterID, podUID, namespace string, observedAt time.Time, current []models.PodNetworkConnection) ([]models.RuntimeEvent, error) {
	if clusterID == "" || podUID == "" || len(current) == 0 {
		return nil, nil
	}
	windowMin := envIntDefault("POD_DETAIL_NET_SPIKE_WINDOW_MINUTES", 30)
	minSamples := envIntDefault("POD_DETAIL_NET_SPIKE_MIN_SAMPLES", 5)
	multiplier := envFloatDefault("POD_DETAIL_NET_SPIKE_MULTIPLIER", 4.0)
	minQueueBytes := int64(envIntDefault("POD_DETAIL_NET_SPIKE_MIN_QUEUE_BYTES", 4096))
	cooldownMinutes := envIntDefault("POD_DETAIL_NET_SPIKE_COOLDOWN_MINUTES", 10)
	type baselineRow struct { ContainerName, DestIP string; DestPort int; Protocol string; AvgQueue float64; Samples int64 }
	var baseline []baselineRow
	fromTime := observedAt.Add(-time.Duration(windowMin) * time.Minute)
	if err := db.Table("pod_network_connections").
		Select("container_name, dest_ip, dest_port, protocol, AVG(bytes_sent + bytes_recv) AS avg_queue, COUNT(*) AS samples").
		Where("cluster_id = ? AND pod_uid = ? AND observed_at >= ? AND observed_at < ?", clusterID, podUID, fromTime, observedAt).
		Group("container_name, dest_ip, dest_port, protocol").Scan(&baseline).Error; err != nil {
		return nil, err
	}
	baseMap := make(map[string]baselineRow, len(baseline))
	for i := range baseline { baseMap[networkAnomalyKey(baseline[i].ContainerName, baseline[i].DestIP, baseline[i].DestPort, baseline[i].Protocol)] = baseline[i] }
	events := make([]models.RuntimeEvent, 0, len(current))
	emitted := map[string]struct{}{}
	for i := range current {
		row := current[i]
		key := networkAnomalyKey(row.ContainerName, row.DestIP, row.DestPort, row.Protocol)
		if _, exists := emitted[key]; exists { continue }
		base, ok := baseMap[key]
		if !ok || base.Samples < int64(minSamples) { continue }
		currentQueue := row.BytesSent + row.BytesRecv
		if currentQueue < minQueueBytes || base.AvgQueue <= 0 || float64(currentQueue) < base.AvgQueue*multiplier { continue }
		ratio := float64(currentQueue) / base.AvgQueue
		suppressed, err := isNetworkSpikeSuppressedScoped(db, clusterID, podUID, key, observedAt, cooldownMinutes)
		if err != nil { return nil, err }
		if suppressed { continue }
		target := "key="+key+" dst="+strings.TrimSpace(row.DestIP)+":"+strconv.Itoa(row.DestPort)+" proto="+strings.ToLower(strings.TrimSpace(row.Protocol))+" q="+strconv.FormatInt(currentQueue,10)+" avg="+strconv.FormatFloat(base.AvgQueue,'f',0,64)+" ratio="+strconv.FormatFloat(ratio,'f',2,64)+" samples="+strconv.FormatInt(base.Samples,10)
		if len(target) > 500 { target = target[:500] }
		events = append(events, models.RuntimeEvent{ClusterID:clusterID, PodUID:podUID, Namespace:namespace, Syscall:"connect", TargetPath:target, Capability:"NETWORK_TXRX_QUEUE_SPIKE", CreatedAt:observedAt, Confidence:0.75})
		emitted[key] = struct{}{}
		if len(events) >= 100 { break }
	}
	return events, nil
}

func isNetworkSpikeSuppressedScoped(db *gorm.DB, clusterID, podUID, key string, observedAt time.Time, cooldownMinutes int) (bool, error) {
	if cooldownMinutes <= 0 { return false, nil }
	from := observedAt.Add(-time.Duration(cooldownMinutes) * time.Minute)
	var count int64
	err := db.Model(&models.RuntimeEvent{}).
		Where("cluster_id = ? AND pod_uid = ? AND capability = ? AND created_at >= ? AND target_path LIKE ?", clusterID, podUID, "NETWORK_TXRX_QUEUE_SPIKE", from, "key="+key+" %").Count(&count).Error
	return count > 0, err
}
