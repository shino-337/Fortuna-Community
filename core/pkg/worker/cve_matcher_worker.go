package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fortuna/core/internal/repository"
	"github.com/fortuna/core/pkg/cve/database"
	"github.com/fortuna/core/pkg/cve/matcher"
	"github.com/fortuna/core/pkg/epss"
	"github.com/fortuna/core/pkg/insightbuilder"
	"github.com/fortuna/core/pkg/insightevidence"
	"github.com/fortuna/core/pkg/kev"
	"github.com/fortuna/core/pkg/malware"
	"github.com/fortuna/core/pkg/metrics"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
	"github.com/fortuna/core/pkg/riskengine"
	"github.com/fortuna/core/pkg/sbom"
	"github.com/nats-io/nats.go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SubjectInsightsUpdated is published after insights are created/updated so Risk Center WS can broadcast.
const SubjectInsightsUpdated = "fortuna.insights.updated"

// SubjectSIEMEvents is published for critical/high insights so SIEM adapters can forward to webhooks.
const SubjectSIEMEvents = "fortuna.siem.events"

// PublishInsightsUpdatedFunc publishes insight-update notification (e.g. via core NATS for fan-out to all Core replicas). Finding #1.3.
// When nil, worker falls back to JetStream publish (single-replica broadcast).
type PublishInsightsUpdatedFunc func(data []byte) error

func incCVEMatcherRun(result string) {
	metrics.CVEMatcherRunsTotal.WithLabelValues(result, matcher.ResolverVersion).Inc()
}

func (w *CVEMatcherWorker) activeCatalogGenerationID(ctx context.Context, mirrorVersion string) uint {
	if w == nil || w.db == nil || !w.db.Migrator().HasTable("catalog_generations") {
		return 0
	}
	query := w.db.WithContext(ctx).
		Where("catalog_type = ? AND status = ?", "cve", "active")
	if strings.TrimSpace(mirrorVersion) != "" {
		query = query.Where("mirror_version = ?", mirrorVersion)
	}
	var generation models.CatalogGeneration
	if err := query.Order("activated_at DESC, id DESC").First(&generation).Error; err == nil {
		return generation.ID
	}
	return 0
}

// CVEMatcherWorker implements: SBOM_CREATED -> CVE Matching -> Persist cve_matches -> Vulnerability Insights;
// plus malware DB matching -> malware_matches -> supply_chain_malware insights (Risk Operations / trend / SIEM).
type CVEMatcherWorker struct {
	js                     nats.JetStreamContext
	db                     *gorm.DB
	dbManager              *database.Manager
	matcher                *matcher.Matcher
	insightMgr             *riskengine.InsightManager
	logger                 *log.Logger
	onlySeverities         map[string]bool
	publishInsightsUpdated PublishInsightsUpdatedFunc
}

func NewCVEMatcherWorker(js nats.JetStreamContext, db *gorm.DB, publishInsightsUpdated PublishInsightsUpdatedFunc) *CVEMatcherWorker {
	dbMgr := database.NewPostgresManager(db)
	m := matcher.NewMatcher(dbMgr, db)

	// Always attached: the feeds may be empty now and synced later (the manager reloads).
	m.SetMalwareChecker(malware.NewManager(db))

	return &CVEMatcherWorker{
		js:                     js,
		db:                     db,
		dbManager:              dbMgr,
		matcher:                m,
		insightMgr:             riskengine.NewInsightManager(db),
		logger:                 log.New(log.Writer(), "[CVEMatcherWorker] ", log.LstdFlags),
		publishInsightsUpdated: publishInsightsUpdated,
		onlySeverities: map[string]bool{
			"CRITICAL": true,
			"HIGH":     true,
			"MEDIUM":   true, // Temporarily added for testing
		},
	}
}

func (w *CVEMatcherWorker) Subject() string { return "fortuna.sbom.created" }

func (w *CVEMatcherWorker) Process(ctx context.Context, msg *nats.Msg) error {
	var ev sbom.SBOMCreatedEvent
	if err := json.Unmarshal(msg.Data, &ev); err != nil {
		incCVEMatcherRun("error")
		return fmt.Errorf("unmarshal sbom.created: %w", err)
	}
	return w.ProcessSBOMCreatedEvent(ctx, ev)
}

// ProcessSBOMCreatedEventJSON unmarshals data and runs ProcessSBOMCreatedEvent (gRPC in-process CVE path).
func (w *CVEMatcherWorker) ProcessSBOMCreatedEventJSON(ctx context.Context, data []byte) error {
	var ev sbom.SBOMCreatedEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		return fmt.Errorf("unmarshal sbom.created: %w", err)
	}
	return w.ProcessSBOMCreatedEvent(ctx, ev)
}

