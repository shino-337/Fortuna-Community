package service

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fortuna/core/pkg/authorization"
	"github.com/fortuna/core/pkg/models"
)

// Agent-sync audit rows must name a dedicated account, never a human admin, and
// that account must not be able to sign in or do anything.
func TestSystemUserIsNotAnAdmin(t *testing.T) {
	db := openTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.User{}))
	admin := models.User{Username: "admin", Email: "admin@test", Password: "x", Role: models.RoleAdmin, Active: true}
	require.NoError(t, db.Create(&admin).Error)

	id := NewAgentService(db).getSystemUserID()
	require.NotZero(t, id)
	require.NotEqual(t, admin.ID, id, "audit rows must not be attributed to a human admin")

	var system models.User
	require.NoError(t, db.First(&system, id).Error)
	require.Equal(t, "system", system.Username)
	require.Equal(t, models.RoleSystem, system.Role)
	require.False(t, system.Active)
	require.Empty(t, authorization.PermissionsForUser(system.Role))
}
