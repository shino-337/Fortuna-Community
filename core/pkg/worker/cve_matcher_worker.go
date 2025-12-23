package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/cve/database"
	"github.com/fortuna/core/pkg/cve/matcher"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/riskengine"
	"github.com/fortuna/core/pkg/sbom"
	"github.com/nats-io/nats.go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CVEMatcherWorker implements: SBOM_CREATED -> CVE Matching -> Persist cve_matches -> Vulnerability Insights.
type CVEMatcherWorker struct {
	db             *gorm.DB
	dbManager      *database.Manager
	matcher        *matcher.Matcher
	insightMgr     *riskengine.InsightManager
	logger         *log.Logger
	onlySeverities map[string]bool
}

func NewCVEMatcherWorker(db *gorm.DB) *CVEMatcherWorker {
	dbMgr := database.NewPostgresManager(db)
	m := matcher.NewMatcher(dbMgr, db)
	return &CVEMatcherWorker{
		db:         db,
		dbManager:  dbMgr,
		matcher:    m,
		insightMgr: riskengine.NewInsightManager(db),
		logger:     log.New(log.Writer(), "[CVEMatcherWorker] ", log.LstdFlags),
		onlySeverities: map[string]bool{
			"CRITICAL": true,
			"HIGH":     true,
		},
	}
}

func (w *CVEMatcherWorker) Name() string { return "cve_matcher" }

func (w *CVEMatcherWorker) Subject() string { return "ksam.sbom.created" }

func (w *CVEMatcherWorker) Process(ctx context.Context, msg *nats.Msg) error {
	var ev sbom.SBOMCreatedEvent
	if err := json.Unmarshal(msg.Data, &ev); err != nil {
		return fmt.Errorf("unmarshal sbom.created: %w", err)
	}
	if ev.SBOMID == 0 {
		return nil
	}

	// Load SBOM (for component_count/logging)
	var sbomModel models.SBOM
	if err := w.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", ev.SBOMID).
		First(&sbomModel).Error; err != nil {
		// If SBOM doesn't exist (deleted or never created), skip silently to avoid retry loops
		w.logger.Printf("⚠️  SBOM id=%d not found (may have been deleted), skipping CVE matching", ev.SBOMID)
		return nil // Don't retry deleted SBOMs
	}

	// Match CVEs using postgres-backed manager (cves + package_vulnerabilities)
	matches, err := w.matcher.MatchSBOM(ctx, &sbomModel)
	if err != nil {
		return fmt.Errorf("match sbom id=%d: %w", sbomModel.ID, err)
	}
	if len(matches) == 0 {
		return nil
	}

	// Persist matches to cve_matches with dedup
	if err := w.persistMatches(ctx, matches); err != nil {
		return err
	}

	// Create insights (critical/high only)
	created := 0
	for _, m := range matches {
		if !w.onlySeverities[strings.ToUpper(m.Severity)] {
			continue
		}

		// Reload persisted match to get ID (using new schema: package_name instead of component_id)
		var persisted models.CVEMatch
		if err := w.db.WithContext(ctx).
			Where("sbom_id = ? AND package_name = ? AND cve_id = ? AND deleted_at IS NULL",
				m.SBOMID, m.PackageName, m.CVEID).
			First(&persisted).Error; err != nil {
			w.logger.Printf("⚠️  Cannot load persisted CVEMatch: %v", err)
			continue
		}

		// Load component by package name (using new schema)
		var component models.SBOMComponent
		if err := w.db.WithContext(ctx).
			Where("sbom_id = ? AND component_name = ? AND deleted_at IS NULL", m.SBOMID, m.PackageName).
			First(&component).Error; err != nil {
			w.logger.Printf("⚠️  Cannot load component for package %s: %v", m.PackageName, err)
			continue
		}

		insight := buildVulnInsightFromEvent(ev, &component, &persisted)
		// Use batch processing if multiple insights (future optimization)
		// For now, process individually but within transaction (handled by InsightManager)
		if err := w.insightMgr.CreateOrUpdateInsight(insight); err != nil {
			w.logger.Printf("⚠️  Failed to create/update insight for %s: %v", m.CVEID, err)
			continue
		}
		created++
	}

	if created > 0 {
		w.logger.Printf("✅ Created/updated %d vulnerability insights for pod %s/%s", created, ev.PodNamespace, ev.PodName)
	}
	return nil
}

func (w *CVEMatcherWorker) persistMatches(ctx context.Context, matches []*models.CVEMatch) error {
	now := time.Now()
	for _, m := range matches {
		if m.MatchedAt.IsZero() {
			m.MatchedAt = now
		}
		if m.MatchedBy == "" {
			m.MatchedBy = "fortuna-core-cve-matcher"
		}
	}

	// Batch insert with ON CONFLICT DO NOTHING (requires unique index: (sbom_id, cve_id, package_name))
	const batchSize = 500
	for i := 0; i < len(matches); i += batchSize {
		end := i + batchSize
		if end > len(matches) {
			end = len(matches)
		}
		batch := matches[i:end]
		if err := w.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "sbom_id"}, {Name: "component_id"}, {Name: "cve_id"}},
			DoNothing: true,
		}).Create(&batch).Error; err != nil {
			return fmt.Errorf("persist cve_matches batch: %w", err)
		}
	}
	return nil
}

func buildVulnInsightFromEvent(ev sbom.SBOMCreatedEvent, component *models.SBOMComponent, match *models.CVEMatch) *models.Insight {
	sevLower := strings.ToLower(strings.TrimSpace(match.Severity))
	if sevLower == "" {
		sevLower = "medium"
	}

	description := fmt.Sprintf(
		"Vulnerability %s (%s) detected in package %s@%s for pod %s/%s (container=%s, image=%s). Fixed version: %s",
		match.CVEID,
		strings.ToUpper(match.Severity),
		component.ComponentName,
		component.ComponentVersion,
		ev.PodNamespace,
		ev.PodName,
		ev.ContainerName,
		ev.ContainerImage,
		match.FixedVersion,
	)

	return &models.Insight{
		ResourceType:      "Pod",
		ResourceNamespace: ev.PodNamespace,
		ResourceName:      ev.PodName,
		ResourceUID:       ev.PodUID,
		InsightType:       "vulnerability",
		Severity:          sevLower,
		Title:             fmt.Sprintf("%s in %s", match.CVEID, component.ComponentName),
		Description:       description,
		Status:            "active",
		Recommendation:    fmt.Sprintf("Update image/package to a fixed version (package %s -> %s, or update image %s).",
			component.ComponentName, match.FixedVersion, ev.ContainerImage),

		// CVE specific fields
		CVEID:             match.CVEID,
		CVSS:              match.CVSS, // float32
		AffectedComponent: component.ComponentName,
		AffectedVersion:   component.ComponentVersion,
		FixedVersion:      match.FixedVersion,
		DetectedAt:        time.Now(),
	}
}
