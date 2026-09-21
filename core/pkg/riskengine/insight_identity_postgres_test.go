package riskengine

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPodInsightLifecyclePostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL is not configured")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	adminSQL, err := admin.DB()
	require.NoError(t, err)
	defer adminSQL.Close()
	schema := fmt.Sprintf("insight_identity_%d", time.Now().UnixNano())
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
	require.NoError(t, db.AutoMigrate(&models.Insight{}, &models.ExceptionPolicy{}))
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX idx_insight_resource_identity ON insights(cluster_id,resource_uid,cve_id,insight_type)").Error)
	assertPodInsightRestore(t, db)
	manager := NewInsightManager(db)
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cluster := "a"
			if i%2 == 1 {
				cluster = "b"
			}
			incoming := &models.Insight{ClusterID: cluster, ResourceUID: "race", ResourceType: "Pod", CVEID: "CVE-race", InsightType: "vulnerability", Severity: "high", Title: "concurrent"}
			errs <- db.Transaction(func(tx *gorm.DB) error { _, err := manager.createOrUpdatePodInsightTx(tx, incoming); return err })
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var count int64
	require.NoError(t, db.Model(&models.Insight{}).Where("resource_uid = ?", "race").Count(&count).Error)
	require.EqualValues(t, 2, count)
}
