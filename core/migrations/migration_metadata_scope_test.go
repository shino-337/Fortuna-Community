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

func TestMigrationMetadataUsesCurrentSchemaPostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL required")
	}
	t.Chdir("..")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	defer pool.Close()
	own := fmt.Sprintf("migration_owner_%d", time.Now().UnixNano())
	foreign := own + "_foreign"
	for _, name := range []string{own, foreign} {
		require.NoError(t, db.Exec("CREATE SCHEMA "+name).Error)
	}
	defer func() {
		db.Exec("SET search_path TO public")
		db.Exec("DROP SCHEMA " + own + " CASCADE")
		db.Exec("DROP SCHEMA " + foreign + " CASCADE")
	}()
	require.NoError(t, db.Exec("CREATE TABLE "+foreign+".risk_scores (id integer)").Error)
	require.NoError(t, db.Exec("SET search_path TO "+own).Error)
	require.NoError(t, Migration012_AddRiskScores(db))
	require.True(t, db.Migrator().HasTable("risk_scores"))
	require.True(t, db.Migrator().HasColumn("risk_scores", "cluster_id"))
	var foreignColumns int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = ? AND table_name = 'risk_scores'", foreign).Scan(&foreignColumns).Error)
	require.EqualValues(t, 1, foreignColumns, "migration must not adopt a foreign schema's table")
}