// ProcessSBOMCreatedEvent runs CVE matching + persistence + insights for one sbom.created event.
func (w *CVEMatcherWorker) ProcessSBOMCreatedEvent(ctx context.Context, ev sbom.SBOMCreatedEvent) error {
	startProcess := time.Now()
	if ev.SBOMID == 0 {
		incCVEMatcherRun("skipped")
		return nil
	}
	if ev.SchemaVersion != "" && ev.SchemaVersion != sbom.SBOMCreatedEventSchemaVersion {
		w.logger.Printf("[CVEMatcherRun] correlation_id=%s sbom_id=%d schema_version=%q expected=%q resolver_version=%s result=schema_mismatch_skipped",
			ev.CorrelationID, ev.SBOMID, ev.SchemaVersion, sbom.SBOMCreatedEventSchemaVersion, matcher.ResolverVersion)
		incCVEMatcherRun("skipped")
		return nil
	}

	// Load SBOM (for component_count/logging)
	var sbomModel models.SBOM
	if err := w.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", ev.SBOMID).
		First(&sbomModel).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("load SBOM: %w", err)
		}
		// If SBOM doesn't exist (deleted or never created), skip silently to avoid retry loops
		w.logger.Printf("⚠️  SBOM id=%d not found (may have been deleted), skipping CVE matching", ev.SBOMID)
		incCVEMatcherRun("skipped")
		return nil // Don't retry deleted SBOMs
	}

	// Validate ownership before replay claims, match runs, or writes.
	if _, err := resourceidentity.New(ev.ClusterID, ev.PodUID); err != nil {
		return fmt.Errorf("SBOM event ownership: %w", err)
	}
	if ev.ClusterID != sbomModel.ClusterID || ev.PodUID != sbomModel.PodUID || ev.ContainerName != sbomModel.ContainerName || ev.ContainerName == "" || ev.ImageDigest != sbomModel.ImageDigest {
		return fmt.Errorf("SBOM event ownership mismatch for sbom_id=%d", ev.SBOMID)
	}
	ev.PodName, ev.PodNamespace = sbomModel.PodName, sbomModel.Namespace

	// Resolve mirror version once, then freeze on ctx for the whole run (avoid mid-run mirror_state bump drift).
	mirrorVersion := w.dbManager.GetMirrorVersion(ctx, "osv")
	if strings.TrimSpace(mirrorVersion) == "" {
		mirrorVersion = fmt.Sprintf("ts-%d", time.Now().Unix()/3600)
	}
	catalogGenerationID := w.activeCatalogGenerationID(ctx, mirrorVersion)
	ctx = database.WithFrozenMirrorVersion(ctx, "osv", mirrorVersion)
	sbomRepo := repository.NewSBOMRepository(w.db)

	resolverVersion := matcher.ResolverVersion
	matcherVersion := matcher.ResolverVersion

	// PR-4 replay guard (atomic): skip stale replayed events by timestamp.
	if ev.Timestamp > 0 {
		ok, err := sbomRepo.ClaimSBOMEvent(ctx, ev.SBOMID, ev.EventID, ev.Timestamp)
		if err != nil {
			incCVEMatcherRun("error")
			return fmt.Errorf("claim sbom event sbom_id=%d ts=%d: %w", ev.SBOMID, ev.Timestamp, err)
		}
		if !ok {
			incCVEMatcherRun("replay")
			w.logger.Printf("[CVEMatcherRun] correlation_id=%s sbom_id=%d resolver_version=%s result=replay_skipped event_id=%s event_ts=%d",
				ev.CorrelationID, ev.SBOMID, matcher.ResolverVersion, ev.EventID, ev.Timestamp)
			return nil
		}
	}

	ok, err := sbomRepo.EnsureMatchRun(ctx, sbomModel.ID, sbomModel.Version, mirrorVersion, catalogGenerationID, resolverVersion, matcherVersion)
	if err != nil {
		incCVEMatcherRun("error")
		return fmt.Errorf("ensure match run sbom_id=%d version=%d mirror=%s: %w", sbomModel.ID, sbomModel.Version, mirrorVersion, err)
	}
	if !ok {
		incCVEMatcherRun("duplicate")
		w.logger.Printf("[CVEMatcherRun] correlation_id=%s sbom_id=%d version=%d mirror=%s resolver_version=%s result=duplicate",
			ev.CorrelationID, sbomModel.ID, sbomModel.Version, mirrorVersion, matcher.ResolverVersion)
		return nil
	}
	runStatus := "failed"
	runErrorCode := "processing_error"
	defer func() {
		if err := sbomRepo.CompleteMatchRun(ctx, sbomModel.ID, sbomModel.Version, mirrorVersion, runStatus, runErrorCode); err != nil {
			w.logger.Printf("⚠️  Failed to complete match run sbom_id=%d version=%d mirror=%s status=%s: %v",
				sbomModel.ID, sbomModel.Version, mirrorVersion, runStatus, err)
		}
	}()

	// P1-5: when event carries component snapshot, use it to avoid soft-delete race; else load from DB
	var componentsOverride []*models.SBOMComponent
	if len(ev.ComponentsSnapshot) > 0 {
		for i := range ev.ComponentsSnapshot {
			s := &ev.ComponentsSnapshot[i]
			componentsOverride = append(componentsOverride, &models.SBOMComponent{
				SBOMID:           sbomModel.ID,
				ComponentName:    s.Name,
				ComponentVersion: s.Version,
				PURL:             s.PURL,
				Source:           s.Source,
				TrustLevel:       s.TrustLevel,
				OriginalPURL:     s.OriginalPURL,
				PURLValidated:    s.PURLValidated,
				NormalizedName:   s.NormalizedName,
				VersionClass:     s.VersionClass,
				Ecosystem:        s.Ecosystem,
				Namespace:        s.Namespace,
				Arch:             s.Arch,
			})
		}
	}
	// Match CVEs using postgres-backed manager (cves + package_vulnerabilities)
	startMatch := time.Now()
	matches, catalogMalware, complete, err := w.matcher.MatchSBOMFindings(ctx, &sbomModel, componentsOverride)
	if err != nil {
		incCVEMatcherRun("error")
		return fmt.Errorf("match sbom id=%d: %w", sbomModel.ID, err)
	}
	metrics.CVEMatchingDuration.Observe(time.Since(startMatch).Seconds())

	if err := w.persistMatches(ctx, sbomModel.ID, matches, complete); err != nil {
		incCVEMatcherRun("error")
		return err
	}

	// Malware matching: always run even when CVE matches=0 (demo / supply-chain hits may have no OSV row).
	var allComponents []models.SBOMComponent
	if componentsOverride != nil {
		for _, co := range componentsOverride {
			if co != nil {
				allComponents = append(allComponents, *co)
			}
		}
	} else {
		if err := w.db.WithContext(ctx).Where("sbom_id = ? AND deleted_at IS NULL", sbomModel.ID).Find(&allComponents).Error; err != nil {
			return fmt.Errorf("load malware components: %w", err)
		}
	}
	malwareMatches := w.matcher.MatchMalwareWithCatalog(ctx, &sbomModel, allComponents, catalogMalware)
	if err := w.persistMalwareMatches(ctx, sbomModel.ID, malwareMatches, complete); err != nil {
		return err
	}
	if len(malwareMatches) > 0 {
		w.logger.Printf("[MalwareMatch] sbom_id=%d malware_hits=%d", sbomModel.ID, len(malwareMatches))
	}
	if complete {
		// Malicious-package advisories used to be reported as vulnerabilities; they are now
		// supply_chain_malware insights, so retire the pod's vulnerability insights for them.
		if err := w.db.WithContext(ctx).Exec(`UPDATE insights SET deleted_at = ?, updated_at = ?
WHERE insight_type = 'vulnerability' AND cluster_id = ? AND resource_uid = ? AND cve_id LIKE 'MAL-%' AND deleted_at IS NULL`,
			time.Now(), time.Now(), sbomModel.ClusterID, sbomModel.PodUID).Error; err != nil {
			w.logger.Printf("⚠️  Failed to retire malicious-package vulnerability insights sbom_id=%d: %v", sbomModel.ID, err)
		}
	}

	if len(matches) == 0 && len(malwareMatches) == 0 {
		runStatus = "succeeded"
		runErrorCode = ""
		incCVEMatcherRun("skipped")
		w.logger.Printf("[CVEMatcherRun] correlation_id=%s sbom_id=%d version=%d mirror=%s resolver_version=%s result=skipped matches=0 malware=0 duration_ms=%d",
			ev.CorrelationID, sbomModel.ID, sbomModel.Version, mirrorVersion, matcher.ResolverVersion, time.Since(startProcess).Milliseconds())
		return nil
	}

	// Observability: count CVE matches by severity (FORTUNA_CVE_MATCHING_ENGINE §13)
	for _, m := range matches {
		sev := strings.TrimSpace(strings.ToUpper(m.Severity))
		if sev == "" {
			sev = "UNKNOWN"
		}
		metrics.CVEMatchesTotal.WithLabelValues(sev).Inc()
	}
	incCVEMatcherRun("processed")
	w.logger.Printf("[CVEMatcherRun] correlation_id=%s sbom_id=%d version=%d mirror=%s resolver_version=%s result=processed cve_matches=%d malware_hits=%d duration_ms=%d",
		ev.CorrelationID, sbomModel.ID, sbomModel.Version, mirrorVersion, matcher.ResolverVersion, len(matches), len(malwareMatches), time.Since(startProcess).Milliseconds())

	// INS-1: snapshot map for component resolution (CVE + supply-chain malware insights).
	snapshotByName := make(map[string]*models.SBOMComponent, len(componentsOverride))
	for _, co := range componentsOverride {
		if co == nil || strings.TrimSpace(co.ComponentName) == "" {
			continue
		}
		if _, ok := snapshotByName[co.ComponentName]; !ok {
			cp := *co
			snapshotByName[co.ComponentName] = &cp
		}
	}

	sbomStatus := strings.ToLower(strings.TrimSpace(sbomModel.Status))
	var cveInsights []*models.Insight
	var cveInsightEcosystems []string
	var allSBOMComponents []models.SBOMComponent

	// --- CVE → vulnerability insights (unchanged semantics) ---
	if len(matches) > 0 {
		packageNames := make([]string, 0, len(matches))
		for _, m := range matches {
			if w.onlySeverities[strings.ToUpper(m.Severity)] {
				packageNames = append(packageNames, m.PackageName)
			}
		}
		if len(packageNames) > 0 {
			var components []models.SBOMComponent
			if err := w.db.WithContext(ctx).
				Where("sbom_id = ? AND component_name IN ? AND deleted_at IS NULL",
					sbomModel.ID, packageNames).
				Find(&components).Error; err != nil {
				w.logger.Printf("⚠️  Failed to load components: %v", err)
				return fmt.Errorf("load components: %w", err)
			}
			if err := w.db.WithContext(ctx).
				Where("sbom_id = ? AND deleted_at IS NULL", sbomModel.ID).
				Find(&allSBOMComponents).Error; err != nil {
				w.logger.Printf("⚠️  Failed to load all SBOM components for coverage metrics: %v", err)
			}
			w.emitSBOMCoverageMetrics(ctx, &sbomModel, allSBOMComponents, matches)

			componentMap := make(map[string]*models.SBOMComponent)
			for i := range components {
				componentMap[components[i].ComponentName] = &components[i]
			}
			for name, snap := range snapshotByName {
				if _, ok := componentMap[name]; !ok && snap != nil {
					componentMap[name] = snap
				}
			}

			cveInsights = make([]*models.Insight, 0, len(matches))
			cveInsightEcosystems = make([]string, 0, len(matches))

			epssLimit := 40
			if s := strings.TrimSpace(os.Getenv("FORTUNA_EPSS_MAX_PER_SBOM")); s != "" {
				if v, err := strconv.Atoi(s); err == nil {
					epssLimit = v
				}
			}
			if epssLimit < 0 {
				epssLimit = 10000
			}

			type matchWork struct {
				m         *models.CVEMatch
				component *models.SBOMComponent
			}
			works := make([]matchWork, 0, len(matches))
			for _, m := range matches {
				if !w.onlySeverities[strings.ToUpper(m.Severity)] {
					continue
				}
				component, foundComp := componentMap[m.PackageName]
				if !foundComp {
					w.logger.Printf("⚠️  Component not found for package %s", m.PackageName)
					continue
				}
				works = append(works, matchWork{m: m, component: component})
			}

			// KEV and EPSS come from the vulnerabilities table once the update job has loaded
			// them; before that, from the optional live CISA and FIRST lookups.
			workIDs := make([]string, 0, len(works))
			for _, wk := range works {
				workIDs = append(workIDs, canonicalCVEID(wk.m.CVEID))
			}
			stored := loadExploitData(ctx, w.db, workIDs)

			epssResults := make(map[string]epss.EpssResult)
			useEPSSAPI := !stored.epssLoaded && epss.Enabled() && epssLimit > 0
			if useEPSSAPI && len(works) > 0 {
				seenCVE := make(map[string]struct{})
				ids := make([]string, 0, epssLimit)
				for _, id := range workIDs {
					if id == "" {
						continue
					}
					if _, ok := seenCVE[id]; ok {
						continue
					}
					seenCVE[id] = struct{}{}
					ids = append(ids, id)
					if len(ids) >= epssLimit {
						break
					}
				}
				if len(ids) > 0 {
					epssResults = epss.LookupManyDefault(ctx, ids)
				}
			}

			for _, wk := range works {
				insight := buildVulnInsightFromEvent(ev, sbomStatus, wk.component, wk.m)
				patch := map[string]interface{}{}
				id := canonicalCVEID(wk.m.CVEID)
				if row, ok := stored.byID[id]; ok && stored.epssLoaded && row.EPSSScore != nil {
					patch["epss"] = *row.EPSSScore
					patch["epss_percentile"] = derefFloat(row.EPSSPercentile)
					patch["epss_source"] = "first.org"
					if row.EPSSDate != nil {
						patch["epss_date"] = row.EPSSDate.Format("2006-01-02")
					}
				} else if useEPSSAPI {
					if r, ok := epssResults[id]; ok {
						patch["epss"] = r.EPSS
						patch["epss_percentile"] = r.Percentile
						patch["epss_source"] = "first.org"
					}
				}
				if stored.kevLoaded {
					if row, ok := stored.byID[id]; ok && row.KEVAddedAt != nil {
						patch["cisa_kev"] = true
						patch["cisa_kev_added"] = row.KEVAddedAt.Format("2006-01-02")
						if row.KEVRansomware != nil && *row.KEVRansomware {
							patch["cisa_kev_ransomware"] = true
						}
					}
				} else if kev.Enabled() && kev.Contains(id) {
					patch["cisa_kev"] = true
				}
				if len(patch) > 0 {
					insight.Evidence = insightevidence.Merge(insight.Evidence, patch)
				}
				cveInsights = append(cveInsights, insight)
				cveInsightEcosystems = append(cveInsightEcosystems, resolveComponentEcosystemForMetrics(&sbomModel, wk.component))
			}

			if len(cveInsights) > 0 {
				w.emitRiskConfidenceDistributionMetrics(sbomStatus, sbomModel.StatusReason, cveInsights, cveInsightEcosystems, allSBOMComponents, resolverVersion, &sbomModel)
			}
		}
	}

	// --- Malware packages → supply_chain_malware insights (Risk Operations / trend / SIEM) ---
	var malInsights []*models.Insight
	if len(malwareMatches) > 0 {
		seenName := make(map[string]struct{})
		var malNames []string
		for _, mm := range malwareMatches {
			n := strings.TrimSpace(mm.PackageName)
			if n == "" {
				continue
			}
			if _, ok := seenName[n]; ok {
				continue
			}
			seenName[n] = struct{}{}
			malNames = append(malNames, n)
		}
		if len(malNames) > 0 {
			var malComponents []models.SBOMComponent
			if err := w.db.WithContext(ctx).
				Where("sbom_id = ? AND component_name IN ? AND deleted_at IS NULL", sbomModel.ID, malNames).
				Find(&malComponents).Error; err != nil {
				w.logger.Printf("⚠️  Failed to load components for malware insights: %v", err)
				return fmt.Errorf("load components for malware insights: %w", err)
			}
			malMap := make(map[string]*models.SBOMComponent)
			for i := range malComponents {
				malMap[malComponents[i].ComponentName] = &malComponents[i]
			}
			for name, snap := range snapshotByName {
				if _, ok := malMap[name]; !ok && snap != nil {
					malMap[name] = snap
				}
			}
			for _, mm := range malwareMatches {
				comp := malMap[strings.TrimSpace(mm.PackageName)]
				if comp == nil {
					comp = &models.SBOMComponent{
						SBOMID:           sbomModel.ID,
						ComponentName:    mm.PackageName,
						ComponentVersion: mm.PackageVersion,
					}
				}
				malInsights = append(malInsights, buildSupplyChainMalwareInsight(ev, sbomStatus, comp, mm))
			}
		}
	}

	allInsights := make([]*models.Insight, 0, len(cveInsights)+len(malInsights))
	allInsights = append(allInsights, cveInsights...)
	allInsights = append(allInsights, malInsights...)

	if len(allInsights) > 0 {
		for _, insight := range allInsights {
			if insight == nil {
				continue
			}
			insight.ClusterID = ev.ClusterID
			insight.ResourceType = "Pod"
		}
		start := time.Now()
		if err := w.insightMgr.BatchCreateOrUpdatePodInsights(allInsights); err != nil {
			w.logger.Printf("⚠️  Failed to batch create/update insights: %v", err)
			return fmt.Errorf("batch create insights: %w", err)
		}
		metrics.RiskEvaluationDuration.Observe(time.Since(start).Seconds())
		metrics.InsightsBatchSize.Observe(float64(len(allInsights)))
		if len(cveInsights) > 0 {
			w.logger.Printf("✅ Created/updated %d vulnerability insights for pod %s/%s", len(cveInsights), ev.PodNamespace, ev.PodName)
		}
		if len(malInsights) > 0 {
			w.logger.Printf("✅ Created/updated %d supply-chain malware insights for pod %s/%s", len(malInsights), ev.PodNamespace, ev.PodName)
		}
		if w.publishInsightsUpdated != nil {
			_ = w.publishInsightsUpdated([]byte("{}"))
		} else if w.js != nil {
			_, _ = w.js.Publish(SubjectInsightsUpdated, []byte("{}"))
		}
		PublishSIEMEvents(w.js, allInsights)
	}

	runStatus = "succeeded"
	runErrorCode = ""
	return nil
}

