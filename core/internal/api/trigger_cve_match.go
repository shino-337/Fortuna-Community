package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TriggerCVEMatchRequest is the body for POST /api/v1/internal/trigger-cve-match
type TriggerCVEMatchRequest struct {
	SBOMID uint `json:"sbom_id"`
}

// TriggerCVEMatch republishes sbom.created for an existing SBOM so the CVE matcher worker runs again.
// Use after restoring soft-deleted components (e.g. UPDATE sbom_components SET deleted_at = NULL).
// Requires admin. NATS client must be non-nil.
// PublishSBOMCreatedFunc publishes to subject fortuna.sbom.created (e.g. NATS JetStream).
type PublishSBOMCreatedFunc func(subject string, data []byte) error

func TriggerCVEMatch(db *gorm.DB, publishSBOMCreated PublishSBOMCreatedFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if publishSBOMCreated == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "NATS not available"})
			return
		}
		var req TriggerCVEMatchRequest
		if err := c.ShouldBindJSON(&req); err != nil || req.SBOMID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "body must be {\"sbom_id\": <number>}"})
			return
		}
		var sbom models.SBOM
		if err := db.Where("id = ? AND deleted_at IS NULL", req.SBOMID).First(&sbom).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("SBOM id=%d not found", req.SBOMID)})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		var pod models.Pod
		clusterID := "unknown"
		if err := db.Where("uid = ? AND deleted_at IS NULL", sbom.PodUID).First(&pod).Error; err == nil {
			clusterID = pod.ClusterID
		}
		podName := sbom.PodUID
		podNamespace := ""
		if pod.ID != 0 {
			podName = pod.Name
			podNamespace = pod.Namespace
		}
		containerImage := fmt.Sprintf("%s:%s", sbom.ImageName, sbom.ImageTag)
		containerName := sbom.ContainerName
		event := map[string]interface{}{
			"type":            "sbom.created",
			"timestamp":      time.Now().Unix(),
			"cluster_id":     clusterID,
			"pod_uid":        sbom.PodUID,
			"pod_name":       podName,
			"pod_namespace":  podNamespace,
			"container_name": containerName,
			"container_image": containerImage,
			"sbom_id":        sbom.ID,
			"image_digest":   sbom.ImageDigest,
		}
		eventJSON, err := json.Marshal(event)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "marshal event: " + err.Error()})
			return
		}
		if err := publishSBOMCreated("fortuna.sbom.created", eventJSON); err != nil {
			log.Printf("[TriggerCVEMatch] Publish failed for sbom_id=%d: %v", req.SBOMID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "publish failed: " + err.Error()})
			return
		}
		log.Printf("[TriggerCVEMatch] Published sbom.created for sbom_id=%d (pod_uid=%s)", sbom.ID, sbom.PodUID)
		LogPlatformSecurityAudit(db, c, "internal_cve_trigger", "sbom", fmt.Sprintf("%d", sbom.ID), map[string]interface{}{
			"pod_uid": sbom.PodUID,
		})
		c.JSON(http.StatusOK, gin.H{"ok": true, "sbom_id": sbom.ID, "message": "sbom.created event published; CVE matcher will run shortly"})
	}
}
