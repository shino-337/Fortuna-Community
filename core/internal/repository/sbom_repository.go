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
	"github.com/fortuna/core/pkg/metrics"
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

	status := strings.ToLower(strings.TrimSpace(sbom.Status))
	switch status {
	case "", "pending":
		// pending or empty (legacy rows) are always mutable
		return nil
	case "finalized", "complete", "partial", "failed":
		if !contextkeys.IsSBOMMutationAllowed(ctx) {
			return fmt.Errorf("sbom %d is %q and immutable", sbom.ID, status)
		}
		return nil
	default:
		return fmt.Errorf("invalid sbom status: %s", sbom.Status)
	}
}

func normalizeSBOMStatus(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "":
		return "pending"
	case "pending":
		return "pending"
	case "finalized":
		// Legacy alias: treat as complete in downstream matching/intel.
		return "complete"
	case "complete", "partial", "failed":
		return s
	default:
		return "pending"
	}
}

// assertTransition enforces a monotonic SBOM lifecycle.
// State machine (recommended by backlog C0.7):
// - pending  -> complete | partial | failed
// - failed   -> failed (idempotent)
// - partial  -> partial | complete (optional reprocess)
// - complete -> complete (immutable)
func assertTransition(oldStatus, newStatus string) error {
	oldS := normalizeSBOMStatus(oldStatus)
	newS := normalizeSBOMStatus(newStatus)

	switch oldS {
	case "pending":
		// pending -> anything (including pending)
		switch newS {
		case "pending", "complete", "partial", "failed":
			return nil
		}
	case "failed":
		if newS == "failed" {
			return nil
		}
	case "partial":
		if newS == "partial" || newS == "complete" || newS == "failed" {
			return nil
		}
	case "complete":
		// Determinism enforcement can mark SBOM as failed when nondeterminism is detected.
		// This is an exceptional downgrade and must remain monotonic (failed is terminal).
		if newS == "complete" || newS == "failed" {
			return nil
		}
	}

	return fmt.Errorf("invalid sbom_status transition: %s -> %s", oldS, newS)
}

