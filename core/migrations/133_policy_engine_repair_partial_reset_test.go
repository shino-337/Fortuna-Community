package migrations

import (
	"testing"

	"github.com/fortuna/core/pkg/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestMigration133_RepairPolicyEngineAfterPartialReset(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}))

	require.NoError(t, Migration133_RepairPolicyEngineAfterPartialReset(db))

	require.True(t, db.Migrator().HasTable("policy_templates"))
	require.True(t, db.Migrator().HasTable("policy_instances"))

	var tplCount, instCount int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM policy_templates").Scan(&tplCount).Error)
	require.NoError(t, db.Model(&models.PolicyInstance{}).Count(&instCount).Error)
	require.GreaterOrEqual(t, tplCount, int64(1))
	require.GreaterOrEqual(t, instCount, int64(1))

	// Idempotent second run
	require.NoError(t, Migration133_RepairPolicyEngineAfterPartialReset(db))
}
