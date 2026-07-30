package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

// Dashboard endpoints (GET /dashboard/stats, GET /dashboard/metrics/*) are aggregate APIs:
// they compose multiple sources and may use cache; not stable for external clients (see docs/02-architecture/API_ARCHITECTURE_RECOMMENDATIONS.md §6.2).

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

		// Cross-checks: agents count (all ready) vs dashboard-visible data
		if db.Migrator().HasTable("agents") {
			db.Model(&models.Agent{}).
				Where("deleted_at IS NULL AND (status = ? OR status IS NULL)", "ready").
				Count(&resp.CrossChecks.ActiveAgentsCount)
			resp.CrossChecks.DashboardAgentsCount = resp.CrossChecks.ActiveAgentsCount
		}

		if db.Migrator().HasTable("clusters") {
			cutoff := time.Now().Add(-7 * 24 * time.Hour)
			db.Table("clusters").Where("source IN ?", []string{"auto", "env"}).Where("last_sync >= ?", cutoff).Count(&resp.CrossChecks.ClustersCount)
		}
		// Pod count: distinct UIDs only (matches dashboard stats and cluster reality)
		db.Raw("SELECT COUNT(DISTINCT uid) FROM pods WHERE deleted_at IS NULL").Scan(&resp.CrossChecks.PodsCount)
		db.Model(&models.Insight{}).Where("insight_type = ? AND deleted_at IS NULL", "vulnerability").Count(&resp.CrossChecks.InsightsCount)
		db.Model(&models.Insight{}).
			Where("insight_type = ? AND LOWER(severity) = ? AND deleted_at IS NULL", "vulnerability", "critical").
			Count(&resp.CrossChecks.CriticalInsights)

		// CVE reference tables (populated by cve-loader Job, not by Core)
		if db.Migrator().HasTable("cves") {
			db.Table("cves").Count(&resp.CrossChecks.CVEsCount)
		}
		if db.Migrator().HasTable("package_vulnerabilities") {
			db.Table("package_vulnerabilities").Count(&resp.CrossChecks.PackageVulnerabilitiesCount)
		}
		if db.Migrator().HasTable("osv_packages") {
			db.Table("osv_packages").Count(&resp.CrossChecks.OsvPackagesCount)
		}
		if db.Migrator().HasTable("malware_packages") {
			db.Table("malware_packages").Where("deleted_at IS NULL").Count(&resp.CrossChecks.MalwarePackagesCount)
		}
		if db.Migrator().HasTable("sboms") {
			db.Table("sboms").Where("deleted_at IS NULL").Count(&resp.CrossChecks.SbomsCount)
		}
		if db.Migrator().HasTable("sboms") && db.Migrator().HasTable("pods") {
			_ = db.Raw(`
				SELECT COUNT(*) FROM pods p
				WHERE p.deleted_at IS NULL
				  AND COALESCE(TRIM(p.uid), '') <> ''
				  AND p.uid NOT IN (
					SELECT DISTINCT TRIM(s.pod_uid) FROM sboms s
					WHERE s.deleted_at IS NULL AND COALESCE(TRIM(s.pod_uid), '') <> ''
				  )
			`).Scan(&resp.CrossChecks.PodsMissingSbom).Error
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

		resp.CatalogHealth = buildCatalogHealth(db, resp.CrossChecks)
		resp.RuntimeHealth = buildRuntimeHealth(db)
		resp.Alerts = append(resp.Alerts, catalogHealthAlerts(resp.CatalogHealth)...)
		resp.Alerts = append(resp.Alerts, runtimeHealthAlerts(resp.RuntimeHealth)...)

		c.JSON(http.StatusOK, resp)
	}
}

func nullTimePtr(nt sql.NullTime) *time.Time {
	if !nt.Valid || nt.Time.IsZero() {
		return nil
	}
	t := nt.Time
	return &t
}

