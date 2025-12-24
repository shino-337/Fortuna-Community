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
	// OPTIMIZATION: Load all persisted matches and components in bulk to avoid N+1 queries
	if len(matches) == 0 {
		return nil
	}

	// Collect all package names and CVE IDs for bulk loading
	packageNames := make([]string, 0, len(matches))
	cveIDs := make([]string, 0, len(matches))
	for _, m := range matches {
		if w.onlySeverities[strings.ToUpper(m.Severity)] {
			packageNames = append(packageNames, m.PackageName)
			cveIDs = append(cveIDs, m.CVEID)
		}
	}

	if len(packageNames) == 0 {
		return nil
	}

	// Bulk load all persisted matches (2 queries instead of N*2)
	var persistedMatches []models.CVEMatch
	if err := w.db.WithContext(ctx).
		Where("sbom_id = ? AND package_name IN ? AND cve_id IN ? AND deleted_at IS NULL",
			sbomModel.ID, packageNames, cveIDs).
		Find(&persistedMatches).Error; err != nil {
		w.logger.Printf("⚠️  Failed to load persisted matches: %v", err)
		return fmt.Errorf("load persisted matches: %w", err)
	}

	// Bulk load all components
	var components []models.SBOMComponent
	if err := w.db.WithContext(ctx).
		Where("sbom_id = ? AND component_name IN ? AND deleted_at IS NULL",
			sbomModel.ID, packageNames).
		Find(&components).Error; err != nil {
		w.logger.Printf("⚠️  Failed to load components: %v", err)
		return fmt.Errorf("load components: %w", err)
	}

	// Create lookup maps for O(1) access
	matchMap := make(map[string]*models.CVEMatch)
	for i := range persistedMatches {
		key := persistedMatches[i].PackageName + ":" + persistedMatches[i].CVEID
		matchMap[key] = &persistedMatches[i]
	}

	componentMap := make(map[string]*models.SBOMComponent)
	for i := range components {
		componentMap[components[i].ComponentName] = &components[i]
	}

	// Build insights in batch
	insights := make([]*models.Insight, 0, len(matches))
	for _, m := range matches {
		if !w.onlySeverities[strings.ToUpper(m.Severity)] {
			continue
		}

		// Lookup from maps (O(1) instead of query)
		key := m.PackageName + ":" + m.CVEID
		persisted, foundMatch := matchMap[key]
		if !foundMatch {
			w.logger.Printf("⚠️  Persisted match not found for %s:%s", m.PackageName, m.CVEID)
			continue
		}

		component, foundComp := componentMap[m.PackageName]
		if !foundComp {
			w.logger.Printf("⚠️  Component not found for package %s", m.PackageName)
			continue
		}

		insight := buildVulnInsightFromEvent(ev, component, persisted)
		insights = append(insights, insight)
	}

	// Batch create/update insights (single transaction)
	if len(insights) > 0 {
		if err := w.insightMgr.BatchCreateOrUpdateInsights(insights); err != nil {
			w.logger.Printf("⚠️  Failed to batch create/update insights: %v", err)
			return fmt.Errorf("batch create insights: %w", err)
		}
		w.logger.Printf("✅ Created/updated %d vulnerability insights for pod %s/%s", len(insights), ev.PodNamespace, ev.PodName)
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

	// Batch insert with ON CONFLICT DO NOTHING (requires unique index: (sbom_id, package_name, cve_id))
	const batchSize = 500
	for i := 0; i < len(matches); i += batchSize {
		end := i + batchSize
		if end > len(matches) {
			end = len(matches)
		}
		batch := matches[i:end]
		if err := w.db.WithContext(ctx).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "sbom_id"}, {Name: "package_name"}, {Name: "cve_id"}},
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
