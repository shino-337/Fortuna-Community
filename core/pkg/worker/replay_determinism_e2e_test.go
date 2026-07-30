package worker

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/fortuna/core/migrations"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/sbom"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type matchFingerprint struct {
	CVEID          string
	PackageName    string
	PackageVersion string
	Severity       string
	FixedVersion   string
	PURL           string
	CVSS           float32
}

type insightFingerprint struct {
	CVEID             string
	AffectedComponent string
	FixedVersion      string
	Severity          string
	CVSS              float32
	Recommendation    string
	FinalRiskConfidence string
	Degraded            bool
}

type replayScenarioSnapshot struct {
	LatestEventTS   int64
	LatestEventID   string
	MatchRunsCount  int64
	CVEMatchesFP    string
	InsightsFP      string
	CVEMatchesCount int64
	InsightsCount   int64
}

func newDeterministicReplayWorkerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(
		sqlite.Open(":memory:"),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	// Schema: expand beyond replay_contract_test to enable end-to-end matching persistence + insights.
	if err := db.AutoMigrate(
		&models.Cluster{},
		&models.Pod{},
		&models.SBOM{},
		&models.SBOMComponent{},
		&models.SBOMMatchRun{},
		&models.CVE{},
		&models.PackageVulnerability{},
		&models.CVEMatch{},
		&models.Insight{},
		&models.RiskScore{},
		&models.ExceptionPolicy{},
		&models.MirrorState{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := migrations.Migration086_AddSBOMProcessingState(db); err != nil {
		t.Fatalf("migration 086: %v", err)
	}

	// Worker persistMatches uses INSERT ... ON CONFLICT(sbom_id, package_name, cve_id) DO NOTHING.
	// SQLite requires a matching UNIQUE index.
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_cve_matches_unique_replay_test
		ON cve_matches(sbom_id, package_name, cve_id);
	`).Error; err != nil {
		t.Fatalf("create cve_matches unique index: %v", err)
	}

	// InsightManager batch upsert uses:
	// ON CONFLICT (resource_uid, cve_id, insight_type)
	// WHERE deleted_at IS NULL
	// SQLite requires a matching unique constraint/index.
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_insights_unique_replay_partial
		ON insights(resource_uid, cve_id, insight_type)
		WHERE deleted_at IS NULL;
	`).Error; err != nil {
		t.Fatalf("create insights partial unique index: %v", err)
	}
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_insights_unique_replay_all
		ON insights(resource_uid, cve_id, insight_type);
	`).Error; err != nil {
		t.Fatalf("create insights unique index: %v", err)
	}

	return db
}

func seedDeterministicFixtures(t *testing.T, db *gorm.DB, sb *models.SBOM) {
	t.Helper()

	// Seed pod context so insights + risk scoring have required ResourceUID -> pod lookup.
	clusterID := "cluster-1"
	if err := db.Create(&models.Cluster{
		ID:     clusterID,
		Name:   "c1",
		Source: "test",
		// Other fields are optional in tests.
	}).Error; err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	if err := db.Create(&models.Pod{
		ClusterID:     clusterID,
		Name:          sb.PodName,
		Namespace:     sb.Namespace,
		ServiceAccount: "sa-1",
		UID:            sb.PodUID,
		Containers:     "[]",
		ImageDigests:   "[]",
		Phase:          "Running",
	}).Error; err != nil {
		t.Fatalf("create pod: %v", err)
	}

	// Single component + single CVE that is guaranteed to match deterministically.
	component := models.SBOMComponent{
		SBOMID:           sb.ID,
		ComponentType:    "library",
		ComponentName:    "openssl",
		ComponentVersion: "1.0",
		PURL:             "pkg:deb/debian/openssl@1.0",
		TrustLevel:       "high",
		PURLValidated:    true,
		Source:           "test-fixture",
	}
	if err := db.Create(&component).Error; err != nil {
		t.Fatalf("create sbom component: %v", err)
	}

	cve := models.CVE{
		CVEID:          "CVE-TEST-1",
		Severity:      "HIGH",
		CVSSScore:     7.5,
		CVSSVector:    "AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
		Title:         "test",
		Description:   "test",
		Source:        "osv",
		PublishedDate: nil,
		CreatedAt:     time.Now(),
	}
	if err := db.Create(&cve).Error; err != nil {
		t.Fatalf("create cve: %v", err)
	}

	// Debian-like version constraint: vulnerable if installedVersion < 2.0.
	pv := models.PackageVulnerability{
		CVEID:                 cve.CVEID,
		PackageName:           "openssl",
		PackageType:           "deb",
		Ecosystem:             "debian",
		VersionEndExcluding:  "2.0",
		FixedVersion:         "2.0",
		Vendor:                "test",
		Product:               "test",
		CreatedAt:             time.Now(),
	}
	if err := db.Create(&pv).Error; err != nil {
		t.Fatalf("create package vulnerability: %v", err)
	}
}

func upsertOSVMirrorVersion(t *testing.T, db *gorm.DB, v int64) {
	t.Helper()
	var row models.MirrorState
	err := db.Where("name = ?", "osv").First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			db.Create(&models.MirrorState{
				Name:    "osv",
				Version: v,
			})
			return
		}
		db.Create(&models.MirrorState{
			Name:    "osv",
			Version: v,
		})
		return
	}
	if err := db.Model(&row).Update("version", v).Error; err != nil {
		t.Fatalf("update mirror_state: %v", err)
	}
}

func captureSnapshot(t *testing.T, db *gorm.DB, sbomID uint, podUID string) replayScenarioSnapshot {
	t.Helper()

	var latestTS int64
	var latestID string
	if err := db.Raw(`
		SELECT latest_event_ts, latest_event_id
		FROM sbom_processing_state
		WHERE sbom_id = ?
	`, sbomID).Row().Scan(&latestTS, &latestID); err != nil {
		t.Fatalf("query latest watermark: %v", err)
	}

	var matchRuns int64
	if err := db.Model(&models.SBOMMatchRun{}).Where("sbom_id = ?", sbomID).Count(&matchRuns).Error; err != nil {
		t.Fatalf("count match runs: %v", err)
	}

	var cvematches []models.CVEMatch
	if err := db.Where("sbom_id = ? AND deleted_at IS NULL", sbomID).Find(&cvematches).Error; err != nil {
		t.Fatalf("query cve_matches: %v", err)
	}

	cveMatchesFP := func(matches []models.CVEMatch) string {
		out := make([]matchFingerprint, 0, len(matches))
		for _, m := range matches {
			out = append(out, matchFingerprint{
				CVEID:          m.CVEID,
				PackageName:    m.PackageName,
				PackageVersion: m.PackageVersion,
				Severity:       m.Severity,
				FixedVersion:   m.FixedVersion,
				PURL:           m.PURL,
				CVSS:           m.CVSS,
			})
		}
		sort.Slice(out, func(i, j int) bool {
			a, b := out[i], out[j]
			if a.CVEID != b.CVEID {
				return a.CVEID < b.CVEID
			}
			if a.PackageName != b.PackageName {
				return a.PackageName < b.PackageName
			}
			if a.PackageVersion != b.PackageVersion {
				return a.PackageVersion < b.PackageVersion
			}
			if a.FixedVersion != b.FixedVersion {
				return a.FixedVersion < b.FixedVersion
			}
			return a.PURL < b.PURL
		})

		var b strings.Builder
		for _, m := range out {
			// float32 stringification via fmt keeps deterministic formatting in-memory.
			b.WriteString(fmt.Sprintf("%s|%s|%s|%s|%s|%s|%.1f;", m.CVEID, m.PackageName, m.PackageVersion, m.Severity, m.FixedVersion, m.PURL, m.CVSS))
		}
		return b.String()
	}(cvematches)

	var insights []models.Insight
	if err := db.Where("resource_uid = ? AND insight_type = 'vulnerability' AND deleted_at IS NULL", podUID).Find(&insights).Error; err != nil {
		t.Fatalf("query insights: %v", err)
	}
	insightsFP := func(items []models.Insight) string {
		out := make([]insightFingerprint, 0, len(items))
		for _, i := range items {
			out = append(out, insightFingerprint{
				CVEID:             i.CVEID,
				AffectedComponent: i.AffectedComponent,
				FixedVersion:      i.FixedVersion,
				Severity:          i.Severity,
				CVSS:              i.CVSS,
				Recommendation:    i.Recommendation,
				FinalRiskConfidence: i.FinalRiskConfidence,
				Degraded:            i.Degraded,
			})
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].CVEID != out[j].CVEID {
				return out[i].CVEID < out[j].CVEID
			}
			return out[i].AffectedComponent < out[j].AffectedComponent
		})

		var b strings.Builder
		for _, i := range out {
			b.WriteString(fmt.Sprintf("%s|%s|%s|%s|%.1f|%s|%s|%t;",
				i.CVEID, i.AffectedComponent, i.FixedVersion, i.Severity, i.CVSS, i.Recommendation,
				i.FinalRiskConfidence, i.Degraded))
		}
		return b.String()
	}(insights)

	return replayScenarioSnapshot{
		LatestEventTS:    latestTS,
		LatestEventID:    latestID,
		MatchRunsCount:   matchRuns,
		CVEMatchesFP:     cveMatchesFP,
		InsightsFP:       insightsFP,
		CVEMatchesCount:  int64(len(cvematches)),
		InsightsCount:    int64(len(insights)),
	}
}

func runScenario(t *testing.T, mirrorAfterFirst int64, order string) (replayScenarioSnapshot, replayScenarioSnapshot) {
	t.Helper()

	db := newDeterministicReplayWorkerTestDB(t)
	sb := seedFinalizedSBOM(t, db)
	seedDeterministicFixtures(t, db, &sb)
	w := NewCVEMatcherWorker(nil, db, nil)

	// Events: same SBOM content but different ingest timestamps.
	evNew := sbom.SBOMCreatedEvent{
		Type:            "sbom.created",
		Timestamp:       100,
		EventID:         "ev-new",
		SchemaVersion:   sbom.SBOMCreatedEventSchemaVersion,
		SBOMID:          sb.ID,
		ImageDigest:    sb.ImageDigest,
		ClusterID:      "cluster-1",
		PodUID:         sb.PodUID,
		PodName:        sb.PodName,
		PodNamespace:   sb.Namespace,
		ContainerName:  sb.ContainerName,
		ContainerImage: sb.ImageName + ":" + sb.ImageTag,
	}
	evOld := sbom.SBOMCreatedEvent{
		Type:            "sbom.created",
		Timestamp:       90,
		EventID:         "ev-old",
		SchemaVersion:   sbom.SBOMCreatedEventSchemaVersion,
		SBOMID:          sb.ID,
		ImageDigest:    sb.ImageDigest,
		ClusterID:      "cluster-1",
		PodUID:         sb.PodUID,
		PodName:        sb.PodName,
		PodNamespace:   sb.Namespace,
		ContainerName:  sb.ContainerName,
		ContainerImage: sb.ImageName + ":" + sb.ImageTag,
	}

	ctx := context.Background()
	_ = ctx // kept for parity with other helper patterns

	// mirror version is a knob to force EnsureMatchRun to create a new run on the second processed event.
	// For deterministic replay proof:
	// - new-first: ts=90 should be stale and skipped (watermark-only would be insufficient)
	// - old-first: ts=90 processes, ts=100 processes, then final output must match new-first final.
	//
	// Mirror versions:
	// - "new" event uses mirror=10
	// - "old" event uses mirror=mirrorAfterFirst (usually 11)
	switch order {
	case "new-first":
		upsertOSVMirrorVersion(t, db, 10)
		runWorkerEvent(t, w, evNew)
		afterFirst := captureSnapshot(t, db, sb.ID, sb.PodUID)

		upsertOSVMirrorVersion(t, db, mirrorAfterFirst)
		runWorkerEvent(t, w, evOld)
		final := captureSnapshot(t, db, sb.ID, sb.PodUID)
		return afterFirst, final

	case "old-first":
		upsertOSVMirrorVersion(t, db, mirrorAfterFirst)
		runWorkerEvent(t, w, evOld)

		// Change mirror version before the newest event to force a second matcher run.
		upsertOSVMirrorVersion(t, db, 10)
		runWorkerEvent(t, w, evNew)
		final := captureSnapshot(t, db, sb.ID, sb.PodUID)

		// For old-first we don't need "afterFirst" beyond sanity checks;
		// still return latest-new snapshot as "afterFirst" to simplify comparisons.
		afterFirst := final
		return afterFirst, final

	default:
		t.Fatalf("unknown scenario order: %s", order)
		return replayScenarioSnapshot{}, replayScenarioSnapshot{}
	}
}

func TestE2E_IngestionReplay_DeterministicOutput(t *testing.T) {
	// new-first: expect ts=90 stale -> no additional match/DB drift.
	afterNew, finalNewFirst := runScenario(t, 11, "new-first")
	require.Equal(t, int64(100), finalNewFirst.LatestEventTS)
	require.Equal(t, "ev-new", finalNewFirst.LatestEventID)
	require.Equal(t, afterNew.CVEMatchesFP, finalNewFirst.CVEMatchesFP)
	require.Equal(t, afterNew.InsightsFP, finalNewFirst.InsightsFP)
	require.Equal(t, afterNew.MatchRunsCount, finalNewFirst.MatchRunsCount)
	require.Equal(t, int64(1), finalNewFirst.CVEMatchesCount)
	require.Equal(t, int64(1), finalNewFirst.InsightsCount)

	// old-first: older event processed first, then newest event processes again (forced via mirror version).
	_, finalOldFirst := runScenario(t, 11, "old-first")
	require.Equal(t, int64(100), finalOldFirst.LatestEventTS)
	require.Equal(t, "ev-new", finalOldFirst.LatestEventID)

	// Deterministic output invariant: final state should match new-first final.
	require.Equal(t, finalNewFirst.CVEMatchesFP, finalOldFirst.CVEMatchesFP)
	require.Equal(t, finalNewFirst.InsightsFP, finalOldFirst.InsightsFP)

	// Side-effect invariant: cve_matches / insights must not duplicate, even if matcher runs twice.
	require.Equal(t, int64(1), finalOldFirst.CVEMatchesCount)
	require.Equal(t, int64(1), finalOldFirst.InsightsCount)
	require.Equal(t, int64(2), finalOldFirst.MatchRunsCount)
}