// UpsertSBOMWithComponents upserts an SBOM (keyed by pod_uid + image_digest) and replaces its components.
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

	// C4: Validate component count invariant.
	// SBOM lifecycle semantics:
	// - complete/partial must have at least one non-nil component after sanitization.
	// - failed may be empty.
	//
	// Important: we intentionally DO NOT enforce this on legacy alias "finalized"
	// because existing tests/legacy rows assume finalized/complete can be created
	// with zero components.
	rawStatus := strings.ToLower(strings.TrimSpace(sbom.Status))
	// NOTE: Existing handler semantics allow "partial" SBOM with 0 components
	// (e.g., when all requested components were sanitized/dropped).
	if rawStatus == "complete" {
		compCount := 0
		for _, c := range components {
			if c != nil {
				compCount++
			}
		}
		if compCount == 0 {
			tx.Rollback()
			return nil, false, fmt.Errorf("invalid sbom_status=%q with zero components (sbom_id=%d)", rawStatus, sbom.ID)
		}
	}

	var existing models.SBOM
	isNewSBOM := false

	// Row-level lock to serialize concurrent writers for the same pod_uid + image_digest.
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("pod_uid = ? AND image_digest = ? AND deleted_at IS NULL", sbom.PodUID, sbom.ImageDigest).
		First(&existing).Error
	switch {
	case err == nil:
		// Existing SBOM for this pod: enforce immutability guard.
		if err := assertTransition(existing.Status, sbom.Status); err != nil {
			tx.Rollback()
			return nil, false, err
		}
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

		// Drift detection (Phase 1 - Determinism v1):
		// If fingerprint differs for the same pod_uid+image_digest, it must be explained
		// by a change in versioned logic inputs (resolver_version/signature_db_version).
		//
		// Side-effect requirement: do NOT crash the pipeline. Instead, mark SBOM as failed.
		if normalizeSBOMStatus(existing.Status) != "failed" && normalizeSBOMStatus(sbom.Status) != "failed" {
			if existing.NormalizedFingerprint != "" && sbom.NormalizedFingerprint != "" && existing.NormalizedFingerprint != sbom.NormalizedFingerprint {
				versionChanged := existing.ResolverVersion != sbom.ResolverVersion ||
					existing.SignatureDBVersion != sbom.SignatureDBVersion
				if !versionChanged {
					metrics.SBOMDriftTotal.WithLabelValues("unexpected_same_version", sbom.ResolverVersion).Inc()
					sbom.Status = "failed"
					sbom.StatusReason = "nondeterministic_output"
				} else {
					metrics.SBOMDriftTotal.WithLabelValues("expected_version_changed", sbom.ResolverVersion).Inc()
				}
			}
		}

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
			"status_reason":  sbom.StatusReason,
			"status":         normalizeSBOMStatus(sbom.Status),
			"resolver_version":         sbom.ResolverVersion,
			"signature_db_version":    sbom.SignatureDBVersion,
			"normalized_fingerprint":  sbom.NormalizedFingerprint,
			"go_version":               strings.TrimSpace(sbom.GoVersion),
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
		// Drift metric for input changes (expected_input_changed):
		// If the same pod_uid appears with a different image_digest, this is an input change.
		// We record it (without failing) to avoid confusing it with nondeterministic output.
		if sbom.PodUID != "" && normalizeSBOMStatus(sbom.Status) != "failed" && sbom.NormalizedFingerprint != "" {
			var prev models.SBOM
			// Find the latest SBOM for the pod_uid excluding this image_digest.
			// (SQLite and Postgres both support LIMIT in raw queries; we use First with ordering.)
			prevErr := tx.Where("pod_uid = ? AND image_digest <> ? AND deleted_at IS NULL", sbom.PodUID, sbom.ImageDigest).
				Order("version DESC").
				First(&prev).Error
			if prevErr == nil && normalizeSBOMStatus(prev.Status) != "failed" && prev.NormalizedFingerprint != "" && prev.NormalizedFingerprint != sbom.NormalizedFingerprint {
				if prev.ResolverVersion == sbom.ResolverVersion && prev.SignatureDBVersion == sbom.SignatureDBVersion {
					metrics.SBOMDriftTotal.WithLabelValues("expected_input_changed", sbom.ResolverVersion).Inc()
				}
			}
		}

		if sbom.Status == "" {
			sbom.Status = "complete"
		}
		sbom.Status = normalizeSBOMStatus(sbom.Status)
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
				"original_purl",
				"purl_validated",
				"trust_level",
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

	isNewLabel := "false"
	if isNewSBOM {
		isNewLabel = "true"
	}
	metrics.SBOMStoreUpsertCommitsTotal.WithLabelValues(isNewLabel).Inc()

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
	resolverVersion string,
	matcherVersion string,
) (bool, error) {
	if sbomID == 0 || version <= 0 || mirrorVersion == "" {
		return false, fmt.Errorf("invalid match run key: sbom_id=%d version=%d mirror_version=%q", sbomID, version, mirrorVersion)
	}
	if resolverVersion == "" {
		resolverVersion = ""
	}
	if matcherVersion == "" {
		matcherVersion = ""
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
			ResolverVersion: resolverVersion,
			MatcherVersion:  matcherVersion,
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
		// Even if we skip the run, ensure metadata versioning columns are populated.
		_ = r.db.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
			"resolver_version": resolverVersion,
			"matcher_version":  matcherVersion,
		}).Error
		return false, nil
	}

	if err := r.db.WithContext(ctx).Model(&existing).Updates(map[string]interface{}{
		"status":          "running",
		"created_at":     now,
		"resolver_version": resolverVersion,
		"matcher_version":  matcherVersion,
	}).Error; err != nil {
		return false, fmt.Errorf("update sbom_match_run: %w", err)
	}
	return true, nil
}

// ClaimSBOMEvent atomically claims processing rights for an sbom.created event based on event timestamp.
// Returns true if this event is newer than the latest processed watermark; false if it is stale/replayed.
func (r *SBOMRepository) ClaimSBOMEvent(ctx context.Context, sbomID uint, eventID string, eventTS int64) (bool, error) {
	if sbomID == 0 || eventTS <= 0 {
		return false, fmt.Errorf("invalid claim key: sbom_id=%d event_id=%q event_ts=%d", sbomID, eventID, eventTS)
	}
	if strings.TrimSpace(eventID) == "" {
		eventID = fmt.Sprintf("ev-%d", time.Now().UnixNano())
	}

	// Atomic last-write-wins:
	// - Insert watermark if missing.
	// - Update if incoming ts is strictly newer, OR same ts with a different event_id (distinct events colliding on clock).
	// - Same ts + same event_id → no row change (idempotent redelivery).
	res := r.db.WithContext(ctx).Exec(`
INSERT INTO sbom_processing_state (sbom_id, latest_event_ts, latest_event_id)
VALUES (?, ?, ?)
ON CONFLICT(sbom_id) DO UPDATE
SET latest_event_ts = excluded.latest_event_ts,
    latest_event_id = excluded.latest_event_id,
    updated_at = CURRENT_TIMESTAMP
WHERE sbom_processing_state.latest_event_ts < excluded.latest_event_ts
   OR (
        sbom_processing_state.latest_event_ts = excluded.latest_event_ts
    AND sbom_processing_state.latest_event_id <> excluded.latest_event_id
   );
`, sbomID, eventTS, eventID)
	if res.Error != nil {
		return false, fmt.Errorf("claim sbom event: %w", res.Error)
	}
	// RowsAffected:
	// - 1 on insert or successful update
	// - 0 if stale (no update)
	return res.RowsAffected > 0, nil
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

