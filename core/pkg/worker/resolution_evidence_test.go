package worker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/inventoryevidence"
	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/riskengine"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func resolutionFixture(t *testing.T) (*gorm.DB, *InsightStatusUpdater, models.Insight) {
	t.Helper()
	db := reconciliationTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.InventoryCollection{}))
	dir := t.TempDir()
	rule := `id: static-role
name: Static role
category: rbac
severity: high
enabled: true
base_score: 5
conditions:
  - type: expression
    expression: "object.rules.exists(r, '*' in r.verbs)"
tags: ["resource-kind:Role", "resource-kind:ClusterRole"]
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rule.yaml"), []byte(rule), 0600))
	ye, err := riskengine.NewYAMLEngine(db, dir)
	require.NoError(t, err)
	row := models.Role{ClusterID: "a", UID: "same", Name: "role", Namespace: "ns", Rules: "[]"}
	require.NoError(t, db.Create(&row).Error)
	f := models.Insight{ClusterID: "a", ResourceType: "Role", ResourceUID: "same", ResourceName: "role", ResourceNamespace: "ns", CVEID: "static-role", Title: "Static role", InsightType: "rbac", Status: "active", Severity: "high", DetectedAt: time.Now().Add(-time.Minute)}
	require.NoError(t, db.Create(&f).Error)
	digest, err := inventoryevidence.RoleDigest("a", "Role", "same", "role", "ns", "[]")
	require.NoError(t, err)
	hashes, _ := json.Marshal(map[string]string{inventoryevidence.Key("Role", "same"): digest})
	now := time.Now().UTC()
	kindTimes, _ := json.Marshal(map[string]time.Time{"roles": now, "clusterRoles": now})
	require.NoError(t, db.Create(&models.InventoryCollection{ClusterID: "a", AgentID: "agent-a", Status: "complete", StartedAt: now.Add(-time.Second), ObservedAt: now, KindObservedAt: string(kindTimes), RoleDigests: string(hashes)}).Error)
	return db, &InsightStatusUpdater{db: db, yamlEngine: ye, riskEngine: ye.Engine}, f
}

func TestResolutionEvidenceBoundaries(t *testing.T) {
	for _, tc := range []string{"fresh-static", "fresh-clusterrole", "old-snapshot-fresh-observation", "missing-receipt", "failed-receipt", "legacy-receipt", "snapshot-not-observed", "foreign-duplicate", "ambiguous", "stale", "future", "missing", "foreign-cluster", "unknown-owner", "malformed", "null-rules", "incomplete-rules", "before-finding", "kind-before-finding", "missing-kind-time", "runtime", "collection-error"} {
		t.Run(tc, func(t *testing.T) {
			db, u, f := resolutionFixture(t)
			switch tc {
			case "fresh-clusterrole":
				require.NoError(t, db.Create(&models.ClusterRole{ClusterID: "a", UID: f.ResourceUID, Name: "role", Rules: "[]"}).Error)
				require.NoError(t, db.Model(&f).UpdateColumn("resource_type", "ClusterRole").Error)
				digest, err := inventoryevidence.RoleDigest("a", "ClusterRole", f.ResourceUID, "role", "", "[]")
				require.NoError(t, err)
				hashes, _ := json.Marshal(map[string]string{inventoryevidence.Key("ClusterRole", f.ResourceUID): digest})
				require.NoError(t, db.Model(&models.InventoryCollection{}).Where("cluster_id = ?", "a").Updates(map[string]interface{}{"namespace": "ns", "role_digests": string(hashes)}).Error)
			case "old-snapshot-fresh-observation":
				require.NoError(t, db.Model(&models.Role{}).Where("uid = ?", f.ResourceUID).UpdateColumn("updated_at", time.Now().Add(-time.Hour)).Error)
			case "missing-receipt":
				require.NoError(t, db.Where("cluster_id = ?", "a").Delete(&models.InventoryCollection{}).Error)
			case "failed-receipt":
				require.NoError(t, db.Model(&models.InventoryCollection{}).Where("cluster_id = ?", "a").UpdateColumn("status", "failed").Error)
			case "legacy-receipt":
				require.NoError(t, db.Model(&models.InventoryCollection{}).Where("cluster_id = ?", "a").UpdateColumn("agent_id", "").Error)
			case "snapshot-not-observed":
				require.NoError(t, db.Model(&models.Role{}).Where("uid = ?", f.ResourceUID).UpdateColumn("name", "changed").Error)
			case "foreign-duplicate":
				require.NoError(t, db.Create(&models.Role{ClusterID: "b", UID: f.ResourceUID, Name: "foreign", Namespace: "ns", Rules: `[{"verbs":["*"],"resources":["*"],"apiGroups":["*"]}]`}).Error)
			case "ambiguous":
				require.NoError(t, db.Create(&models.Role{ClusterID: "a", UID: f.ResourceUID, Name: "duplicate", Namespace: "ns", Rules: "[]"}).Error)
			case "stale":
				require.NoError(t, db.Model(&models.InventoryCollection{}).Where("cluster_id = ?", "a").UpdateColumn("observed_at", time.Now().Add(-time.Hour)).Error)
			case "future":
				require.NoError(t, db.Model(&models.InventoryCollection{}).Where("cluster_id = ?", "a").UpdateColumn("observed_at", time.Now().Add(time.Hour)).Error)
			case "missing":
				require.NoError(t, db.Where("uid = ?", f.ResourceUID).Delete(&models.Role{}).Error)
			case "foreign-cluster":
				require.NoError(t, db.Model(&models.Role{}).Where("uid = ?", f.ResourceUID).UpdateColumn("cluster_id", "b").Error)
			case "unknown-owner":
				require.NoError(t, db.Model(&f).UpdateColumn("cluster_id", "").Error)
			case "malformed":
				require.NoError(t, db.Model(&models.Role{}).Where("uid = ?", f.ResourceUID).UpdateColumn("rules", "{bad").Error)
			case "null-rules":
				require.NoError(t, db.Model(&models.Role{}).Where("uid = ?", f.ResourceUID).UpdateColumn("rules", "null").Error)
			case "incomplete-rules":
				require.NoError(t, db.Model(&models.Role{}).Where("uid = ?", f.ResourceUID).UpdateColumn("rules", "[{}]").Error)
			case "before-finding":
				require.NoError(t, db.Model(&f).UpdateColumn("detected_at", time.Now().Add(time.Minute)).Error)
			case "kind-before-finding":
				var receipt models.InventoryCollection
				require.NoError(t, db.Where("cluster_id = ?", "a").First(&receipt).Error)
				findingAt := receipt.StartedAt.Add(500 * time.Millisecond)
				require.NoError(t, db.Model(&f).UpdateColumn("detected_at", findingAt).Error)
				times, _ := json.Marshal(map[string]time.Time{"roles": receipt.StartedAt.Add(250 * time.Millisecond), "clusterRoles": receipt.ObservedAt})
				require.NoError(t, db.Model(&models.InventoryCollection{}).Where("cluster_id = ?", "a").UpdateColumn("kind_observed_at", string(times)).Error)
			case "missing-kind-time":
				times, _ := json.Marshal(map[string]time.Time{"clusterRoles": time.Now().UTC()})
				require.NoError(t, db.Model(&models.InventoryCollection{}).Where("cluster_id = ?", "a").UpdateColumn("kind_observed_at", string(times)).Error)
			case "runtime":
				require.NoError(t, db.Model(&f).Updates(map[string]any{"resource_type": "Pod", "insight_type": "runtime-behavior"}).Error)
			case "collection-error":
				require.NoError(t, db.Migrator().DropTable(&models.Role{}))
			}
			err := u.UpdateStatusForResolvedRisks(context.Background())
			var actual models.Insight
			require.NoError(t, db.First(&actual, f.ID).Error)
			var audits int64
			require.NoError(t, db.Model(&models.AuditLog{}).Where("action = ?", "auto_resolve").Count(&audits).Error)
			if tc == "fresh-static" || tc == "fresh-clusterrole" || tc == "foreign-duplicate" || tc == "old-snapshot-fresh-observation" {
				require.NoError(t, err)
				require.Equal(t, "resolved", actual.Status)
				require.EqualValues(t, 1, audits)
			} else {
				require.Error(t, err)
				require.Equal(t, "active", actual.Status)
				require.Nil(t, actual.ResolvedAt)
				require.Zero(t, audits)
			}
		})
	}
}

func TestResolutionPreservesConcurrentFindingUpdate(t *testing.T) {
	db, u, f := resolutionFixture(t)
	// The snapshot lock is queried immediately before the conditional status write.
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("test:concurrent_detection", func(tx *gorm.DB) {
		if strings.Contains(tx.Statement.SQL.String(), "FROM `roles`") {
			// UpdateColumns deliberately avoids recursive query callbacks.
			tx.Session(&gorm.Session{NewDB: true}).Model(&models.Insight{}).Where("id = ?", f.ID).UpdateColumn("updated_at", time.Now().Add(time.Second))
		}
	}))
	require.NoError(t, u.UpdateStatusForResolvedRisks(context.Background()))
	var actual models.Insight
	require.NoError(t, db.First(&actual, f.ID).Error)
	require.Equal(t, "active", actual.Status)
	var count int64
	require.NoError(t, db.Model(&models.AuditLog{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestResolutionPreservesConcurrentSnapshotChange(t *testing.T) {
	db, u, f := resolutionFixture(t)
	reads := 0
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("test:concurrent_sync", func(tx *gorm.DB) {
		if strings.Contains(tx.Statement.SQL.String(), "FROM `roles`") {
			reads++
			if reads == 2 {
				tx.Session(&gorm.Session{NewDB: true}).Model(&models.Role{}).Where("cluster_id = ? AND uid = ?", f.ClusterID, f.ResourceUID).UpdateColumn("updated_at", time.Now())
			}
		}
	}))
	require.Error(t, u.UpdateStatusForResolvedRisks(context.Background()))
	var actual models.Insight
	require.NoError(t, db.First(&actual, f.ID).Error)
	require.Equal(t, "active", actual.Status)
	var count int64
	require.NoError(t, db.Model(&models.AuditLog{}).Count(&count).Error)
	require.Zero(t, count)
}