// persistMatches stores the findings of one match run. A finding already stored for the SBOM is
// updated (a new catalog can change its severity, fix or advisories). When the run is complete
// the stored set is replaced: findings the catalog no longer reports are removed.
func (w *CVEMatcherWorker) persistMatches(ctx context.Context, sbomID uint, matches []*models.CVEMatch, complete bool) error {
	now := time.Now()
	// The table keeps one row per (package name, CVE); two versions of a package share it.
	byKey := make(map[string]*models.CVEMatch, len(matches))
	unique := make([]*models.CVEMatch, 0, len(matches))
	for _, m := range matches {
		key := m.PackageName + "\x1f" + m.CVEID
		if first := byKey[key]; first != nil {
			for _, id := range m.AdvisoryIDs {
				if !containsString(first.AdvisoryIDs, id) {
					first.AdvisoryIDs = append(first.AdvisoryIDs, id)
				}
			}
			continue
		}
		if m.MatchedAt.IsZero() {
			m.MatchedAt = now
		}
		if m.MatchedBy == "" {
			m.MatchedBy = "fortuna-core-cve-matcher"
		}
		byKey[key] = m
		unique = append(unique, m)
	}
	matches = unique
	return w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Upsert on the unique index (sbom_id, package_name, cve_id).
		const batchSize = 500
		for i := 0; i < len(matches); i += batchSize {
			end := i + batchSize
			if end > len(matches) {
				end = len(matches)
			}
			batch := matches[i:end]
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "sbom_id"}, {Name: "package_name"}, {Name: "cve_id"}},
				DoUpdates: clause.AssignmentColumns([]string{
					"package_version", "p_url", "severity", "cvss", "fixed_version", "matched_by",
					"advisory_ids", "severity_source", "updated_at", "deleted_at",
				}),
			}).Create(&batch).Error; err != nil {
				return fmt.Errorf("persist cve_matches batch: %w", err)
			}
		}
		if !complete {
			return nil
		}
		var stored []struct {
			ID          uint
			PackageName string
			CVEID       string
		}
		if err := tx.Model(&models.CVEMatch{}).Unscoped().Select("id, package_name, cve_id").
			Where("sbom_id = ?", sbomID).Scan(&stored).Error; err != nil {
			return fmt.Errorf("list stored cve_matches: %w", err)
		}
		var stale []uint
		for _, row := range stored {
			if byKey[row.PackageName+"\x1f"+row.CVEID] == nil {
				stale = append(stale, row.ID)
			}
		}
		if len(stale) > 0 {
			if err := tx.Unscoped().Where("id IN ?", stale).Delete(&models.CVEMatch{}).Error; err != nil {
				return fmt.Errorf("remove stale cve_matches: %w", err)
			}
		}
		return nil
	})
}

