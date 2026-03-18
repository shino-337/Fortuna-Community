package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/fortuna/core/internal/contextkeys"
	"github.com/fortuna/core/pkg/models"
)

// SBOMRepository provides a single guarded entrypoint for SBOM writes.
type SBOMRepository struct {
	db *gorm.DB
}

func NewSBOMRepository(db *gorm.DB) *SBOMRepository {
	return &SBOMRepository{db: db}
}

// assertMutable enforces SBOM immutability once finalized, unless the
// SBOM mutation flag is explicitly allowed on the context. It also
// validates that the status field is in a known set of values.
func (r *SBOMRepository) assertMutable(ctx context.Context, sbom *models.SBOM) error {
	if sbom == nil {
		return errors.New("nil sbom")
	}

	switch sbom.Status {
	case "", "pending":
		// pending or empty (legacy rows) are always mutable
		return nil
	case "finalized":
		if !contextkeys.IsSBOMMutationAllowed(ctx) {
			return fmt.Errorf("sbom %d is finalized and immutable", sbom.ID)
		}
		return nil
	default:
		return fmt.Errorf("invalid sbom status: %s", sbom.Status)
	}
}

// UpsertSBOMWithComponents upserts an SBOM (keyed by pod_uid) and replaces its components.
// It is the single write entrypoint for the SBOM ingest path.
func (r *SBOMRepository) UpsertSBOMWithComponents(
	ctx context.Context,
	sbom *models.SBOM,
	components []*models.SBOMComponent,
) (*models.SBOM, bool, error) {
	if sbom == nil {
		return nil, false, errors.New("sbom is required")
	}

	tx := r.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var existing models.SBOM
	isNewSBOM := false

	// Row-level lock to serialize concurrent writers for the same pod_uid.
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("pod_uid = ? AND deleted_at IS NULL", sbom.PodUID).
		First(&existing).Error
	switch {
	case err == nil:
		// Existing SBOM for this pod: enforce immutability guard.
		if err := r.assertMutable(ctx, &existing); err != nil {
			tx.Rollback()
			return nil, false, err
		}

		// Reuse ID and bump version.
		sbom.ID = existing.ID
		sbom.UseCount = existing.UseCount + 1
		if sbom.LastUsedAt.IsZero() {
			sbom.LastUsedAt = time.Now()
		}

		nextVersion := existing.Version
		if nextVersion <= 0 {
			nextVersion = 1
		}
		nextVersion++

		if err := tx.Model(&existing).Updates(map[string]interface{}{
			"image_name":     sbom.ImageName,
			"image_tag":      sbom.ImageTag,
			"image_digest":   sbom.ImageDigest,
			"pod_name":       sbom.PodName,
			"namespace":      sbom.Namespace,
			"container_name": sbom.ContainerName,
			"package_count":  sbom.PackageCount,
			"generated_at":   sbom.GeneratedAt,
			"last_used_at":   sbom.LastUsedAt,
			"use_count":      sbom.UseCount,
			"sbom_source":    sbom.SbomSource,
			"confidence":     sbom.Confidence,
			"status":         "finalized",
			"version":        nextVersion,
		}).Error; err != nil {
			tx.Rollback()
			return nil, false, fmt.Errorf("update existing sbom: %w", err)
		}

		// Replace components for this SBOM (delete old, insert new from request).
		if err := tx.Where("sbom_id = ?", sbom.ID).Delete(&models.SBOMComponent{}).Error; err != nil {
			tx.Rollback()
			return nil, false, fmt.Errorf("delete old components: %w", err)
		}

	case errors.Is(err, gorm.ErrRecordNotFound):
		// New pod: create fresh SBOM row, mark as finalized v1 by default.
		if sbom.Status == "" {
			sbom.Status = "finalized"
		}
		if sbom.Version == 0 {
			sbom.Version = 1
		}
		if sbom.LastUsedAt.IsZero() {
			sbom.LastUsedAt = time.Now()
		}
		if err := tx.Create(sbom).Error; err != nil {
			tx.Rollback()
			return nil, false, fmt.Errorf("insert sbom: %w", err)
		}
		isNewSBOM = true

	default:
		tx.Rollback()
		return nil, false, fmt.Errorf("query existing sbom: %w", err)
	}

	// Insert components in batches to avoid slow large INSERTs.
	const componentBatchSize = 200
	if len(components) > 0 {
		// Ensure SBOMID is set on all components.
		for _, c := range components {
			if c != nil {
				c.SBOMID = sbom.ID
			}
		}

		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "sbom_id"}, {Name: "purl"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"component_type",
				"component_name",
				"component_version",
				"licenses",
				"source",
				"description",
				"homepage",
				"maintainer",
				"updated_at",
			}),
		}).CreateInBatches(components, componentBatchSize).Error; err != nil {
			tx.Rollback()
			return nil, false, fmt.Errorf("insert components: %w", err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, false, fmt.Errorf("commit sbom upsert: %w", err)
	}

	return sbom, isNewSBOM, nil
}

// EnsureMatchRun tries to create a match-run record for the given SBOM snapshot
// and mirror version. It returns (true, nil) on first run, and (false, nil) if
// a run with the same (sbom_id, version, mirror_version) already exists.
func (r *SBOMRepository) EnsureMatchRun(
	ctx context.Context,
	sbomID uint,
	version int,
	mirrorVersion string,
) (bool, error) {
	if sbomID == 0 || version <= 0 || mirrorVersion == "" {
		return false, fmt.Errorf("invalid match run key: sbom_id=%d version=%d mirror_version=%q", sbomID, version, mirrorVersion)
	}

	now := time.Now()

	// First, check if a run already exists for this key.
	var existing models.SBOMMatchRun
	err := r.db.WithContext(ctx).First(&existing,
		"sbom_id = ? AND version = ? AND mirror_version = ?",
		sbomID, version, mirrorVersion,
	).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, fmt.Errorf("query sbom_match_run: %w", err)
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// No existing run: create a fresh one. Concurrent workers may both miss SELECT
		// and try INSERT; one wins, the other gets duplicate key → treat as skip.
		run := &models.SBOMMatchRun{
			SBOMID:        sbomID,
			Version:       version,
			MirrorVersion: mirrorVersion,
			Status:        "running",
			CreatedAt:     now,
		}
		if errCreate := r.db.WithContext(ctx).Create(run).Error; errCreate != nil {
			if isDuplicateKey(errCreate) {
				return false, nil
			}
			return false, fmt.Errorf("create sbom_match_run: %w", errCreate)
		}
		return true, nil
	}

	// Existing run found. Implement simple crash-recovery:
	// - If it's still "running" and recent, treat as duplicate (another worker).
	// - If it's old or finished, allow re-run by bumping to "running" and updating CreatedAt.
	// staleAfter must be >= max expected match duration to avoid reclaim while job still running.
	const staleAfter = 15 * time.Minute
	if existing.Status == "running" && now.Sub(existing.CreatedAt) < staleAfter {
		return false, nil
	}

	if err := r.db.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
		"status":     "running",
		"created_at": now,
	}).Error; err != nil {
		return false, fmt.Errorf("update sbom_match_run: %w", err)
	}
	return true, nil
}

// isDuplicateKey returns true if err indicates a unique/primary key violation
// (PostgreSQL 23505, or driver message), so callers can treat as idempotent skip.
// Uses specific phrases to avoid false positives on unrelated error text.
func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key value") ||
		strings.Contains(msg, "violates unique constraint") ||
		strings.Contains(msg, "unique constraint failed") || // SQLite
		strings.Contains(msg, "23505")
}

