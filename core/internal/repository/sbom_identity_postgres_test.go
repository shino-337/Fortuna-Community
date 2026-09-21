package repository

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fortuna/core/internal/contextkeys"
	"github.com/fortuna/core/migrations"
	"github.com/fortuna/core/pkg/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSBOMConcurrentOwnershipPostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL is not configured")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	adminSQL, err := admin.DB()
	require.NoError(t, err)
	defer adminSQL.Close()
	schema := fmt.Sprintf("sbom_identity_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	defer admin.Exec("DROP SCHEMA " + schema + " CASCADE")
	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		require.NoError(t, err)
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		dsn = u.String()
	} else {
		dsn += " search_path=" + schema
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(8)
	require.NoError(t, db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.SBOMMatchRun{}, &models.PodImageScan{}, &models.MalwareMatch{}, &models.CVEMatch{}))
	// Reproduce the schema before content separation, not only fresh AutoMigrate.
	require.NoError(t, db.Exec("ALTER TABLE sboms DROP COLUMN content_id CASCADE").Error)
	require.NoError(t, db.Exec("DROP TABLE sbom_image_contents").Error)
	// Populated legacy data: two workloads share bytes, unresolved ownership stays quarantined.
	for _, cluster := range []string{"legacy-a", "legacy-b", ""} {
		row := models.SBOM{ClusterID: cluster, PodUID: "legacy", ContainerName: "app", ImageDigest: "sha256:legacy", SBOMContent: "{}", Status: "pending", GeneratedAt: time.Now()}
		require.NoError(t, db.Omit("ContentID", "ContentRef").Create(&row).Error)
	}
	require.NoError(t, migrations.EnsureSBOMContentIdentity(db))
	require.NoError(t, migrations.EnsureSBOMContentIdentity(db))
	var legacy []models.SBOM
	require.NoError(t, db.Where("pod_uid = ?", "legacy").Order("id").Find(&legacy).Error)
	require.Len(t, legacy, 3)
	require.NotNil(t, legacy[0].ContentID)
	require.Equal(t, legacy[0].ContentID, legacy[1].ContentID)
	require.Nil(t, legacy[2].ContentID)
	require.Error(t, db.Exec("UPDATE sbom_image_contents SET payload='{}' WHERE id=?", *legacy[0].ContentID).Error)
	require.Error(t, db.Exec("UPDATE sboms SET cluster_id='foreign' WHERE id=?", legacy[0].ID).Error)
	require.NoError(t, db.Create(&models.MalwareMatch{ClusterID: "legacy-a", PodUID: "legacy", SBOMID: legacy[0].ID, PackageName: "evil", PackageVersion: "1", Reason: "MALWARE"}).Error)
	require.ErrorContains(t, db.Create(&models.MalwareMatch{ClusterID: "legacy-b", PodUID: "legacy", SBOMID: legacy[0].ID, PackageName: "evil", PackageVersion: "1", Reason: "MALWARE"}).Error, "SBOM association ownership mismatch")
	require.ErrorContains(t, db.Create(&models.PodImageScan{ClusterID: "legacy-b", PodUID: "legacy", ContainerName: "app", SBOMID: &legacy[0].ID}).Error, "SBOM association ownership mismatch")
	require.ErrorContains(t, db.Create(&models.CVEMatch{ClusterID: "legacy-b", PodUID: "legacy", ContainerName: "app", SBOMID: legacy[0].ID, PackageName: "lib", PackageVersion: "1", CVEID: "CVE-test", Severity: "high"}).Error, "SBOM association ownership mismatch")
	repo := NewSBOMRepository(db)
	ctx := contextkeys.WithSBOMMutationAllowed(context.Background())
	var wg sync.WaitGroup
	errors := make(chan error, 24)
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			row := &models.SBOM{ClusterID: fmt.Sprintf("cluster-%d", i%2), PodUID: "same", ContainerName: fmt.Sprintf("container-%d", (i/2)%2), ImageDigest: "sha256:same", Status: "pending", GeneratedAt: time.Now()}
			_, _, err := repo.UpsertSBOMWithComponents(ctx, row, nil)
			errors <- err
		}(i)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	var count int64
	require.NoError(t, db.Model(&models.SBOM{}).Where("pod_uid = ?", "same").Count(&count).Error)
	require.EqualValues(t, 4, count, "concurrent first inserts must not duplicate or collapse owners")
	var contents int64
	require.NoError(t, db.Model(&models.SBOMImageContent{}).Count(&contents).Error)
	require.EqualValues(t, 2, contents, "legacy and current digest content, shared across all owners")
	var current models.SBOM
	require.NoError(t, db.Where("pod_uid = ?", "same").First(&current).Error)
	require.NotNil(t, current.ContentID)
	require.ErrorContains(t, db.Exec("UPDATE sboms SET content_id=? WHERE id=?", *current.ContentID, legacy[0].ID).Error, "SBOM content digest mismatch")

	duplicate := models.SBOM{ClusterID: "cluster-0", PodUID: "same", ContainerName: "container-0", ImageDigest: "sha256:same", SBOMContent: "{}", GeneratedAt: time.Now()}
	require.Error(t, db.Create(&duplicate).Error, "direct writers must obey the unique ownership key")
	require.NoError(t, db.Exec("DROP INDEX idx_sbom_active_workload_identity").Error)
	duplicate.ID = 0
	require.NoError(t, db.Create(&duplicate).Error)
	require.ErrorContains(t, migrations.EnsureSBOMContentIdentity(db), "duplicate active SBOM ownership")
	require.NoError(t, db.Model(&models.SBOM{}).Where("pod_uid = ?", "same").Count(&count).Error)
	require.EqualValues(t, 5, count, "migration must preserve conflicting evidence for explicit repair")

}
