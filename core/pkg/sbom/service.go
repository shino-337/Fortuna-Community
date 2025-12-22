package sbom

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/ksam/core/pkg/models"
	"github.com/ksam/core/pkg/sbom/extractor"
	"github.com/ksam/core/pkg/sbom/normalizer"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Service implements cache-first SBOM generation/persistence.
// It is intentionally separated from CVE matching and insights generation.
type Service struct {
	extractor  *extractor.Extractor
	normalizer *normalizer.Normalizer
	db         *gorm.DB
	logger     *log.Logger
}

func NewService(db *gorm.DB) *Service {
	return &Service{
		extractor:  extractor.NewExtractor(),
		normalizer: normalizer.NewNormalizer(),
		db:         db,
		logger:     log.New(log.Writer(), "[SBOMService] ", log.LstdFlags),
	}
}

// EnsureSBOM ensures an SBOM exists for the given imageRef.
// Behavior:
// - Resolve digest (no layer extraction)
// - Check SBOM cache by digest
// - Only if missing: extract+normalize+persist SBOM + components
func (s *Service) EnsureSBOM(ctx context.Context, imageRef string) (*models.SBOM, error) {
	imageName, imageTag := parseImageRef(imageRef)

	digest, err := s.extractor.ResolveDigest(ctx, imageRef)
	if err != nil {
		// Fall back to full extraction (which will also attempt digest)
		s.logger.Printf("⚠️  ResolveDigest failed for %s: %v (falling back to full extraction)", imageRef, err)
		digest = ""
	}

	// Cache-first check
	if digest != "" {
		var existing models.SBOM
		if err := s.db.WithContext(ctx).
			Where("image_digest = ? AND deleted_at IS NULL", digest).
			First(&existing).Error; err == nil {
			// Update cache metadata and image tag if it changed
			updates := map[string]interface{}{
				"use_count":    gorm.Expr("use_count + ?", 1),
				"last_used_at": time.Now(),
			}
			// Fix: Update image_tag if it changed (same digest, different tag)
			if existing.ImageTag != imageTag {
				updates["image_tag"] = imageTag
				updates["image_name"] = imageName
				s.logger.Printf("🔄 SBOM cache hit with tag update: digest=%s id=%d old_tag=%s new_tag=%s",
					digest, existing.ID, existing.ImageTag, imageTag)
			} else {
				s.logger.Printf("✅ SBOM cache hit: digest=%s id=%d use_count++", digest, existing.ID)
			}
			_ = s.db.WithContext(ctx).Model(&models.SBOM{}).
				Where("id = ?", existing.ID).
				Updates(updates).Error

			// Return updated model
			existing.ImageTag = imageTag
			existing.ImageName = imageName
			return &existing, nil
		}
	}

	// Missing: generate SBOM
	raw, err := s.extractor.ExtractSBOM(ctx, imageRef)
	if err != nil {
		return nil, fmt.Errorf("SBOM extraction failed: %w", err)
	}

	// Prefer digest from resolver; fall back to extracted digest
	if digest == "" {
		digest = raw.ImageDigest
	}
	if digest == "" {
		// Last resort: use image name:tag as a non-immutable key
		digest = fmt.Sprintf("%s:%s", imageName, imageTag)
		s.logger.Printf("⚠️  Image digest unavailable for %s, using fallback digest=%s", imageRef, digest)
	}

	sbomModel, err := s.normalizer.Normalize(raw)
	if err != nil {
		return nil, fmt.Errorf("SBOM normalization failed: %w", err)
	}
	sbomModel.ImageName = imageName
	sbomModel.ImageTag = imageTag
	sbomModel.ImageDigest = digest
	sbomModel.LastUsedAt = time.Now()

	// Upsert SBOM row (digest is unique)
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "image_digest"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"last_used_at": gorm.Expr("CURRENT_TIMESTAMP"),
			"use_count":    gorm.Expr("sboms.use_count + 1"),
			"updated_at":   gorm.Expr("CURRENT_TIMESTAMP"),
		}),
	}).Create(sbomModel).Error; err != nil {
		return nil, fmt.Errorf("failed to upsert SBOM: %w", err)
	}

	// Reload to get ID (especially on conflict/update)
	var persisted models.SBOM
	if err := s.db.WithContext(ctx).
		Where("image_digest = ? AND deleted_at IS NULL", digest).
		First(&persisted).Error; err != nil {
		return nil, fmt.Errorf("failed to reload persisted SBOM: %w", err)
	}

	// Build components list (SBOMComponent rows)
	components := make([]models.SBOMComponent, 0, len(raw.Packages))
	for _, pkg := range raw.Packages {
		purl := generatePURL(pkg, raw.OS)
		purl = strings.TrimSpace(purl)
		if purl == "" {
			continue
		}
		components = append(components, models.SBOMComponent{
			SBOMID:           persisted.ID,
			ComponentType:    mapComponentType(pkg.Type),
			ComponentName:    pkg.Name,
			ComponentVersion: pkg.Version,
			PURL:             purl,
			// licenses is JSONB in Postgres; store valid empty JSON by default
			Licenses: models.ToJSONBString([]string{}),
		})
	}

	// Insert components with dedup (requires unique index on (sbom_id, purl))
	const batchSize = 500
	for i := 0; i < len(components); i += batchSize {
		end := i + batchSize
		if end > len(components) {
			end = len(components)
		}
		batch := components[i:end]
		if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "sbom_id"}, {Name: "purl"}},
			DoNothing: true,
		}).Create(&batch).Error; err != nil {
			return nil, fmt.Errorf("failed to insert sbom components: %w", err)
		}
	}

	s.logger.Printf("✅ SBOM generated: digest=%s id=%d components=%d", digest, persisted.ID, len(components))
	return &persisted, nil
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

// Helpers (kept here to avoid importing pipeline.go)

func generatePURL(pkg extractor.Package, os extractor.OSInfo) string {
	switch pkg.Type {
	case "deb":
		return fmt.Sprintf("pkg:deb/%s/%s@%s", os.Name, pkg.Name, pkg.Version)
	case "apk":
		return fmt.Sprintf("pkg:apk/alpine/%s@%s", pkg.Name, pkg.Version)
	case "rpm":
		return fmt.Sprintf("pkg:rpm/%s/%s@%s", os.Name, pkg.Name, pkg.Version)
	case "npm":
		return fmt.Sprintf("pkg:npm/%s@%s", pkg.Name, pkg.Version)
	case "pypi":
		return fmt.Sprintf("pkg:pypi/%s@%s", pkg.Name, pkg.Version)
	case "go":
		return fmt.Sprintf("pkg:golang/%s@%s", pkg.Name, pkg.Version)
	default:
		return fmt.Sprintf("pkg:generic/%s@%s", pkg.Name, pkg.Version)
	}
}

func mapComponentType(pkgType string) string {
	switch pkgType {
	case "deb", "rpm", "apk":
		return "library"
	case "npm", "pypi", "go":
		return "library"
	default:
		return "library"
	}
}
