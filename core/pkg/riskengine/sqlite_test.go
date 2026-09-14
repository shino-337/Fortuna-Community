package riskengine

import (
	"testing"

	"gorm.io/gorm"
)

// SQLite :memory: databases belong to a connection, not to the pool.
// A background scorer otherwise opens another connection with an empty schema.
func configureRiskEngineTestDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
}
