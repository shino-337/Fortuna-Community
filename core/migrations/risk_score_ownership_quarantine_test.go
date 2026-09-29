package migrations

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRiskScoreOwnershipQuarantinePostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL required")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	defer pool.Close()
	name := fmt.Sprintf("risk_quarantine_%d", time.Now().UnixNano())
	require.NoError(t, db.Exec("CREATE SCHEMA "+name).Error)
	require.NoError(t, db.Exec("SET search_path TO "+name).Error)
	defer func() { db.Exec("SET search_path TO public"); db.Exec("DROP SCHEMA " + name + " CASCADE") }()
	createClusterIdentityFoundationSchema(t, db, "")
	require.NoError(t, db.Exec(`ALTER TABLE risk_scores ADD COLUMN cluster_id text, ADD COLUMN total_score numeric, ADD COLUMN calculated_at timestamptz`).Error)
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX original_snapshot_key ON risk_scores(resource_type,resource_uid,cluster_id)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO pods(uid,cluster_id) VALUES ('occupied','a'),('two-unowned','a'),('unambiguous','a')`).Error)
	require.NoError(t, db.Exec(`INSERT INTO risk_scores(resource_type,resource_uid,cluster_id,total_score,calculated_at) VALUES ('Pod','occupied','a',90,now()),('Pod','occupied','',10,now()),('Pod','two-unowned',NULL,20,now()),('Pod','two-unowned',NULL,30,now()),('Pod','unambiguous','',40,now())`).Error)
	for i := 0; i < 2; i++ {
		require.NoError(t, EnsureClusterResourceIdentityFoundation(db))
	}
	var all, quarantined, unowned int64
	require.NoError(t, db.Table("risk_scores").Count(&all).Error)
	require.EqualValues(t, 5, all)
	require.NoError(t, db.Table("risk_score_ownership_quarantines").Count(&quarantined).Error)
	require.EqualValues(t, 3, quarantined)
	require.NoError(t, db.Table("risk_scores").Where("COALESCE(cluster_id,'') = ''").Count(&unowned).Error)
	require.EqualValues(t, 3, unowned)
	var corrupt int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM risk_score_ownership_quarantines q JOIN risk_scores r ON r.id=q.risk_score_id WHERE q.evidence <> to_jsonb(r)`).Scan(&corrupt).Error)
	require.Zero(t, corrupt, "complete original rows must be retained unchanged")
	var owner float64
	require.NoError(t, db.Table("risk_scores").Select("total_score").Where("cluster_id = 'a' AND resource_uid = 'occupied'").Scan(&owner).Error)
	require.Equal(t, float64(90), owner)
	// The snapshot upsert contract still works; removing its unique key would
	// conceal the migration collision but break every canonical scorer write.
	require.NoError(t, db.Exec(`INSERT INTO risk_scores(resource_type,resource_uid,cluster_id,total_score) VALUES ('Pod','occupied','a',95) ON CONFLICT(resource_type,resource_uid,cluster_id) DO UPDATE SET total_score=excluded.total_score`).Error)
	require.NoError(t, db.Table("risk_scores").Count(&all).Error)
	require.EqualValues(t, 5, all)
}