func buildCatalogHealth(db *gorm.DB, checks CrossChecks) CatalogHealth {
	health := CatalogHealth{
		Status:                      "healthy",
		CVEsCount:                   checks.CVEsCount,
		PackageVulnerabilitiesCount: checks.PackageVulnerabilitiesCount,
		OsvPackagesCount:            checks.OsvPackagesCount,
		MalwarePackagesCount:        checks.MalwarePackagesCount,
	}

	if db.Migrator().HasTable("mirror_state") {
		var row models.MirrorState
		if err := db.Where("name = ?", "osv").First(&row).Error; err == nil {
			health.MirrorVersion = strconv.FormatInt(row.Version, 10)
			if !row.UpdatedAt.IsZero() {
				t := row.UpdatedAt
				health.MirrorUpdatedAt = &t
			}
		}
	}

	if db.Migrator().HasTable("cves") {
		var t sql.NullTime
		if err := db.Table("cves").Select("MAX(updated_at)").Scan(&t).Error; err == nil {
			health.LastCVEUpdatedAt = nullTimePtr(t)
		}
	}
	if db.Migrator().HasTable("package_vulnerabilities") {
		var t sql.NullTime
		if err := db.Table("package_vulnerabilities").Select("MAX(updated_at)").Scan(&t).Error; err == nil {
			health.LastPackageVulnerabilityUpdate = nullTimePtr(t)
		}
	}
	if db.Migrator().HasTable("malware_packages") {
		var t sql.NullTime
		if err := db.Table("malware_packages").Where("deleted_at IS NULL").Select("MAX(updated_at)").Scan(&t).Error; err == nil {
			health.LastMalwareUpdatedAt = nullTimePtr(t)
		}
	}
	if db.Migrator().HasTable("malware_feed_sync_runs") {
		var latest models.MalwareFeedSyncRun
		if err := db.Order("completed_at DESC, id DESC").First(&latest).Error; err == nil && latest.ID != 0 {
			health.LastMalwareFeedSyncStatus = latest.Status
			if !latest.CompletedAt.IsZero() {
				t := latest.CompletedAt
				health.LastMalwareFeedSyncAt = &t
			}
		}
	}
	if db.Migrator().HasTable("catalog_generations") {
		var active models.CatalogGeneration
		if err := db.
			Where("catalog_type = ? AND status = ?", "cve", "active").
			Order("activated_at DESC, id DESC").
			First(&active).Error; err == nil && active.ID != 0 {
			health.ActiveCatalogGenerationID = active.ID
			health.ActiveCatalogGenerationStatus = active.Status
			health.ActiveCatalogSourceDigest = active.SourceDigest
			if active.ActivatedAt != nil && !active.ActivatedAt.IsZero() {
				t := *active.ActivatedAt
				health.ActiveCatalogActivatedAt = &t
			}
		}
		var malwareGen models.CatalogGeneration
		if err := db.
			Where("catalog_type = ? AND status = ?", "malware", "active").
			Order("activated_at DESC, id DESC").
			First(&malwareGen).Error; err == nil && malwareGen.ID != 0 {
			health.ActiveMalwareGenerationID = malwareGen.ID
			health.ActiveMalwareGenerationStatus = malwareGen.Status
			health.ActiveMalwareSourceDigest = malwareGen.SourceDigest
			if malwareGen.ActivatedAt != nil && !malwareGen.ActivatedAt.IsZero() {
				t := *malwareGen.ActivatedAt
				health.ActiveMalwareActivatedAt = &t
			}
		}
	}

	if db.Migrator().HasTable("sboms") && db.Migrator().HasTable("pods") {
		db.Raw(`
			SELECT COUNT(*) FROM sboms s
			INNER JOIN pods p ON p.uid = s.pod_uid AND p.deleted_at IS NULL
			WHERE s.deleted_at IS NULL
		`).Scan(&health.ActiveSBOMs)
		db.Raw(`
			SELECT COUNT(*) FROM sboms s
			LEFT JOIN pods p ON p.uid = s.pod_uid AND p.deleted_at IS NULL
			WHERE s.deleted_at IS NULL AND p.id IS NULL
		`).Scan(&health.StaleSBOMs)
	}

	if health.MirrorVersion != "" && db.Migrator().HasTable("sbom_match_runs") && db.Migrator().HasTable("sboms") && db.Migrator().HasTable("pods") {
		db.Raw(`
			SELECT COUNT(DISTINCT s.id)
			FROM sboms s
			INNER JOIN pods p ON p.uid = s.pod_uid AND p.deleted_at IS NULL
			INNER JOIN sbom_match_runs r ON r.sbom_id = s.id
				AND r.version = s.version
				AND r.mirror_version = ?
				AND r.status = 'succeeded'
			WHERE s.deleted_at IS NULL
		`, health.MirrorVersion).Scan(&health.ActiveSBOMsMatchedMirror)

		if health.ActiveSBOMs > health.ActiveSBOMsMatchedMirror {
			health.ActiveSBOMsMissingMirrorMatch = health.ActiveSBOMs - health.ActiveSBOMsMatchedMirror
		}

		db.Model(&models.SBOMMatchRun{}).Where("mirror_version = ? AND status = ?", health.MirrorVersion, "succeeded").Count(&health.CurrentMirrorSucceededRuns)
		db.Model(&models.SBOMMatchRun{}).Where("mirror_version = ? AND status = ?", health.MirrorVersion, "failed").Count(&health.CurrentMirrorFailedRuns)
		db.Model(&models.SBOMMatchRun{}).Where("mirror_version = ? AND status = ?", health.MirrorVersion, "running").Count(&health.CurrentMirrorRunningRuns)
	}

	if health.ActiveCatalogGenerationID > 0 && db.Migrator().HasTable("sbom_match_runs") && db.Migrator().HasTable("sboms") && db.Migrator().HasTable("pods") {
		db.Raw(`
			SELECT COUNT(DISTINCT s.id)
			FROM sboms s
			INNER JOIN pods p ON p.uid = s.pod_uid AND p.deleted_at IS NULL
			INNER JOIN sbom_match_runs r ON r.sbom_id = s.id
				AND r.version = s.version
				AND r.catalog_generation_id = ?
				AND r.status = 'succeeded'
			WHERE s.deleted_at IS NULL
		`, health.ActiveCatalogGenerationID).Scan(&health.ActiveSBOMsMatchedGeneration)

		if health.ActiveSBOMs > health.ActiveSBOMsMatchedGeneration {
			health.ActiveSBOMsMissingGenerationMatch = health.ActiveSBOMs - health.ActiveSBOMsMatchedGeneration
		}

		db.Model(&models.SBOMMatchRun{}).Where("catalog_generation_id = ? AND status = ?", health.ActiveCatalogGenerationID, "succeeded").Count(&health.CurrentGenerationSucceededRuns)
		db.Model(&models.SBOMMatchRun{}).Where("catalog_generation_id = ? AND status = ?", health.ActiveCatalogGenerationID, "failed").Count(&health.CurrentGenerationFailedRuns)
		db.Model(&models.SBOMMatchRun{}).Where("catalog_generation_id = ? AND status = ?", health.ActiveCatalogGenerationID, "running").Count(&health.CurrentGenerationRunningRuns)
	}

	if db.Migrator().HasTable("cve_matches") && db.Migrator().HasTable("sboms") && db.Migrator().HasTable("pods") {
		db.Raw(`
			SELECT COUNT(cm.id)
			FROM cve_matches cm
			INNER JOIN sboms s ON s.id = cm.sbom_id AND s.deleted_at IS NULL
			INNER JOIN pods p ON p.uid = s.pod_uid AND p.deleted_at IS NULL
			WHERE cm.deleted_at IS NULL
		`).Scan(&health.ActivePodCVEMatches)
		db.Raw(`
			SELECT COUNT(cm.id)
			FROM cve_matches cm
			INNER JOIN sboms s ON s.id = cm.sbom_id AND s.deleted_at IS NULL
			LEFT JOIN pods p ON p.uid = s.pod_uid AND p.deleted_at IS NULL
			WHERE cm.deleted_at IS NULL AND p.id IS NULL
		`).Scan(&health.StalePodCVEMatches)
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

	return health
}

func buildRuntimeHealth(db *gorm.DB) RuntimeHealth {
	health := RuntimeHealth{
		Status:                 "unavailable",
		Source:                 "db-derived",
		FalcoStatus:            "unavailable",
		RuntimeFreshnessMinute: -1,
		Message:                "Runtime ingest tables are not populated.",
	}

	if db.Migrator().HasTable("runtime_events") {
		db.Table("runtime_events").Count(&health.RuntimeEventsCount)
		var lastEvent sql.NullTime
		_ = db.Table("runtime_events").Select("MAX(COALESCE(observed_at, ingested_at, created_at))").Scan(&lastEvent).Error
		health.LastRuntimeEventAt = nullTimePtr(lastEvent)

		db.Table("runtime_events").
			Where("LOWER(COALESCE(source_kind, runtime, '')) = ? OR LOWER(COALESCE(runtime, source_kind, '')) = ?", "falco", "falco").
			Count(&health.FalcoEventsCount)
		var lastFalco sql.NullTime
		_ = db.Table("runtime_events").
			Where("LOWER(COALESCE(source_kind, runtime, '')) = ? OR LOWER(COALESCE(runtime, source_kind, '')) = ?", "falco", "falco").
			Select("MAX(COALESCE(observed_at, ingested_at, created_at))").
			Scan(&lastFalco).Error
		health.LastFalcoEventAt = nullTimePtr(lastFalco)
	}

	if db.Migrator().HasTable("runtime_signals") {
		db.Table("runtime_signals").Count(&health.RuntimeSignalsCount)
		var lastSignal sql.NullTime
		_ = db.Table("runtime_signals").Select("MAX(COALESCE(last_seen_at, created_at))").Scan(&lastSignal).Error
		health.LastRuntimeSignalAt = nullTimePtr(lastSignal)
	}

	if db.Migrator().HasTable("pod_runtime_metrics") {
		db.Table("pod_runtime_metrics").Count(&health.RuntimeMetricsCount)
		var lastMetric sql.NullTime
		_ = db.Table("pod_runtime_metrics").Select("MAX(last_observed_at)").Scan(&lastMetric).Error
		health.LastRuntimeMetricAt = nullTimePtr(lastMetric)
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

	return health
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
