package worker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/fortuna/core/pkg/riskengine"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func resolutionFixture(t *testing.T) (*gorm.DB, *InsightStatusUpdater, models.Insight) {
	t.Helper()
	db := reconciliationTestDB(t)
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
tags: ["resource-kind:Role"]
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rule.yaml"), []byte(rule), 0600))
	ye, err := riskengine.NewYAMLEngine(db, dir)
	require.NoError(t, err)
	row := models.Role{ClusterID: "a", UID: "same", Name: "role", Namespace: "ns", Rules: "[]"}
	require.NoError(t, db.Create(&row).Error)
	f := models.Insight{ClusterID: "a", ResourceType: "Role", ResourceUID: "same", ResourceName: "role", ResourceNamespace: "ns", CVEID: "static-role", Title: "Static role", InsightType: "rbac", Status: "active", Severity: "high", DetectedAt: time.Now().Add(-time.Minute)}
	require.NoError(t, db.Create(&f).Error)
	return db, &InsightStatusUpdater{db: db, yamlEngine: ye, riskEngine: ye.Engine}, f
}

func TestResolutionEvidenceBoundaries(t *testing.T) {
	for _, tc := range []string{"fresh-static", "foreign-duplicate", "ambiguous", "stale", "future", "missing", "foreign-cluster", "unknown-owner", "malformed", "null-rules", "incomplete-rules", "before-finding", "runtime", "collection-error"} {
		t.Run(tc, func(t *testing.T) {
			db, u, f := resolutionFixture(t)
			switch tc {
			case "foreign-duplicate":
				require.NoError(t, db.Create(&models.Role{ClusterID: "b", UID: f.ResourceUID, Name: "foreign", Namespace: "ns", Rules: `[{"verbs":["*"],"resources":["*"],"apiGroups":["*"]}]`}).Error)
			case "ambiguous":
				require.NoError(t, db.Create(&models.Role{ClusterID: "a", UID: f.ResourceUID, Name: "duplicate", Namespace: "ns", Rules: "[]"}).Error)
			case "stale":
				require.NoError(t, db.Model(&models.Role{}).Where("uid = ?", f.ResourceUID).UpdateColumn("updated_at", time.Now().Add(-time.Hour)).Error)
			case "future":
				require.NoError(t, db.Model(&models.Role{}).Where("uid = ?", f.ResourceUID).UpdateColumn("updated_at", time.Now().Add(time.Hour)).Error)
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
			if tc == "fresh-static" || tc == "foreign-duplicate" {
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
