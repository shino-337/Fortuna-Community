package sbom

import (
	"context"
	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestPodImageScanRejectsForeignSBOM(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	defer sqlDB.Close()
	require.NoError(t, db.AutoMigrate(&models.SBOM{}, &models.PodImageScan{}))
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_pod_image_scan_identity ON pod_image_scans(cluster_id,pod_uid,container_name)").Error)
	service := NewService(db)
	for _, cluster := range []string{"a", "b"} {
		row := models.SBOM{ClusterID: cluster, PodUID: "same", ContainerName: "app", ImageDigest: "sha256:same", GeneratedAt: time.Now()}
		require.NoError(t, db.Create(&row).Error)
		require.NoError(t, service.UpsertPodImageScan(context.Background(), cluster, "same", "pod", "ns", "app", "image@sha256:same", row.ID))
		require.NoError(t, service.UpsertPodImageScan(context.Background(), cluster, "same", "pod", "ns", "app", "image@sha256:same", row.ID))
		for _, target := range [][3]string{{"foreign", "same", "app"}, {cluster, "foreign", "app"}, {cluster, "same", "sidecar"}, {"", "same", "app"}} {
			require.Error(t, service.UpsertPodImageScan(context.Background(), target[0], target[1], "pod", "ns", target[2], "image@sha256:same", row.ID))
		}
	}
	var count int64
	require.NoError(t, db.Model(&models.PodImageScan{}).Count(&count).Error)
	require.EqualValues(t, 2, count)
}