func containsString(values []string, v string) bool {
	for _, have := range values {
		if have == v {
			return true
		}
	}
	return false
}

// persistMalwareMatches stores the malware findings of one match run, one row per package of
// the SBOM: a stored row is refreshed (verdict, feeds, advisories), a new one inserted, and when
// the run was complete the rows of packages no longer flagged (dropped from a feed) are removed.
func (w *CVEMatcherWorker) persistMalwareMatches(ctx context.Context, sbomID uint, matches []*models.MalwareMatch, complete bool) error {
	if len(matches) == 0 && (!complete || !w.db.Migrator().HasTable(&models.MalwareMatch{})) {
		return nil // nothing to store or retire
	}
	err := w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var stored []models.MalwareMatch
		if err := tx.Where("sbom_id = ?", sbomID).Find(&stored).Error; err != nil {
			return fmt.Errorf("load stored rows: %w", err)
		}
		byKey := make(map[string]*models.MalwareMatch, len(stored))
		for i := range stored {
			byKey[stored[i].PackageName+"\x1f"+stored[i].PackageVersion] = &stored[i]
		}
		keep := make(map[uint]bool, len(matches))
		for _, m := range matches {
			if m == nil {
				continue
			}
			key := m.PackageName + "\x1f" + m.PackageVersion
			if old := byKey[key]; old != nil {
				if keep[old.ID] {
					continue
				}
				keep[old.ID] = true
				m.ID, m.MatchedAt, m.CreatedAt = old.ID, old.MatchedAt, old.CreatedAt
				if err := tx.Model(&models.MalwareMatch{}).Where("id = ?", old.ID).Updates(map[string]interface{}{
					"component_id":   m.ComponentID,
					"ecosystem":      m.Ecosystem,
					"reason":         m.Reason,
					"confidence":     m.Confidence,
					"malware_family": m.MalwareFamily,
					"sources":        m.Sources,
					"advisory_ids":   m.AdvisoryIDs,
				}).Error; err != nil {
					return fmt.Errorf("update row: %w", err)
				}
				continue
			}
			if err := tx.Create(m).Error; err != nil {
				return fmt.Errorf("insert row: %w", err)
			}
			byKey[key] = m
			keep[m.ID] = true
		}
		if !complete {
			return nil
		}
		var stale []uint
		for i := range stored {
			if !keep[stored[i].ID] {
				stale = append(stale, stored[i].ID)
			}
		}
		if len(stale) > 0 {
			if err := tx.Where("id IN ?", stale).Delete(&models.MalwareMatch{}).Error; err != nil {
				return fmt.Errorf("delete stale rows: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("persist malware match: %w", err)
	}
	return nil
}

func buildVulnInsightFromEvent(ev sbom.SBOMCreatedEvent, sbomStatus string, component *models.SBOMComponent, match *models.CVEMatch) *models.Insight {
	sevLower := strings.ToLower(strings.TrimSpace(match.Severity))
	if sevLower == "" {
		sevLower = "medium"
	}

	// Confidence levels are data trust (not CVSS severity).
	// Ordering for "min" semantics:
	// VERY_LOW < LOW < MEDIUM < HIGH
	confRank := func(c string) int {
		switch strings.ToUpper(strings.TrimSpace(c)) {
		case "HIGH":
			return 3
		case "MEDIUM":
			return 2
		case "LOW":
			return 1
		case "VERY_LOW":
			return 0
		default:
			return 1 // default LOW to be conservative
		}
	}
	minConf := func(a, b string) string {
		if confRank(a) <= confRank(b) {
			return a
		}
		return b
	}
	min3Conf := func(a, b, c string) string {
		return minConf(minConf(a, b), c)
	}
	capConf := func(v, cap string) string {
		if confRank(v) > confRank(cap) {
			return cap
		}
		return v
	}

	sbomConf := func(status string) string {
		switch strings.ToLower(strings.TrimSpace(status)) {
		case "complete":
			return "HIGH"
		case "partial":
			return "MEDIUM"
		case "failed":
			return "VERY_LOW"
		case "pending":
			return "LOW"
		default:
			return "LOW"
		}
	}(sbomStatus)

	componentConf := func(c *models.SBOMComponent) string {
		if c == nil {
			return "VERY_LOW"
		}
		if strings.EqualFold(strings.TrimSpace(c.ComponentVersion), "unknown") {
			// Unknown versions must be capped at LOW (per Phase 2 acceptance).
			return "LOW"
		}

		sd := strings.ToLower(strings.TrimSpace(c.SourceDetail))

		switch sd {
		case "agent-fields":
			// Within agent-fields, use TrustLevel to distinguish high vs inferred.
			tl := strings.ToLower(strings.TrimSpace(c.TrustLevel))
			switch tl {
			case "high":
				return "HIGH"
			case "medium":
				return "MEDIUM"
			case "low":
				return "LOW"
			default:
				return "LOW"
			}
		case "core-regenerated-purl":
			// Core regenerated PURL due to invalid/malformed inputs => low trust.
			return "LOW"
		case "core-generated-purl":
			// Core-generated/inferred PURL (fallback) => medium trust.
			return "MEDIUM"
		default:
			// Unknown provenance => low trust (do not mark as high).
			return "LOW"
		}
	}(component)

	versionKnown := component != nil && !strings.EqualFold(strings.TrimSpace(component.ComponentVersion), "unknown")
	matchConf := matchConfidenceLevelForMetrics(match, versionKnown)

	// Attach computed confidence to match object (transient, not persisted).
	if match != nil {
		match.MatchConfidence = matchConf
	}

	finalConf := min3Conf(sbomConf, componentConf, matchConf)
	// Guard 1: failed SBOM overrides final.
	if strings.ToLower(strings.TrimSpace(sbomStatus)) == "failed" {
		finalConf = "VERY_LOW"
	}
	// Guard 2: degraded (partial/pending) can't be HIGH.
	if strings.ToLower(strings.TrimSpace(sbomStatus)) != "complete" {
		finalConf = capConf(finalConf, "MEDIUM")
	}
	degraded := strings.ToLower(strings.TrimSpace(sbomStatus)) == "partial"

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

	title := fmt.Sprintf("%s in %s", match.CVEID, component.ComponentName)
	if sbomStatus == "partial" {
		title = "[DEGRADED] " + title
		description = description + "\nSBOM status: partial (degraded match; trust-level reduced)."
	}

	return &models.Insight{
		ResourceType:      "Pod",
		ResourceNamespace: ev.PodNamespace,
		ResourceName:      ev.PodName,
		ResourceUID:       ev.PodUID,
		InsightType:       "vulnerability",
		Severity:          sevLower,
		Title:             title,
		Description:       description,
		Status:            "active",
		Recommendation: fmt.Sprintf("Update image/package to a fixed version (package %s -> %s, or update image %s).",
			component.ComponentName, match.FixedVersion, ev.ContainerImage),

		MatchConfidence:     matchConf,
		ComponentConfidence: componentConf,
		SBOMConfidence:      sbomConf,
		FinalRiskConfidence: finalConf,
		Degraded:            degraded,

		// CVE specific fields
		CVEID:             match.CVEID,
		CVSS:              match.CVSS, // float32
		AffectedComponent: component.ComponentName,
		AffectedVersion:   component.ComponentVersion,
		FixedVersion:      match.FixedVersion,
		DetectedAt:        time.Now(),
	}
}

func buildSupplyChainMalwareInsight(ev sbom.SBOMCreatedEvent, sbomStatus string, component *models.SBOMComponent, mm *models.MalwareMatch) *models.Insight {
	return insightbuilder.BuildSupplyChainMalwareInsight(insightbuilder.SupplyChainMalwareInput{
		PodUID:         ev.PodUID,
		PodNamespace:   ev.PodNamespace,
		PodName:        ev.PodName,
		ContainerName:  ev.ContainerName,
		ContainerImage: ev.ContainerImage,
		SBOMStatus:     sbomStatus,
		Component:      component,
		Match:          mm,
	})
}

// resolveComponentEcosystemForMetrics resolves ecosystem for metric grouping.
// Fallback chain (review requirement):
// 1) PURL.ecosystem
// 2) component.Ecosystem (non-persisted; may be empty)
// 3) sbom.OSName
// 4) "unknown"
func resolveComponentEcosystemForMetrics(sbom *models.SBOM, c *models.SBOMComponent) string {
	eco := "unknown"
	if c == nil {
		return eco
	}

	// 1) Try PURL ecosystem (best-effort)
	if p, err := matcher.ParsePURL(strings.TrimSpace(c.PURL)); err == nil && p != nil && strings.TrimSpace(p.Ecosystem) != "" {
		eco = strings.ToLower(strings.TrimSpace(p.Ecosystem))
		if eco == "golang" {
			eco = "go"
		}
		return eco
	}

	// 2) Non-persisted ecosystem field (if present in memory)
	if strings.TrimSpace(c.Ecosystem) != "" {
		eco = strings.ToLower(strings.TrimSpace(c.Ecosystem))
		return eco
	}

	// 3) SBOM OS name fallback (persisted)
	if sbom != nil && strings.TrimSpace(sbom.OSName) != "" {
		eco = strings.ToLower(strings.TrimSpace(sbom.OSName))
		return eco
	}

	return eco
}

func componentConfidenceLevelForCoverage(c *models.SBOMComponent) string {
	// Confidence is derived from provenance+trust (no CVSS).
	if c == nil {
		return "LOW"
	}
	if strings.EqualFold(strings.TrimSpace(c.ComponentVersion), "unknown") {
		return "LOW"
	}
	sd := strings.ToLower(strings.TrimSpace(c.SourceDetail))
	tl := strings.ToLower(strings.TrimSpace(c.TrustLevel))
	if tl == "" {
		tl = "high"
	}
	switch sd {
	case "agent-fields":
		switch tl {
		case "high":
			return "HIGH"
		case "medium":
			return "MEDIUM"
		case "low":
			return "LOW"
		default:
			return "LOW"
		}
	case "core-generated-purl":
		return "MEDIUM"
	case "core-regenerated-purl":
		return "LOW"
	default:
		return "LOW"
	}
}

// matchConfidenceLevelForMetrics computes match trust confidence (data trust, not CVSS).
// Single source of truth for "match confidence" used in both:
// - insight.MatchConfidence (Phase 2)
// - CVE coverage metrics confidence_level (Phase 3)
//
// Design:
// - If component version is unknown => LOW (prevents confidence inflation).
// - If constraint is present & satisfied => HIGH (subject to heuristic/fallback cap).
// - Heuristic/fallback matches are always capped to LOW.
func matchConfidenceLevelForMetrics(m *models.CVEMatch, componentVersionKnown bool) string {
	if m == nil {
		return "VERY_LOW"
	}

	if !componentVersionKnown {
		return "LOW"
	}

	mb := strings.ToLower(strings.TrimSpace(m.MatchedBy))
	if strings.Contains(mb, "fallback") || strings.Contains(mb, "heuristic") || strings.Contains(mb, "low-confidence") {
		return "LOW"
	}

	// Constraint-aware confidence.
	if m.HasConstraint && m.ConstraintSatisfied {
		return "HIGH"
	}
	return "LOW"
}

// emitSBOMCoverageMetrics emits SBOM coverage quality metrics (Phase 3).
// Best-effort: if any parsing fails, we degrade gracefully and keep ratios meaningful.
func (w *CVEMatcherWorker) emitSBOMCoverageMetrics(ctx context.Context, sbom *models.SBOM, allComponents []models.SBOMComponent, matches []*models.CVEMatch) {
	_ = ctx // reserved for future use

	if sbom == nil {
		return
	}
	sbomStatus := strings.ToLower(strings.TrimSpace(sbom.Status))
	sbomStatusReason := strings.TrimSpace(sbom.StatusReason)
	if sbomStatusReason == "" {
		sbomStatusReason = "ok"
	}
	resolverVersion := matcher.ResolverVersion

	confRank := func(level string) int {
		switch strings.ToUpper(strings.TrimSpace(level)) {
		case "HIGH":
			return 3
		case "MEDIUM":
			return 2
		case "LOW":
			return 1
		default:
			return 0
		}
	}

	total := len(allComponents)
	if total == 0 {
		metrics.SBOMComponentWithVersionRatio.WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).Set(0)
		metrics.SBOMComponentWithEcosystemRatio.WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).Set(0)
		metrics.SBOMComponentWithVersionRatioEffective.WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).Set(0)
		metrics.SBOMComponentWithEcosystemRatioEffective.WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).Set(0)
		metrics.SBOMComponentUnknownVersionRatio.WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).Set(0)
		metrics.SBOMComponentInferredRatio.WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).Set(0)
		metrics.CVEMatchRatioRaw.WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).Set(0)
		metrics.CVEMatchRatioEffective.WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).Set(0)
		metrics.CVEMatchRatio.WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).Set(0)
		metrics.SBOMComponentEffectiveDenominatorTotal.WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).Set(0)
		return
	}

	// Compute component confidences and unknown-version coverage.
	unknownVersionCount := 0
	inferredCount := 0
	effectiveTotal := 0
	effectiveWithVersion := 0
	effectiveWithEcosystem := 0

	// Version coverage
	withVersion := 0
	withEcosystem := 0
	for i := range allComponents {
		c := allComponents[i]
		v := strings.TrimSpace(c.ComponentVersion)
		if strings.EqualFold(v, "unknown") {
			unknownVersionCount++
			reason := "other"
			switch strings.ToLower(strings.TrimSpace(c.Source)) {
			case "distroless-heuristic":
				reason = "distroless"
			}
			switch strings.ToLower(strings.TrimSpace(c.SourceDetail)) {
			case "core-generated-purl", "core-regenerated-purl":
				reason = "inferred"
			}
			if strings.TrimSpace(c.ComponentName) == "" || strings.TrimSpace(c.PURL) == "" {
				reason = "missing_metadata"
			}
			metrics.SBOMComponentUnknownVersionTotal.WithLabelValues(reason, resolverVersion).Inc()
		} else if v != "" {
			withVersion++
		}

		if strings.ToLower(strings.TrimSpace(c.SourceDetail)) != "agent-fields" {
			inferredCount++
		}

		// Resolvable ecosystem coverage (with fallback chain)
		eco := resolveComponentEcosystemForMetrics(sbom, &c)
		if eco != "" && !strings.EqualFold(eco, "unknown") {
			withEcosystem++
		}

		// Effective coverage universe: component_confidence >= MEDIUM
		cc := componentConfidenceLevelForCoverage(&c)
		isEffective := cc == "HIGH" || cc == "MEDIUM"
		if isEffective {
			effectiveTotal++
			if strings.EqualFold(v, "unknown") {
				// Unknown version is always capped to LOW confidence; shouldn't be effective.
				continue
			}
			// version known
			if v != "" {
				effectiveWithVersion++
			}
			// ecosystem resolvable
			if eco != "" && !strings.EqualFold(eco, "unknown") {
				effectiveWithEcosystem++
			}
		}
	}

	// Effective denominator: count components with component_confidence >= MEDIUM
	metrics.SBOMComponentEffectiveDenominatorTotal.
		WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).
		Set(float64(effectiveTotal))

	// component unknown version ratio
	metrics.SBOMComponentUnknownVersionRatio.
		WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).
		Set(float64(unknownVersionCount) / float64(total))
	metrics.SBOMComponentInferredRatio.
		WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).
		Set(float64(inferredCount) / float64(total))

	// CVE match coverage (how many SBOM components actually produced insights)
	matchesByComponentKey := make(map[string][]*models.CVEMatch)
	effectiveMatchedNames := make(map[string]struct{})
	// confidence per matched component (min over its matches)
	minMatchConfByComponent := make(map[string]string)
	// confidence per matched component (max over its matches)
	maxMatchConfByComponent := make(map[string]string)
	for _, m := range matches {
		if m == nil {
			continue
		}
		// Raw: any severity (before worker severity filter)
		matchesByComponentKey[m.PackageName] = append(matchesByComponentKey[m.PackageName], m)

		// Compute match confidence level (for metric confidence_level dimension).
		// "Match trust" is constraint-aware and independent from CVSS severity.
		compVersionKnown := !strings.EqualFold(strings.TrimSpace(m.PackageVersion), "unknown")
		base := matchConfidenceLevelForMetrics(m, compVersionKnown)
		// min across potentially multiple CVEs per same package
		if prev, ok := minMatchConfByComponent[m.PackageName]; ok {
			if confRank(base) < confRank(prev) {
				minMatchConfByComponent[m.PackageName] = base
			}
		} else {
			minMatchConfByComponent[m.PackageName] = base
		}

		// max across potentially multiple CVEs per same package
		if prev, ok := maxMatchConfByComponent[m.PackageName]; ok {
			if confRank(base) > confRank(prev) {
				maxMatchConfByComponent[m.PackageName] = base
			}
		} else {
			maxMatchConfByComponent[m.PackageName] = base
		}

		// Effective: after worker severity filter
		if w.onlySeverities[strings.ToUpper(strings.TrimSpace(m.Severity))] {
			effectiveMatchedNames[m.PackageName] = struct{}{}
		}
	}

	// Single source of truth for raw match ratio:
	// matched = len(matchesByComponentKey[component.ComponentName]) > 0.
	coverageComponents := make([]CoverageComponent, 0, len(allComponents))
	for i := range allComponents {
		coverageComponents = append(coverageComponents, CoverageComponent{ID: allComponents[i].ComponentName})
	}
	coverageMatches := make(map[string][]CoverageMatch, len(matchesByComponentKey))
	for compKey, ms := range matchesByComponentKey {
		// Only presence matters for MatchedComponents; duplicates don't change the count.
		coverageMatches[compKey] = make([]CoverageMatch, len(ms))
	}
	stats := ComputeSBOMCoverageStats(coverageComponents, coverageMatches)

	matchedComponentsEffective := 0
	for i := range allComponents {
		c := allComponents[i]
		if _, ok := effectiveMatchedNames[c.ComponentName]; ok {
			matchedComponentsEffective++
		}
	}

	metrics.SBOMComponentWithVersionRatio.WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).Set(float64(withVersion) / float64(total))
	metrics.SBOMComponentWithEcosystemRatio.WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).Set(float64(withEcosystem) / float64(total))

	// Effective ratios
	versionEffRatio := 0.0
	ecoEffRatio := 0.0
	if effectiveTotal > 0 {
		versionEffRatio = float64(effectiveWithVersion) / float64(effectiveTotal)
		ecoEffRatio = float64(effectiveWithEcosystem) / float64(effectiveTotal)
	}
	metrics.SBOMComponentWithVersionRatioEffective.
		WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).
		Set(versionEffRatio)
	metrics.SBOMComponentWithEcosystemRatioEffective.
		WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).
		Set(ecoEffRatio)

	// CVE match ratios (raw vs effective)
	metrics.CVEMatchRatioRaw.WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).
		Set(stats.RawMatchRatio())
	metrics.CVEMatchRatioEffective.WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).
		Set(float64(matchedComponentsEffective) / float64(total))

	// Backward-compatible alias: old name = effective
	metrics.CVEMatchRatio.WithLabelValues(sbomStatus, sbomStatusReason, resolverVersion).
		Set(float64(matchedComponentsEffective) / float64(total))

	// Counter for raw component match presence (no worker severity filter).
	// Add confidence_level for matched components.
	// confidence_level is derived from match trust (constraint-aware).
	for i := range allComponents {
		c := allComponents[i]
		if list := matchesByComponentKey[c.ComponentName]; len(list) > 0 {
			level := minMatchConfByComponent[c.ComponentName]
			if level == "" {
				level = "LOW"
			}
			metrics.CVEMatchTotal.WithLabelValues("matched", level, resolverVersion).Inc()

			best := maxMatchConfByComponent[c.ComponentName]
			if best == "" {
				best = level
			}
			metrics.CVEMatchBestConfidenceTotal.WithLabelValues(best, resolverVersion).Inc()
		} else {
			metrics.CVEMatchTotal.WithLabelValues("not_matched", "NONE", resolverVersion).Inc()
		}
	}
}

