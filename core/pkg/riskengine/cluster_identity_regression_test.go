package riskengine

import (
	"context"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/resourceidentity"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func clusterInsightDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	configureRiskEngineTestDB(t, db)
	require.NoError(t, db.AutoMigrate(&models.Insight{}, &models.ExceptionPolicy{}, &models.Pod{}, &models.MalwareMatch{}, &models.SBOM{}, &models.SBOMComponent{}))
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX idx_insight_resource_identity ON insights(cluster_id,resource_uid,cve_id,insight_type)").Error)
	return db
}

func TestPodInsightRestorePreservesClusterAndException(t *testing.T) {
	assertPodInsightRestore(t, clusterInsightDB(t))
}

func assertPodInsightRestore(t *testing.T, db *gorm.DB) {
	m := NewInsightManager(db)
	a := &models.Insight{ClusterID: "a", ResourceUID: "same", ResourceType: "Pod", InsightType: "vulnerability", CVEID: "CVE-1", Status: "active", Severity: "high", Title: "a"}
	b := &models.Insight{ClusterID: "b", ResourceUID: "same", ResourceType: "Pod", InsightType: "vulnerability", CVEID: "CVE-1", Status: "resolved", Severity: "low", Title: "b"}
	normalizePodInsightForWrite(a)
	normalizePodInsightForWrite(b)
	require.NoError(t, db.Create(a).Error)
	require.NoError(t, db.Create(b).Error)
	require.NoError(t, db.Delete(a).Error)
	incoming := &models.Insight{ClusterID: "a", ResourceUID: "same", ResourceType: "Pod", InsightType: "vulnerability", CVEID: "CVE-1", Severity: "critical", Title: "new evidence"}
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { _, err := m.createOrUpdatePodInsightTx(tx, incoming); return err }))
	require.Equal(t, a.ID, incoming.ID)
	var restored, foreign models.Insight
	require.NoError(t, db.First(&restored, a.ID).Error)
	require.Equal(t, "active", restored.Status)
	require.False(t, restored.DeletedAt.Valid)
	require.NoError(t, db.First(&foreign, b.ID).Error)
	require.Equal(t, "resolved", foreign.Status)
	require.Equal(t, "b", foreign.Title)
	require.NoError(t, db.Model(&restored).Update("status", "dismissed").Error)
	require.NoError(t, db.Create(&models.ExceptionPolicy{ClusterID: "a", ResourceUID: "same", CVEID: "CVE-1", InsightType: "vulnerability"}).Error)
	require.NoError(t, db.Delete(&restored).Error)
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error { _, err := m.createOrUpdatePodInsightTx(tx, incoming); return err }))
	require.NoError(t, db.Unscoped().First(&restored, a.ID).Error)
	require.Equal(t, "dismissed", restored.Status)
	require.True(t, restored.DeletedAt.Valid, "active exception must prevent restoration")
}

func TestMalwareMaintenanceSeparatesDuplicatePodUID(t *testing.T) {
	db := clusterInsightDB(t)
	for _, cluster := range []string{"a", "b"} {
		require.NoError(t, db.Create(&models.Pod{ClusterID: cluster, UID: "same", Name: "p", Namespace: "ns"}).Error)
		require.NoError(t, db.Create(&models.Insight{ClusterID: cluster, ResourceType: "Pod", ResourceUID: "same", InsightType: "supply_chain_malware", CVEID: "existing", Status: "resolved", AffectedComponent: "evil", Severity: "high", Title: cluster}).Error)
	}
	sb := &models.SBOM{ClusterID: "b", PodUID: "same", Status: "complete", ImageDigest: "sha256:b", ImageName: "image"}
	require.NoError(t, db.Create(sb).Error)
	require.NoError(t, db.Create(&models.MalwareMatch{ClusterID: "b", PodUID: "same", SBOMID: sb.ID, PackageName: "evil", PackageVersion: "1", Reason: "MALWARE", Confidence: 0.95}).Error)
	n, err := ReactivateResolvedMalwareInsightsIfMatchPresent(db)
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
	var a models.Insight
	require.NoError(t, db.Where("cluster_id = ?", "a").First(&a).Error)
	require.Equal(t, "resolved", a.Status)
	nBackfill, err := BackfillSupplyChainMalwareInsightsFromMatches(db)
	require.NoError(t, err)
	require.Equal(t, 1, nBackfill)
	var count int64
	require.NoError(t, db.Model(&models.Insight{}).Where("cluster_id = ?", "a").Count(&count).Error)
	require.EqualValues(t, 1, count)
	nBackfill, err = BackfillSupplyChainMalwareInsightsFromMatches(db)
	require.NoError(t, err)
	require.Zero(t, nBackfill)
	// Unowned evidence cannot activate either cluster.
	require.NoError(t, db.Model(&models.MalwareMatch{}).Where("cluster_id = ?", "b").Update("cluster_id", "").Error)
	require.NoError(t, db.Model(&models.Insight{}).Where("cluster_id = ?", "b").Update("status", "resolved").Error)
	n, err = ReactivateResolvedMalwareInsightsIfMatchPresent(db)
	require.NoError(t, err)
	require.Zero(t, n)
}

func TestRuntimeRescoreDebounceSeparatesClusters(t *testing.T) {
	db := clusterInsightDB(t)
	m := &RuntimeAttackRescoreManager{db: db, enabled: true, debounce: time.Hour, minSeverity: 1, items: map[string]*runtimeAttackItem{}}
	defer func() {
		for _, item := range m.items {
			item.timer.Stop()
		}
	}()
	for _, cluster := range []string{"a", "b", "a", ""} {
		m.Notify(RuntimeEventMeta{ClusterID: cluster, PodUID: "same", Runtime: "falco", SourceRule: "rule"})
	}
	require.Len(t, m.items, 2)
	key, _ := (resourceidentity.Identity{ClusterID: "a", ResourceUID: "same"}).Key()
	require.Equal(t, 2, m.items[key].eventCount)
}

func TestRuntimeEnrichmentUsesExplicitClusterForDuplicateUID(t *testing.T) {
	db := clusterInsightDB(t)
	require.NoError(t, db.AutoMigrate(&models.AssetSecurityState{}, &models.RuntimeSignal{}, &models.PodCapability{}, &models.Role{}, &models.ClusterRole{}, &models.RoleBinding{}, &models.ClusterRoleBinding{}))
	for _, cluster := range []string{"a", "b"} {
		require.NoError(t, db.Create(&models.Pod{ClusterID: cluster, UID: "same", Name: "p", Namespace: "ns", ServiceAccount: "default"}).Error)
	}
	require.NoError(t, db.Create(&models.RuntimeSignal{ClusterID: "b", PodUID: "same", SignalType: "NETWORK_QUEUE_ANOMALY", Count: 3, CreatedAt: time.Now(), Evidence: "{}", Category: "NETWORK"}).Error)
	e := &Engine{db: db}
	for _, cluster := range []string{"a", "b"} {
		data := map[string]interface{}{"cluster_id": cluster, "uid": "same"}
		require.NoError(t, e.enrichPodFortunaContext(context.Background(), data))
		expected := int64(0)
		if cluster == "b" {
			expected = 3
		}
		require.Equal(t, expected, data["fortuna"].(map[string]interface{})["signal_total_24h"])
	}
	require.Error(t, e.enrichPodFortunaContext(context.Background(), map[string]interface{}{"uid": "same"}))
}
