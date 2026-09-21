package reconciler

import (
	"context"
	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestSBOMReconcilePreservesActiveAndUnresolvedOwnership(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	require.NoError(t, db.AutoMigrate(&models.Pod{}, &models.SBOM{}, &models.AuditLog{}))
	a := models.Pod{ClusterID: "a", UID: "same", Name: "app", Namespace: "ns"}
	b := models.Pod{ClusterID: "b", UID: "same", Name: "app", Namespace: "ns"}
	require.NoError(t, db.Create(&a).Error)
	require.NoError(t, db.Create(&b).Error)
	require.NoError(t, db.Delete(&b).Error)
	for _, cluster := range []string{"a", "b", ""} {
		require.NoError(t, db.Create(&models.SBOM{ClusterID: cluster, PodUID: "same", CreatedAt: time.Now().Add(-48 * time.Hour)}).Error)
	}
	r := NewSBOMReconciler(db, time.Hour)
	stats := &ReconciliationStats{}
	require.NoError(t, r.cleanupOrphanedSBOMs(context.Background(), stats))
	var rows []models.SBOM
	require.NoError(t, db.Find(&rows).Error)
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.NotEqual(t, "b", row.ClusterID)
	}
	require.Equal(t, 1, stats.DeletedSBOMs)
}
