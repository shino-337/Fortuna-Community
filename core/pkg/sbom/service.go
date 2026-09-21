package sbom

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Service implements SBOM persistence and management.
// Note: SBOM extraction is now done by Agent, not Core.
type Service struct {
	db *gorm.DB
}

func NewService(db *gorm.DB) *Service {
	return &Service{
		db: db,
	}
}

// UpsertPodImageScan links a container only to its own workload SBOM observation.
func (s *Service) UpsertPodImageScan(
	ctx context.Context,
	clusterID string,
	podUID string,
	podName string,
	podNamespace string,
	containerName string,
	containerImage string,
	sbomID uint,
) error {
	if _, err := resourceidentity.New(clusterID, podUID); err != nil {
		return err
	}
	if strings.TrimSpace(containerName) == "" || sbomID == 0 {
		return fmt.Errorf("container and SBOM ID are required")
	}
	var owned models.SBOM
	if err := s.db.WithContext(ctx).Where("id = ? AND cluster_id = ? AND pod_uid = ? AND container_name = ?", sbomID, clusterID, podUID, containerName).First(&owned).Error; err != nil {
		return fmt.Errorf("SBOM workload association: %w", err)
	}
	imageName, imageTag := parseImageRef(containerImage)

	scan := models.PodImageScan{
		PodUID:         podUID,
		PodName:        podName,
		PodNamespace:   podNamespace,
		ClusterID:      clusterID,
		ContainerName:  containerName,
		ContainerImage: containerImage,
		ImageName:      imageName,
		ImageTag:       imageTag,
		SBOMID:         &sbomID,
		UpdatedAt:      time.Now(),
	}

	// Unique key is (cluster_id, pod_uid, container_name)
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "cluster_id"}, {Name: "pod_uid"}, {Name: "container_name"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"pod_name":        scan.PodName,
			"pod_namespace":   scan.PodNamespace,
			"container_image": scan.ContainerImage,
			"image_name":      scan.ImageName,
			"image_tag":       scan.ImageTag,
			"sbom_id":         scan.SBOMID,
			"updated_at":      gorm.Expr("CURRENT_TIMESTAMP"),
			"deleted_at":      nil,
		}),
	}).Create(&scan).Error
}

// parseImageRef parses image reference into image name + tag/digest suffix.
// Handles registry ports correctly (e.g. registry:5000/repo/image:1.2.3).
func parseImageRef(imageRef string) (string, string) {
	imageRef = strings.TrimSpace(imageRef)
	if imageRef == "" {
		return "", "latest"
	}
	// Digest reference: repo/name@sha256:...
	if at := strings.LastIndex(imageRef, "@"); at > 0 && at < len(imageRef)-1 {
		return imageRef[:at], imageRef[at+1:]
	}
	lastSlash := strings.LastIndex(imageRef, "/")
	lastColon := strings.LastIndex(imageRef, ":")
	// Tag is only valid when ":" appears after the last "/" (so registry port is ignored).
	if lastColon > lastSlash && lastColon < len(imageRef)-1 {
		return imageRef[:lastColon], imageRef[lastColon+1:]
	}
	// Handle trailing colon gracefully: "image:" => ("image","latest")
	if lastColon > lastSlash && lastColon == len(imageRef)-1 {
		return strings.TrimSuffix(imageRef, ":"), "latest"
	}
	return imageRef, "latest"
}
