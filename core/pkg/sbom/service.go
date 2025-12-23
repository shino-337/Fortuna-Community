package sbom

import (
	"context"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/models"
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

// UpsertPodImageScan links a pod container to an SBOM (digest-based scan cache).
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

	// Unique key is (pod_uid, container_name)
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "pod_uid"}, {Name: "container_name"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"pod_name":        scan.PodName,
			"pod_namespace":   scan.PodNamespace,
			"cluster_id":      scan.ClusterID,
			"container_image": scan.ContainerImage,
			"image_name":      scan.ImageName,
			"image_tag":       scan.ImageTag,
			"sbom_id":         scan.SBOMID,
			"updated_at":      gorm.Expr("CURRENT_TIMESTAMP"),
			"deleted_at":      nil,
		}),
	}).Create(&scan).Error
}

// Helper
func parseImageRef(imageRef string) (string, string) {
	parts := strings.SplitN(imageRef, ":", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return imageRef, "latest"
}
