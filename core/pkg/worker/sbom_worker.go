package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/messaging"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/sbom"
	"github.com/nats-io/nats.go"
	"gorm.io/gorm"
)

// SBOMWorker implements: Kubernetes Event -> Digest Resolution -> SBOM Cache Check -> SBOM Generation -> Persist -> SBOM_CREATED event.
type SBOMWorker struct {
	db        *gorm.DB
	publisher *messaging.Publisher
	service   *sbom.Service
	logger    *log.Logger
}

func NewSBOMWorker(js nats.JetStreamContext, db *gorm.DB, natsClient *messaging.NATSClient) *SBOMWorker {
	return &SBOMWorker{
		db:        db,
		publisher: messaging.NewPublisher(natsClient),
		service:   sbom.NewService(db),
		logger:    log.New(log.Writer(), "[SBOMWorker] ", log.LstdFlags),
	}
}

func (w *SBOMWorker) Name() string { return "sbom" }

func (w *SBOMWorker) Subject() string {
	// Only pods drive SBOM generation.
	return "fortuna.normalized.pods"
}

func (w *SBOMWorker) Process(ctx context.Context, msg *nats.Msg) error {
	enabled := strings.ToLower(strings.TrimSpace(getEnv("FORTUNA_SBOM_ENABLED", "true")))
	if enabled == "false" {
		return nil
	}

	var normalized map[string]interface{}
	if err := json.Unmarshal(msg.Data, &normalized); err != nil {
		return fmt.Errorf("unmarshal normalized pod: %w", err)
	}

	// DEBUG: Log all incoming messages to diagnose filtering issues
	podUID, _ := normalized["uid"].(string)
	podName, _ := normalized["name"].(string)
	podNS, _ := normalized["namespace"].(string)
	eventType, _ := normalized["event_type"].(string)
	w.logger.Printf("📥 Received: pod=%s/%s uid=%s eventType=%s", podNS, podName, podUID, eventType)

	if strings.EqualFold(eventType, "Deleted") {
		w.logger.Printf("⏭️  Skipping Deleted event for pod %s/%s", podNS, podName)
		return nil
	}

	clusterID, _ := normalized["cluster_id"].(string)
	if clusterID == "" {
		if v := os.Getenv("DEFAULT_CLUSTER_ID"); v != "" {
			clusterID = v
		} else {
			clusterID = "unknown"
		}
	}

	rawJSON, _ := normalized["raw_json"].(string)
	rawJSON = strings.TrimSpace(rawJSON)
	if rawJSON == "" {
		return nil
	}

	// Parse raw pod JSON to extract containers
	var podObj map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &podObj); err != nil {
		return fmt.Errorf("parse raw pod json: %w", err)
	}

	// Only process running pods (reduce duplicate work on pending/terminating updates)
	if status, ok := podObj["status"].(map[string]interface{}); ok {
		if phase, _ := status["phase"].(string); phase != "" && phase != "Running" {
			w.logger.Printf("⏭️  Skipping pod %s/%s: phase=%s (not Running)", podNS, podName, phase)
			return nil
		}
	}

	spec, _ := podObj["spec"].(map[string]interface{})
	if spec == nil {
		return nil
	}

	containers := extractContainers(spec)
	if len(containers) == 0 {
		return nil
	}

	createdEvents := 0
	for _, c := range containers {
		imageRef := strings.TrimSpace(c.image)
		if imageRef == "" {
			continue
		}

		// De-dup: if this pod/container/image already linked to an SBOM, skip event
		var existing models.PodImageScan
		if err := w.db.WithContext(ctx).
			Where("pod_uid = ? AND container_name = ? AND container_image = ? AND sbom_id IS NOT NULL AND deleted_at IS NULL",
				podUID, c.name, imageRef).
			First(&existing).Error; err == nil {
			continue
		}

		// EnsureSBOM is deprecated - SBOM is already stored by handler
		// Just verify it exists
		var sbomModel models.SBOM
		if err := w.db.WithContext(ctx).Where("image_digest = ? AND deleted_at IS NULL", imageRef).First(&sbomModel).Error; err != nil {
			w.logger.Printf("⚠️  SBOM not found for %s (pod %s/%s): %v", imageRef, podNS, podName, err)
			continue
		}

		if err := w.service.UpsertPodImageScan(ctx, clusterID, podUID, podName, podNS, c.name, imageRef, sbomModel.ID); err != nil {
			w.logger.Printf("⚠️  UpsertPodImageScan failed for pod %s/%s container %s: %v", podNS, podName, c.name, err)
		}

		ev := sbom.SBOMCreatedEvent{
			Type:           "sbom.created",
			Timestamp:      time.Now().Unix(),
			ClusterID:      clusterID,
			PodUID:         podUID,
			PodName:        podName,
			PodNamespace:   podNS,
			ContainerName:  c.name,
			ContainerImage: imageRef,
			SBOMID:         sbomModel.ID,
			ImageDigest:    sbomModel.ImageDigest,
		}

		if err := w.publisher.PublishSBOMCreated(ev); err != nil {
			w.logger.Printf("⚠️  Failed to publish sbom.created for %s: %v", imageRef, err)
			continue
		}
		createdEvents++
	}

	if createdEvents > 0 {
		w.logger.Printf("✅ Published %d sbom.created events for pod %s/%s", createdEvents, podNS, podName)
	}
	return nil
}

type containerRef struct {
	name  string
	image string
}

func extractContainers(spec map[string]interface{}) []containerRef {
	out := make([]containerRef, 0)

	// spec.containers
	if arr, ok := spec["containers"].([]interface{}); ok {
		out = append(out, parseContainerArray(arr)...)
	}
	// spec.initContainers (optional)
	if arr, ok := spec["initContainers"].([]interface{}); ok {
		out = append(out, parseContainerArray(arr)...)
	}
	return out
}

func parseContainerArray(arr []interface{}) []containerRef {
	out := make([]containerRef, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := m["name"].(string)
		image, _ := m["image"].(string)
		out = append(out, containerRef{name: name, image: image})
	}
	return out
}

func getEnv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
