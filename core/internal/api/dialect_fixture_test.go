package api

import (
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// dialectTestDBs returns an in-memory SQLite DB and, when FORTUNA_TEST_POSTGRES_URL is set, a
// PostgreSQL DB in a throwaway schema, each migrated with the given models. Tests run the same
// assertions on each so SQL stays portable.
func dialectTestDBs(t *testing.T, migrate ...interface{}) map[string]*gorm.DB {
	t.Helper()
	out := map[string]*gorm.DB{}
	lite, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	litePool, err := lite.DB()
	require.NoError(t, err)
	litePool.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = litePool.Close() })
	require.NoError(t, lite.AutoMigrate(migrate...))
	out["sqlite"] = lite

	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		return out
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	adminSQL, err := admin.DB()
	require.NoError(t, err)
	schema := fmt.Sprintf("dialect_fixture_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	t.Cleanup(func() {
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		_ = adminSQL.Close()
	})
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	pg, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pgPool, err := pg.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgPool.Close() })
	require.NoError(t, pg.AutoMigrate(migrate...))
	out["postgres"] = pg
	return out
}
