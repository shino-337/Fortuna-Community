package migrations

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func createPopulatedPodKeySchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	createClusterIdentityFoundationSchema(t, db, "")
	require.NoError(t, db.Exec("INSERT INTO pods VALUES ('same','a')").Error)
	extras := map[string]string{"pod_capabilities": "capability_id", "attack_paths": "path_id", "pod_image_scans": "container_name", "pod_attack_steps": "step_id"}
	for table, col := range extras {
		require.NoError(t, db.Exec("ALTER TABLE "+table+" ADD COLUMN "+col+" TEXT NOT NULL DEFAULT 'key'").Error)
	}
	require.NoError(t, db.Exec("ALTER TABLE insights ADD COLUMN cve_id TEXT").Error)
	require.NoError(t, db.Exec("ALTER TABLE insights ADD COLUMN insight_type TEXT NOT NULL DEFAULT 'vulnerability'").Error)
	for _, target := range clusterQualifiedPodUniqueIndexes {
		uid := "pod_uid"
		if target.table == "insights" {
			uid = "resource_uid"
		}
		columns, values := uid, "'same'"
		if target.table == "insights" {
			columns += ",resource_type"
			values += ",'Pod'"
		}
		require.NoError(t, db.Exec("INSERT INTO "+target.table+" ("+columns+") VALUES ("+values+")").Error)
	}
	for _, target := range clusterQualifiedPodPrimaryKeys {
		require.NoError(t, db.Exec("INSERT INTO "+target.table+" (pod_uid) VALUES ('same')").Error)
	}
	require.NoError(t, EnsureClusterResourceIdentityFoundation(db))
}

func TestClusterQualifiedPodUniquenessRejectsUnowned(t *testing.T) {
	db := openClusterIdentityTestDB(t)
	createPopulatedPodKeySchema(t, db)
	require.NoError(t, db.Exec("UPDATE pod_capabilities SET cluster_id = ''").Error)
	require.ErrorContains(t, EnsureClusterQualifiedPodUniqueness(db), "without resolvable cluster ownership")
}

func TestClusterQualifiedPodUniquenessPostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL is not configured")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	adminSQL, err := admin.DB()
	require.NoError(t, err)
	defer adminSQL.Close()
	schema := fmt.Sprintf("pod_keys_%d", time.Now().UnixNano())
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
	createPopulatedPodKeySchema(t, db)
	// Install the old UID-only constraints before exercising the cutover.
	for _, target := range clusterQualifiedPodUniqueIndexes {
		legacyCols := strings.TrimPrefix(target.columns, "cluster_id, ")
		for _, name := range target.legacyIndexes {
			require.NoError(t, db.Exec("CREATE UNIQUE INDEX "+name+" ON "+target.table+" ("+legacyCols+")").Error)
		}
		for _, name := range target.legacyConstraints {
			require.NoError(t, db.Exec("ALTER TABLE "+target.table+" ADD CONSTRAINT "+name+" UNIQUE ("+legacyCols+")").Error)
		}
	}
	for _, target := range clusterQualifiedPodPrimaryKeys {
		require.NoError(t, db.Exec("ALTER TABLE "+target.table+" ADD CONSTRAINT "+target.constraint+" PRIMARY KEY ("+strings.TrimPrefix(target.columns, "cluster_id, ")+")").Error)
	}
	for i := 0; i < 2; i++ {
		require.NoError(t, EnsureClusterQualifiedPodUniqueness(db))
	}
	require.NoError(t, db.Exec("INSERT INTO pods VALUES ('same','b')").Error)
	for _, target := range clusterQualifiedPodUniqueIndexes {
		cols := strings.Split(target.columns, ", ")
		values := make([]string, len(cols))
		for i, col := range cols {
			switch col {
			case "cluster_id":
				values[i] = "'b'"
			case "pod_uid", "resource_uid":
				values[i] = "'same'"
			case "cve_id":
				values[i] = "''"
			case "insight_type":
				values[i] = "'vulnerability'"
			default:
				values[i] = "'key'"
			}
		}
		columns := target.columns
		if target.table == "insights" {
			columns += ",resource_type"
			values = append(values, "'Pod'")
		}
		insert := "INSERT INTO " + target.table + " (" + columns + ") VALUES (" + strings.Join(values, ",") + ")"
		require.NoError(t, db.Exec(insert).Error, "foreign cluster must not collide")
		require.Error(t, db.Exec(insert).Error, "same cluster must remain unique")
	}
	for _, target := range clusterQualifiedPodPrimaryKeys {
		require.NoError(t, db.Exec("INSERT INTO "+target.table+" (cluster_id,pod_uid) VALUES ('b','same')").Error)
	}
	// A partial index is not a valid replacement for a full ON CONFLICT target.
	require.NoError(t, db.Exec("DROP INDEX idx_insight_resource_identity").Error)
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX idx_insight_resource_identity ON insights(cluster_id,resource_uid,cve_id,insight_type) WHERE cluster_id = 'a'").Error)
	require.NoError(t, EnsureClusterQualifiedPodUniqueness(db))
	// Concurrent scoped writers must have an inferable conflict target and one row per identity.
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
			errs <- db.Exec("INSERT INTO insights(cluster_id,resource_uid,resource_type,cve_id,insight_type) VALUES (?, 'concurrent','Pod','CVE-race','vulnerability') ON CONFLICT (cluster_id,resource_uid,cve_id,insight_type) DO NOTHING", cluster).Error
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var count int64
	require.NoError(t, db.Table("insights").Where("resource_uid = ?", "concurrent").Count(&count).Error)
	require.EqualValues(t, 2, count)
	require.NoError(t, EnsureClusterQualifiedPodUniqueness(db))
}
