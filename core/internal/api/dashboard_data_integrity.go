package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Dashboard endpoints (GET /dashboard/stats, GET /dashboard/metrics/*) are aggregate APIs:
// they compose multiple sources and may use cache; not stable for external clients.

// DashboardDataIntegrityResponse is the response for GET /health/dashboard-data-integrity.
// It allows operators to verify that dashboard data is traceable and identifies stubs/placeholders.
type DashboardDataIntegrityResponse struct {
	Timestamp     time.Time            `json:"timestamp"`
	CrossChecks   CrossChecks          `json:"crossChecks"`
	CatalogHealth CatalogHealth        `json:"catalogHealth"`
	RuntimeHealth RuntimeHealth        `json:"runtimeHealth"`
	Alerts        []string             `json:"alerts"`
	Endpoints     []EndpointDataSource `json:"endpoints"`
}

type CrossChecks struct {
	ActiveAgentsCount    int64 `json:"activeAgentsCount"`
	DashboardAgentsCount int64 `json:"dashboardAgentsCount"` // same source as dashboard stats
	ClustersCount        int64 `json:"clustersCount"`        // active clusters (recent sync)
	PodsCount            int64 `json:"podsCount"`
	InsightsCount        int64 `json:"insightsCount"`
	CriticalInsights     int64 `json:"criticalInsights"`
	// CVE reference data (filled by cve-loader Job from OSV sync)
	CVEsCount                   int64 `json:"cvesCount"`
	PackageVulnerabilitiesCount int64 `json:"packageVulnerabilitiesCount"`
	OsvPackagesCount            int64 `json:"osvPackagesCount"`
	MalwarePackagesCount        int64 `json:"malwarePackagesCount"`
	SbomsCount                  int64 `json:"sbomsCount"`
	PodsMissingSbom             int64 `json:"podsMissingSbom"`
}

type CatalogHealth struct {
	Status                            string     `json:"status"` // healthy | degraded | stale | unavailable
	MirrorVersion                     string     `json:"mirrorVersion"`
	MirrorUpdatedAt                   *time.Time `json:"mirrorUpdatedAt,omitempty"`
	LastCVEUpdatedAt                  *time.Time `json:"lastCveUpdatedAt,omitempty"`
	LastPackageVulnerabilityUpdate    *time.Time `json:"lastPackageVulnerabilityUpdatedAt,omitempty"`
	LastMalwareUpdatedAt              *time.Time `json:"lastMalwareUpdatedAt,omitempty"`
	LastMalwareFeedSyncAt             *time.Time `json:"lastMalwareFeedSyncAt,omitempty"`
	LastMalwareFeedSyncStatus         string     `json:"lastMalwareFeedSyncStatus,omitempty"`
	ActiveCatalogGenerationID         uint       `json:"activeCatalogGenerationId,omitempty"`
	ActiveCatalogGenerationStatus     string     `json:"activeCatalogGenerationStatus,omitempty"`
	ActiveCatalogSourceDigest         string     `json:"activeCatalogSourceDigest,omitempty"`
	ActiveCatalogActivatedAt          *time.Time `json:"activeCatalogActivatedAt,omitempty"`
	ActiveMalwareGenerationID         uint       `json:"activeMalwareGenerationId,omitempty"`
	ActiveMalwareGenerationStatus     string     `json:"activeMalwareGenerationStatus,omitempty"`
	ActiveMalwareSourceDigest         string     `json:"activeMalwareSourceDigest,omitempty"`
	ActiveMalwareActivatedAt          *time.Time `json:"activeMalwareActivatedAt,omitempty"`
	CVEsCount                         int64      `json:"cvesCount"`
	PackageVulnerabilitiesCount       int64      `json:"packageVulnerabilitiesCount"`
	OsvPackagesCount                  int64      `json:"osvPackagesCount"`
	MalwarePackagesCount              int64      `json:"malwarePackagesCount"`
	ActiveSBOMs                       int64      `json:"activeSboms"`
	StaleSBOMs                        int64      `json:"staleSboms"`
	ActiveSBOMsMatchedMirror          int64      `json:"activeSbomsMatchedMirror"`
	ActiveSBOMsMissingMirrorMatch     int64      `json:"activeSbomsMissingMirrorMatch"`
	ActiveSBOMsMatchedGeneration      int64      `json:"activeSbomsMatchedGeneration"`
	ActiveSBOMsMissingGenerationMatch int64      `json:"activeSbomsMissingGenerationMatch"`
	CurrentMirrorSucceededRuns        int64      `json:"currentMirrorSucceededRuns"`
	CurrentMirrorFailedRuns           int64      `json:"currentMirrorFailedRuns"`
	CurrentMirrorRunningRuns          int64      `json:"currentMirrorRunningRuns"`
	CurrentGenerationSucceededRuns    int64      `json:"currentGenerationSucceededRuns"`
	CurrentGenerationFailedRuns       int64      `json:"currentGenerationFailedRuns"`
	CurrentGenerationRunningRuns      int64      `json:"currentGenerationRunningRuns"`
	ActivePodCVEMatches               int64      `json:"activePodCveMatches"`
	StalePodCVEMatches                int64      `json:"stalePodCveMatches"`
}

type RuntimeHealth struct {
	Status                 string     `json:"status"` // healthy | degraded | unavailable
	Source                 string     `json:"source"` // db-derived
	RuntimeEventsCount     int64      `json:"runtimeEventsCount"`
	RuntimeSignalsCount    int64      `json:"runtimeSignalsCount"`
	RuntimeMetricsCount    int64      `json:"runtimeMetricsCount"`
	FalcoEventsCount       int64      `json:"falcoEventsCount"`
	FalcoStatus            string     `json:"falcoStatus"` // active | no-events | unavailable
	LastRuntimeEventAt     *time.Time `json:"lastRuntimeEventAt,omitempty"`
	LastRuntimeSignalAt    *time.Time `json:"lastRuntimeSignalAt,omitempty"`
	LastRuntimeMetricAt    *time.Time `json:"lastRuntimeMetricAt,omitempty"`
	LastFalcoEventAt       *time.Time `json:"lastFalcoEventAt,omitempty"`
	RuntimeFreshnessMinute int64      `json:"runtimeFreshnessMinutes"`
	Message                string     `json:"message,omitempty"`
}

type EndpointDataSource struct {
	Path   string `json:"path"`
	Source string `json:"source"` // "real" | "stub" | "placeholder"
	Agent  string `json:"agent"`  // which agent produces data, or ""
	Note   string `json:"note,omitempty"`
}

// DashboardDataIntegrity returns cross-checks and endpoint inventory for dashboard data integrity.
// GET /health/dashboard-data-integrity
func DashboardDataIntegrity(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		resp := DashboardDataIntegrityResponse{
			Timestamp: time.Now(),
			Alerts:    []string{},
			Endpoints: endpointInventory(),
		}

		fail := func(err error, code, message string) bool {
			if err == nil {
				return false
			}
			respondDataUnavailable(c, code, message)
			return true
		}

		for _, required := range []struct {
			table   string
			code    string
			message string
		}{
			{"agents", "dashboard_integrity_agents_schema_unavailable", "Dashboard integrity requires the agents schema"},
			{"clusters", "dashboard_integrity_clusters_schema_unavailable", "Dashboard integrity requires the clusters schema"},
			{"pods", "dashboard_integrity_pods_schema_unavailable", "Dashboard integrity requires the pods schema"},
			{"insights", "dashboard_integrity_insights_schema_unavailable", "Dashboard integrity requires the insights schema"},
		} {
			if !db.Migrator().HasTable(required.table) {
				respondSchemaUnavailable(c, required.code, required.message)
				return
			}
		}

		// Cross-checks: agents count (all ready) vs dashboard-visible data.
		if db.Migrator().HasTable("agents") {
			if fail(db.Model(&models.Agent{}).
				Where("deleted_at IS NULL AND (status = ? OR status IS NULL)", "ready").
				Count(&resp.CrossChecks.ActiveAgentsCount).Error,
				"dashboard_integrity_agents_unavailable", "Dashboard Agent cross-checks could not be loaded") {
				return
			}
			resp.CrossChecks.DashboardAgentsCount = resp.CrossChecks.ActiveAgentsCount
		}

		if db.Migrator().HasTable("clusters") {
			cutoff := time.Now().Add(-7 * 24 * time.Hour)
			if fail(db.Table("clusters").Where("source IN ?", []string{"auto", "env"}).Where("last_sync >= ?", cutoff).
				Count(&resp.CrossChecks.ClustersCount).Error,
				"dashboard_integrity_clusters_unavailable", "Dashboard cluster cross-checks could not be loaded") {
				return
			}
		}
		// Pod/insight cross-checks are required backing data: query failure is not zero.
		if fail(db.Raw(`
			SELECT COUNT(*) FROM (
				SELECT cluster_id, uid
				FROM pods
				WHERE deleted_at IS NULL
				GROUP BY cluster_id, uid
			) scoped_pods
		`).Scan(&resp.CrossChecks.PodsCount).Error,
			"dashboard_integrity_pods_unavailable", "Dashboard Pod cross-checks could not be loaded") {
			return
		}
		if fail(db.Model(&models.Insight{}).Where("insight_type = ? AND deleted_at IS NULL", "vulnerability").
			Count(&resp.CrossChecks.InsightsCount).Error,
			"dashboard_integrity_insights_unavailable", "Dashboard insight cross-checks could not be loaded") {
			return
		}
		if fail(db.Model(&models.Insight{}).
			Where("insight_type = ? AND LOWER(severity) = ? AND deleted_at IS NULL", "vulnerability", "critical").
			Count(&resp.CrossChecks.CriticalInsights).Error,
			"dashboard_integrity_critical_insights_unavailable", "Dashboard critical insight cross-checks could not be loaded") {
			return
		}

		// CVE reference tables are optional when absent, but an existing table that
		// cannot be queried is an availability failure rather than an empty catalog.
		if db.Migrator().HasTable("cves") {
			if fail(db.Table("cves").Count(&resp.CrossChecks.CVEsCount).Error,
				"dashboard_integrity_cves_unavailable", "CVE catalog cross-checks could not be loaded") {
				return
			}
		}
		if db.Migrator().HasTable("package_vulnerabilities") {
			if fail(db.Table("package_vulnerabilities").Count(&resp.CrossChecks.PackageVulnerabilitiesCount).Error,
				"dashboard_integrity_packages_unavailable", "Package vulnerability cross-checks could not be loaded") {
				return
			}
		}
		if db.Migrator().HasTable("osv_packages") {
			if fail(db.Table("osv_packages").Count(&resp.CrossChecks.OsvPackagesCount).Error,
				"dashboard_integrity_osv_unavailable", "OSV cross-checks could not be loaded") {
				return
			}
		}
		if db.Migrator().HasTable("malware_packages") {
			if fail(db.Table("malware_packages").Where("deleted_at IS NULL").Count(&resp.CrossChecks.MalwarePackagesCount).Error,
				"dashboard_integrity_malware_unavailable", "Malware catalog cross-checks could not be loaded") {
				return
			}
		}
		if db.Migrator().HasTable("sboms") {
			if fail(db.Table("sboms").Where("deleted_at IS NULL").Count(&resp.CrossChecks.SbomsCount).Error,
				"dashboard_integrity_sboms_unavailable", "SBOM cross-checks could not be loaded") {
				return
			}
		}
		if db.Migrator().HasTable("sboms") && db.Migrator().HasTable("pods") {
			if fail(db.Raw(`
				SELECT COUNT(*) FROM pods p
				WHERE p.deleted_at IS NULL
				  AND COALESCE(TRIM(p.uid), '') <> ''
				  AND NOT EXISTS (
					SELECT 1 FROM sboms s
					WHERE s.deleted_at IS NULL
					  AND s.cluster_id = p.cluster_id
					  AND TRIM(s.pod_uid) = TRIM(p.uid)
				  )
			`).Scan(&resp.CrossChecks.PodsMissingSbom).Error,
				"dashboard_integrity_sbom_coverage_unavailable", "SBOM coverage cross-checks could not be loaded") {
				return
			}
		}

		// Alerts: data exists but no agents
		if resp.CrossChecks.InsightsCount > 0 && resp.CrossChecks.ActiveAgentsCount == 0 {
			resp.Alerts = append(resp.Alerts, "data_exists_no_agents: insights exist but no agents in agents table")
		}
		if resp.CrossChecks.PodsCount > 0 && resp.CrossChecks.ActiveAgentsCount == 0 {
			resp.Alerts = append(resp.Alerts, "data_exists_no_agents: pods exist but no agents in agents table")
		}
		// Alert: CVE tables exist but reference data not loaded (run sync + load-cve-data.sh)
		if db.Migrator().HasTable("cves") && resp.CrossChecks.CVEsCount == 0 {
			resp.Alerts = append(resp.Alerts, "cve_reference_empty: cves table is empty; run scripts/utils/sync-package-vulnerability-source.sh then scripts/utils/load-cve-data.sh")
		}
		if db.Migrator().HasTable("osv_packages") && resp.CrossChecks.OsvPackagesCount == 0 {
			resp.Alerts = append(resp.Alerts, "osv_mirror_empty: osv_packages has no rows; CVE matching will be degraded until OSV bootstrap/loader runs")
		}
		if db.Migrator().HasTable("malware_packages") && resp.CrossChecks.MalwarePackagesCount == 0 {
			resp.Alerts = append(resp.Alerts, "malware_catalog_empty: malware_packages has no rows; supply-chain malware matching is inactive")
		}
		if resp.CrossChecks.PodsCount > 0 && resp.CrossChecks.SbomsCount == 0 {
			resp.Alerts = append(resp.Alerts, "sbom_pipeline_empty: no sboms rows; confirm agent SBOM sync and inventory path /inventory/sbom")
		} else if resp.CrossChecks.PodsCount > 0 && resp.CrossChecks.PodsMissingSbom > resp.CrossChecks.PodsCount/2 {
			resp.Alerts = append(resp.Alerts, "sbom_coverage_low: majority of pods have no SBOM row; check agent SBOM scanner and pod eligibility (distroless/heuristic)")
		}

		catalogHealth, err := buildCatalogHealth(db, resp.CrossChecks)
		if err != nil {
			respondDataUnavailable(c, "dashboard_catalog_health_unavailable", "Dashboard catalog health could not be loaded")
			return
		}
		resp.CatalogHealth = catalogHealth
		runtimeHealth, err := buildRuntimeHealth(db)
		if err != nil {
			respondDataUnavailable(c, "dashboard_runtime_health_unavailable", "Dashboard runtime health could not be loaded")
			return
		}
		resp.RuntimeHealth = runtimeHealth
		resp.Alerts = append(resp.Alerts, catalogHealthAlerts(resp.CatalogHealth)...)
		resp.Alerts = append(resp.Alerts, runtimeHealthAlerts(resp.RuntimeHealth)...)

		c.JSON(http.StatusOK, resp)
	}
}

type runtimeEventTimeRow struct {
	ObservedAt *time.Time `gorm:"column:observed_at"`
	IngestedAt *time.Time `gorm:"column:ingested_at"`
	CreatedAt  *time.Time `gorm:"column:created_at"`
}

func latestRuntimeEventTime(q *gorm.DB) (*time.Time, error) {
	var row runtimeEventTimeRow
	err := q.
		Select("observed_at, ingested_at, created_at").
		Where("COALESCE(observed_at, ingested_at, created_at) IS NOT NULL").
		Order("COALESCE(observed_at, ingested_at, created_at) DESC").
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	for _, candidate := range []*time.Time{row.ObservedAt, row.IngestedAt, row.CreatedAt} {
		if candidate != nil && !candidate.IsZero() {
			t := *candidate
			return &t, nil
		}
	}
	return nil, nil
}

type runtimeSignalTimeRow struct {
	LastSeenAt *string    `gorm:"column:last_seen_at"`
	CreatedAt  *time.Time `gorm:"column:created_at"`
}

func latestRuntimeSignalTime(q *gorm.DB) (*time.Time, error) {
	var row runtimeSignalTimeRow
	err := q.
		Select("last_seen_at, created_at").
		Where("COALESCE(last_seen_at, created_at) IS NOT NULL").
		Order("COALESCE(last_seen_at, created_at) DESC").
		Limit(1).
		Scan(&row).Error
	if err != nil {
		return nil, err
	}
	if row.LastSeenAt != nil && strings.TrimSpace(*row.LastSeenAt) != "" {
		t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(*row.LastSeenAt))
		if err != nil {
			return nil, err
		}
		return &t, nil
	}
	if row.CreatedAt != nil && !row.CreatedAt.IsZero() {
		t := *row.CreatedAt
		return &t, nil
	}
	return nil, nil
}

func buildCatalogHealth(db *gorm.DB, checks CrossChecks) (CatalogHealth, error) {
	health := CatalogHealth{
		Status:                      "healthy",
		CVEsCount:                   checks.CVEsCount,
		PackageVulnerabilitiesCount: checks.PackageVulnerabilitiesCount,
		OsvPackagesCount:            checks.OsvPackagesCount,
		MalwarePackagesCount:        checks.MalwarePackagesCount,
	}

	if db.Migrator().HasTable("mirror_state") {
		var row models.MirrorState
		err := db.Where("name = ?", "osv").First(&row).Error
		if err == nil {
			health.MirrorVersion = strconv.FormatInt(row.Version, 10)
			if !row.UpdatedAt.IsZero() {
				t := row.UpdatedAt
				health.MirrorUpdatedAt = &t
			}
		} else if err != gorm.ErrRecordNotFound {
			return health, err
		}
	}

	if db.Migrator().HasTable("cves") {
		t, err := latestQueryTime(db.Table("cves"), "updated_at")
		if err != nil {
			return health, err
		}
		health.LastCVEUpdatedAt = t
	}
	if db.Migrator().HasTable("package_vulnerabilities") {
		t, err := latestQueryTime(db.Table("package_vulnerabilities"), "updated_at")
		if err != nil {
			return health, err
		}
		health.LastPackageVulnerabilityUpdate = t
	}
	if db.Migrator().HasTable("malware_packages") {
		t, err := latestQueryTime(db.Table("malware_packages").Where("deleted_at IS NULL"), "updated_at")
		if err != nil {
			return health, err
		}
		health.LastMalwareUpdatedAt = t
	}
	if db.Migrator().HasTable("malware_feed_sync_runs") {
		var latest models.MalwareFeedSyncRun
		err := db.Order("completed_at DESC, id DESC").First(&latest).Error
		if err == nil && latest.ID != 0 {
			health.LastMalwareFeedSyncStatus = latest.Status
			if !latest.CompletedAt.IsZero() {
				t := latest.CompletedAt
				health.LastMalwareFeedSyncAt = &t
			}
		} else if err != nil && err != gorm.ErrRecordNotFound {
			return health, err
		}
	}
	if db.Migrator().HasTable("catalog_generations") {
		var active models.CatalogGeneration
		err := db.
			Where("catalog_type = ? AND status = ?", "cve", "active").
			Order("activated_at DESC, id DESC").
			First(&active).Error
		if err == nil && active.ID != 0 {
			health.ActiveCatalogGenerationID = active.ID
			health.ActiveCatalogGenerationStatus = active.Status
			health.ActiveCatalogSourceDigest = active.SourceDigest
			if active.ActivatedAt != nil && !active.ActivatedAt.IsZero() {
				t := *active.ActivatedAt
				health.ActiveCatalogActivatedAt = &t
			}
		} else if err != nil && err != gorm.ErrRecordNotFound {
			return health, err
		}

		var malwareGen models.CatalogGeneration
		err = db.
			Where("catalog_type = ? AND status = ?", "malware", "active").
			Order("activated_at DESC, id DESC").
			First(&malwareGen).Error
		if err == nil && malwareGen.ID != 0 {
			health.ActiveMalwareGenerationID = malwareGen.ID
			health.ActiveMalwareGenerationStatus = malwareGen.Status
			health.ActiveMalwareSourceDigest = malwareGen.SourceDigest
			if malwareGen.ActivatedAt != nil && !malwareGen.ActivatedAt.IsZero() {
				t := *malwareGen.ActivatedAt
				health.ActiveMalwareActivatedAt = &t
			}
		} else if err != nil && err != gorm.ErrRecordNotFound {
			return health, err
		}
	}

	if db.Migrator().HasTable("sboms") && db.Migrator().HasTable("pods") {
		if err := db.Raw(`
			SELECT COUNT(*) FROM sboms s
			INNER JOIN pods p ON p.cluster_id = s.cluster_id AND p.uid = s.pod_uid AND p.deleted_at IS NULL
			WHERE s.deleted_at IS NULL
		`).Scan(&health.ActiveSBOMs).Error; err != nil {
			return health, err
		}
		if err := db.Raw(`
			SELECT COUNT(*) FROM sboms s
			LEFT JOIN pods p ON p.cluster_id = s.cluster_id AND p.uid = s.pod_uid AND p.deleted_at IS NULL
			WHERE s.deleted_at IS NULL AND p.id IS NULL
		`).Scan(&health.StaleSBOMs).Error; err != nil {
			return health, err
		}
	}

	if health.MirrorVersion != "" && db.Migrator().HasTable("sbom_match_runs") && db.Migrator().HasTable("sboms") && db.Migrator().HasTable("pods") {
		if err := db.Raw(`
			SELECT COUNT(DISTINCT s.id)
			FROM sboms s
			INNER JOIN pods p ON p.cluster_id = s.cluster_id AND p.uid = s.pod_uid AND p.deleted_at IS NULL
			INNER JOIN sbom_match_runs r ON r.sbom_id = s.id
				AND r.version = s.version
				AND r.mirror_version = ?
				AND r.status = 'succeeded'
			WHERE s.deleted_at IS NULL
		`, health.MirrorVersion).Scan(&health.ActiveSBOMsMatchedMirror).Error; err != nil {
			return health, err
		}

		if health.ActiveSBOMs > health.ActiveSBOMsMatchedMirror {
			health.ActiveSBOMsMissingMirrorMatch = health.ActiveSBOMs - health.ActiveSBOMsMatchedMirror
		}

		if err := db.Model(&models.SBOMMatchRun{}).Where("mirror_version = ? AND status = ?", health.MirrorVersion, "succeeded").Count(&health.CurrentMirrorSucceededRuns).Error; err != nil {
			return health, err
		}
		if err := db.Model(&models.SBOMMatchRun{}).Where("mirror_version = ? AND status = ?", health.MirrorVersion, "failed").Count(&health.CurrentMirrorFailedRuns).Error; err != nil {
			return health, err
		}
		if err := db.Model(&models.SBOMMatchRun{}).Where("mirror_version = ? AND status = ?", health.MirrorVersion, "running").Count(&health.CurrentMirrorRunningRuns).Error; err != nil {
			return health, err
		}
	}

	if health.ActiveCatalogGenerationID > 0 && db.Migrator().HasTable("sbom_match_runs") && db.Migrator().HasTable("sboms") && db.Migrator().HasTable("pods") {
		if err := db.Raw(`
			SELECT COUNT(DISTINCT s.id)
			FROM sboms s
			INNER JOIN pods p ON p.cluster_id = s.cluster_id AND p.uid = s.pod_uid AND p.deleted_at IS NULL
			INNER JOIN sbom_match_runs r ON r.sbom_id = s.id
				AND r.version = s.version
				AND r.catalog_generation_id = ?
				AND r.status = 'succeeded'
			WHERE s.deleted_at IS NULL
		`, health.ActiveCatalogGenerationID).Scan(&health.ActiveSBOMsMatchedGeneration).Error; err != nil {
			return health, err
		}

		if health.ActiveSBOMs > health.ActiveSBOMsMatchedGeneration {
			health.ActiveSBOMsMissingGenerationMatch = health.ActiveSBOMs - health.ActiveSBOMsMatchedGeneration
		}

		if err := db.Model(&models.SBOMMatchRun{}).Where("catalog_generation_id = ? AND status = ?", health.ActiveCatalogGenerationID, "succeeded").Count(&health.CurrentGenerationSucceededRuns).Error; err != nil {
			return health, err
		}
		if err := db.Model(&models.SBOMMatchRun{}).Where("catalog_generation_id = ? AND status = ?", health.ActiveCatalogGenerationID, "failed").Count(&health.CurrentGenerationFailedRuns).Error; err != nil {
			return health, err
		}
		if err := db.Model(&models.SBOMMatchRun{}).Where("catalog_generation_id = ? AND status = ?", health.ActiveCatalogGenerationID, "running").Count(&health.CurrentGenerationRunningRuns).Error; err != nil {
			return health, err
		}
	}

	if db.Migrator().HasTable("cve_matches") && db.Migrator().HasTable("sboms") && db.Migrator().HasTable("pods") {
		if err := db.Raw(`
			SELECT COUNT(cm.id)
			FROM cve_matches cm
			INNER JOIN sboms s ON s.id = cm.sbom_id AND s.deleted_at IS NULL
			INNER JOIN pods p ON p.cluster_id = s.cluster_id AND p.uid = s.pod_uid AND p.deleted_at IS NULL
			WHERE cm.deleted_at IS NULL
		`).Scan(&health.ActivePodCVEMatches).Error; err != nil {
			return health, err
		}
		if err := db.Raw(`
			SELECT COUNT(cm.id)
			FROM cve_matches cm
			INNER JOIN sboms s ON s.id = cm.sbom_id AND s.deleted_at IS NULL
			LEFT JOIN pods p ON p.cluster_id = s.cluster_id AND p.uid = s.pod_uid AND p.deleted_at IS NULL
			WHERE cm.deleted_at IS NULL AND p.id IS NULL
		`).Scan(&health.StalePodCVEMatches).Error; err != nil {
			return health, err
		}
	}

	switch {
	case health.CVEsCount == 0 || health.PackageVulnerabilitiesCount == 0:
		health.Status = "unavailable"
	case health.ActiveCatalogGenerationID > 0 && (health.ActiveSBOMsMissingGenerationMatch > 0 || health.CurrentGenerationFailedRuns > 0):
		health.Status = "stale"
	case health.ActiveCatalogGenerationID == 0 && (health.ActiveSBOMsMissingMirrorMatch > 0 || health.CurrentMirrorFailedRuns > 0):
		health.Status = "stale"
	case health.OsvPackagesCount == 0 || health.MalwarePackagesCount == 0 || health.MirrorVersion == "" || health.LastMalwareFeedSyncStatus == "failed" || health.ActiveCatalogGenerationID == 0 || (health.MalwarePackagesCount > 0 && health.ActiveMalwareGenerationID == 0):
		health.Status = "degraded"
	default:
		health.Status = "healthy"
	}

	return health, nil
}

func buildRuntimeHealth(db *gorm.DB) (RuntimeHealth, error) {
	health := RuntimeHealth{
		Status:                 "unavailable",
		Source:                 "db-derived",
		FalcoStatus:            "unavailable",
		RuntimeFreshnessMinute: -1,
		Message:                "Runtime ingest tables are not populated.",
	}

	if db.Migrator().HasTable("runtime_events") {
		if err := db.Table("runtime_events").Count(&health.RuntimeEventsCount).Error; err != nil {
			return health, err
		}
		lastEvent, err := latestRuntimeEventTime(db.Table("runtime_events"))
		if err != nil {
			return health, err
		}
		health.LastRuntimeEventAt = lastEvent

		falcoQuery := db.Table("runtime_events").
			Where("LOWER(COALESCE(source_kind, runtime, '')) = ? OR LOWER(COALESCE(runtime, source_kind, '')) = ?", "falco", "falco")
		if err := falcoQuery.Count(&health.FalcoEventsCount).Error; err != nil {
			return health, err
		}
		lastFalco, err := latestRuntimeEventTime(falcoQuery)
		if err != nil {
			return health, err
		}
		health.LastFalcoEventAt = lastFalco
	}

	if db.Migrator().HasTable("runtime_signals") {
		if err := db.Table("runtime_signals").Count(&health.RuntimeSignalsCount).Error; err != nil {
			return health, err
		}
		lastSignal, err := latestRuntimeSignalTime(db.Table("runtime_signals"))
		if err != nil {
			return health, err
		}
		health.LastRuntimeSignalAt = lastSignal
	}

	if db.Migrator().HasTable("pod_runtime_metrics") {
		if err := db.Table("pod_runtime_metrics").Count(&health.RuntimeMetricsCount).Error; err != nil {
			return health, err
		}
		lastMetric, err := latestQueryTime(db.Table("pod_runtime_metrics"), "last_observed_at")
		if err != nil {
			return health, err
		}
		health.LastRuntimeMetricAt = lastMetric
	}

	last := latestTime(health.LastRuntimeEventAt, health.LastRuntimeSignalAt, health.LastRuntimeMetricAt)
	if last != nil {
		health.RuntimeFreshnessMinute = int64(time.Since(*last).Minutes())
	}

	switch {
	case health.RuntimeEventsCount == 0 && health.RuntimeSignalsCount == 0 && health.RuntimeMetricsCount == 0:
		health.Status = "unavailable"
		health.Message = "No runtime events, semantic signals, or pod runtime metrics are present."
	case health.RuntimeFreshnessMinute >= 0 && health.RuntimeFreshnessMinute > 30:
		health.Status = "degraded"
		health.Message = "Runtime ingest exists, but the latest runtime evidence is stale."
	default:
		health.Status = "healthy"
		health.Message = "Runtime ingest is present in persisted telemetry tables."
	}

	if health.FalcoEventsCount > 0 {
		health.FalcoStatus = "active"
	} else if db.Migrator().HasTable("runtime_events") {
		health.FalcoStatus = "no-events"
	}

	return health, nil
}

func latestTime(values ...*time.Time) *time.Time {
	var latest *time.Time
	for _, value := range values {
		if value == nil || value.IsZero() {
			continue
		}
		if latest == nil || value.After(*latest) {
			t := *value
			latest = &t
		}
	}
	return latest
}

func catalogHealthAlerts(health CatalogHealth) []string {
	alerts := []string{}
	if health.Status == "unavailable" {
		alerts = append(alerts, "catalog_unavailable: CVE reference data is missing or incomplete")
	}
	if health.Status == "stale" {
		alerts = append(alerts, "catalog_stale: active SBOMs are not fully matched against the active CVE catalog")
	}
	if health.MirrorVersion == "" {
		alerts = append(alerts, "catalog_mirror_version_missing: mirror_state row for osv is missing")
	}
	if health.ActiveCatalogGenerationID == 0 {
		alerts = append(alerts, "catalog_generation_missing: cve catalog has no active generation metadata")
	}
	if health.ActiveCatalogGenerationID > 0 && health.ActiveSBOMsMissingGenerationMatch > 0 {
		alerts = append(alerts, "catalog_generation_match_missing: active SBOMs need rematch against the active CVE catalog generation")
	}
	if health.OsvPackagesCount == 0 && health.PackageVulnerabilitiesCount > 0 {
		alerts = append(alerts, "osv_mirror_empty_fallback_active: package_vulnerabilities fallback is required")
	}
	if health.MalwarePackagesCount == 0 {
		alerts = append(alerts, "malware_catalog_empty: malware package matching is inactive")
	}
	if health.MalwarePackagesCount > 0 && health.LastMalwareFeedSyncStatus == "" {
		alerts = append(alerts, "malware_feed_sync_unknown: malware packages exist but feed sync state is missing")
	}
	if health.MalwarePackagesCount > 0 && health.ActiveMalwareGenerationID == 0 {
		alerts = append(alerts, "malware_generation_missing: malware packages exist but no active catalog generation metadata is available")
	}
	if health.LastMalwareFeedSyncStatus == "failed" {
		alerts = append(alerts, "malware_feed_sync_failed: latest malware feed sync failed")
	}
	if health.LastMalwareFeedSyncAt != nil && time.Since(*health.LastMalwareFeedSyncAt) > 24*time.Hour {
		alerts = append(alerts, "malware_feed_stale: latest malware feed sync is older than 24h")
	}
	if health.StaleSBOMs > 0 {
		alerts = append(alerts, "stale_sboms_present: historical SBOMs exist and must be excluded from current-risk views")
	}
	return alerts
}

func runtimeHealthAlerts(health RuntimeHealth) []string {
	alerts := []string{}
	if health.Status == "unavailable" {
		alerts = append(alerts, "runtime_monitor_unavailable: no runtime ingest has been observed")
	} else if health.Status == "degraded" {
		alerts = append(alerts, "runtime_monitor_stale: latest runtime ingest is older than the freshness threshold")
	}
	if health.FalcoStatus == "no-events" {
		alerts = append(alerts, "falco_no_events: no Falco runtime events have been ingested; confirm Falco DaemonSet and agent log reader")
	}
	return alerts
}

func endpointInventory() []EndpointDataSource {
	return []EndpointDataSource{
		{Path: "/api/v1/health/dashboard-data-integrity", Source: "real", Agent: "core", Note: "cross-checks: agents, CVE/OSV/malware/SBOM coverage, alerts"},
		{Path: "/api/v1/dashboard/stats", Source: "real", Agent: "agent (sync)", Note: "clusters/pods/agents/insights from DB"},
		{Path: "/api/v1/inventory/clusters", Source: "real", Agent: "agent (sync)", Note: "clusters from DB, filtered by last_sync"},
		{Path: "/api/v1/agents/status", Source: "real", Agent: "agent (heartbeat)", Note: "agents table"},
		{Path: "/api/v1/risk/insights", Source: "real", Agent: "core (insights)", Note: "insights table"},
		{Path: "/api/v1/inventory/sbom", Source: "real", Agent: "agent (SBOM)", Note: "sbom from agent"},
		{Path: "/api/v1/resources", Source: "real", Agent: "agent (sync)", Note: "resources from sync"},
		{Path: "/api/v1/policy/rules", Source: "real", Agent: "core", Note: "rules from DB"},
		{Path: "/api/v1/audit/logs", Source: "real", Agent: "core", Note: "audit_logs table"},
		{Path: "/api/v1/audit/reports", Source: "real", Agent: "core", Note: "aggregated from audit_logs"},
		{Path: "/api/v1/metrics/system", Source: "real", Agent: "core", Note: "DB aggregates"},
		{Path: "/api/v1/dashboard/metrics/threat-velocity", Source: "real", Agent: "core", Note: "insights by severity/date"},
		{Path: "/api/v1/inventory/pod-capabilities/summary/*", Source: "real", Agent: "agent (PCE)", Note: "pod_capabilities"},
		{Path: "/api/v1/graph/attack-paths/graph", Source: "real", Agent: "core", Note: "graph from DB"},
		{Path: "/api/v1/graph/attack-paths/bundle", Source: "real", Agent: "core", Note: "graph+summary+chains+objectives single pass"},
		{Path: "/api/v1/notifications", Source: "real", Agent: "core", Note: "from notifications table"},
		{Path: "/api/v1/error-logs", Source: "real", Agent: "core", Note: "from error_logs table"},
		// Removed: /api/v1/metrics/workers, /api/v1/metrics/queue (use Prometheus when needed)
		{Path: "/api/v1/cluster/info", Source: "real", Agent: "agent (sync)", Note: "cluster list (infrastructure domain)"},
		{Path: "/api/v1/cluster/:id/nodes", Source: "real", Agent: "agent (sync)", Note: "node names for cluster"},
		{Path: "/api/v1/cluster/certificates/info", Source: "real", Agent: "core", Note: "from CertManager when TLS enabled"},
		{Path: "/api/v1/cluster/certificates/rotation/history", Source: "real", Agent: "core", Note: "empty list until rotation_history table"},
		{Path: "/api/v1/users", Source: "real", Agent: "core", Note: "from users table; admin only when auth enabled"},
	}
}
