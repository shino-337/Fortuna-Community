package api

import (
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
			if err := dbIngestWithRetry(db, func(tx *gorm.DB) error {
				return tx.Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(req.Metrics, 50).Error
			}); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			BroadcastPodDetailUpdate(req.PodUID, "metrics")
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "count": len(req.Metrics)})
	}
}
func processDiffKey(container string, pid int) string {
	return container + "|" + strconv.Itoa(pid)
}

func normalizePodNetworkForUpsert(p *models.PodNetworkConnection) {
	// Match unique index / migration 115 (use empty string, not NULL)
	p.SourceIP = strings.TrimSpace(p.SourceIP)
	p.DestIP = strings.TrimSpace(p.DestIP)
	if strings.TrimSpace(p.Protocol) == "" {
		p.Protocol = "tcp"
	} else {
		p.Protocol = strings.ToLower(strings.TrimSpace(p.Protocol))
	}
	p.State = strings.TrimSpace(p.State)
}

// podNetworkUpsertDedupeKey matches the ON CONFLICT columns on pod_network_connections (migration 115).
func podNetworkUpsertDedupeKey(r models.PodNetworkConnection) string {
	return strings.Join([]string{
		r.ClusterID, r.PodUID, r.Namespace, r.ContainerName,
		r.SourceIP, strconv.Itoa(r.SourcePort), r.DestIP, strconv.Itoa(r.DestPort),
		r.Protocol, r.State, r.Bucket5m.UTC().Format(time.RFC3339Nano),
	}, "\x1f")
}

// dedupePodNetworkConnectionsForUpsert merges rows that would hit the same unique tuple in one INSERT.
// PostgreSQL rejects ON CONFLICT DO UPDATE when two VALUES rows target the same existing row (SQLSTATE 21000).
func dedupePodNetworkConnectionsForUpsert(rows []models.PodNetworkConnection) []models.PodNetworkConnection {
	if len(rows) < 2 {
		return rows
	}
	byKey := make(map[string]models.PodNetworkConnection, len(rows))
	for i := range rows {
		r := rows[i]
		k := podNetworkUpsertDedupeKey(r)
		if ex, ok := byKey[k]; ok {
			if r.BytesSent > ex.BytesSent {
				ex.BytesSent = r.BytesSent
			}
			if r.BytesRecv > ex.BytesRecv {
				ex.BytesRecv = r.BytesRecv
			}
			byKey[k] = ex
		} else {
			byKey[k] = r
		}
	}
	out := make([]models.PodNetworkConnection, 0, len(byKey))
	for _, v := range byKey {
		out = append(out, v)
	}
	return out
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