// emitRiskConfidenceDistributionMetrics emits insight confidence distribution (Phase 3).
func (w *CVEMatcherWorker) emitRiskConfidenceDistributionMetrics(sbomStatus string, sbomStatusReason string, insights []*models.Insight, insightEcosystems []string, allComponents []models.SBOMComponent, resolverVersion string, sbom *models.SBOM) {
	levels := []string{"HIGH", "MEDIUM", "LOW", "VERY_LOW"}
	sbomStatus = strings.ToLower(strings.TrimSpace(sbomStatus))
	sbomStatusReason = strings.TrimSpace(sbomStatusReason)
	if sbomStatusReason == "" {
		sbomStatusReason = "ok"
	}
	if resolverVersion == "" {
		resolverVersion = matcher.ResolverVersion
	}

	// MET-1: do not call Reset() — it causes scrape flicker and cross-replica confusion.
	// We only Set label combinations for this SBOM run; stale ecosystem labels from older runs
	// may persist in-process until worker restart (acceptable for ops; use logs for per-SBOM truth).

	// Build ecosystem universe from the SBOM components, so we can set ratios to 0
	// for ecosystems that have no insights in this run (avoid metric staleness).
	ecoSet := make(map[string]struct{})
	for i := range allComponents {
		c := allComponents[i]
		eco := resolveComponentEcosystemForMetrics(sbom, &c)
		if eco == "" {
			eco = "unknown"
		}
		ecoSet[eco] = struct{}{}
	}
	if len(ecoSet) == 0 {
		ecoSet["unknown"] = struct{}{}
	}

	// Count insights per (ecosystem, confidence level)
	countByEco := make(map[string]int)
	countByEcoLevel := make(map[string]map[string]int)
	for idx, insight := range insights {
		if insight == nil {
			continue
		}
		eco := "unknown"
		if idx < len(insightEcosystems) {
			eco = insightEcosystems[idx]
		}
		if eco == "" {
			eco = "unknown"
		}
		level := strings.ToUpper(strings.TrimSpace(insight.FinalRiskConfidence))
		if level == "" {
			level = "LOW"
		}
		countByEco[eco]++
		if countByEcoLevel[eco] == nil {
			countByEcoLevel[eco] = make(map[string]int)
		}
		countByEcoLevel[eco][level]++
	}

	// Emit ratios for all levels & ecosystems in the SBOM universe.
	for eco := range ecoSet {
		total := countByEco[eco]
		for _, level := range levels {
			c := 0
			if countByEcoLevel[eco] != nil {
				c = countByEcoLevel[eco][level]
			}
			ratio := 0.0
			if total > 0 {
				ratio = float64(c) / float64(total)
			}
			metrics.RiskConfidenceDistributionRatio.
				WithLabelValues(level, sbomStatus, sbomStatusReason, eco, resolverVersion).
				Set(ratio)
		}
	}
}

// distroCVEIDRE matches OSV distro advisory IDs that wrap one CVE: DEBIAN-CVE-2024-1234,
// UBUNTU-CVE-2024-1234, ALPINE-CVE-2024-1234.
var distroCVEIDRE = regexp.MustCompile(`^[A-Z]+-(CVE-\d{4}-\d{4,})$`)

// canonicalCVEID returns the CVE ID that KEV and EPSS are keyed by: the advisory ID itself, or
// the CVE a single-CVE distro advisory wraps.
func canonicalCVEID(id string) string {
	id = strings.TrimSpace(strings.ToUpper(id))
	if m := distroCVEIDRE.FindStringSubmatch(id); m != nil {
		return m[1]
	}
	return id
}
