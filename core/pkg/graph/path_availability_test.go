package graph

import (
	"context"
	"testing"

	"github.com/fortuna/core/pkg/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAttackPathBuildFailsClosedOnSnapshotError(t *testing.T) {
	t.Setenv("FORTUNA_ATTACK_PATH_BUILD_CACHE_TTL", "0s")
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(1)
	defer pool.Close()
	require.NoError(t, db.AutoMigrate(&models.Pod{}, &models.ServiceAccount{}, &models.RoleBinding{}, &models.ClusterRoleBinding{}, &models.Role{}, &models.ClusterRole{}))
	require.NoError(t, db.Create(&models.Pod{ClusterID: "availability-a", UID: "pod", Name: "pod", Namespace: "ns"}).Error)
	require.NoError(t, db.Migrator().DropTable(&models.Role{}))
	paths, err := NewRelationalPathBuilder(db).BuildAllPaths(context.Background(), "availability-a", false)
	require.Error(t, err)
	require.Nil(t, paths, "a failed snapshot must not be returned or cached as a successful clean graph")
}
