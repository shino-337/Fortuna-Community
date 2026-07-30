package migrations

import (
	"testing"

	"github.com/fortuna/core/internal/auth"
	"github.com/fortuna/core/pkg/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateDefaultAdminBootstrapDoesNotOverwriteChangedPassword(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}))
	require.NoError(t, Migration147_UserPasswordBootstrapState(db))

	require.NoError(t, CreateDefaultAdmin(db, "admin", DefaultBootstrapAdminPassword, "admin@fortuna.local", true, true))

	var user models.User
	require.NoError(t, db.Where("username = ?", "admin").First(&user).Error)
	require.True(t, user.MustChangePassword)
	require.True(t, user.BootstrapCredential)
	require.True(t, auth.CheckPasswordHash(DefaultBootstrapAdminPassword, user.Password))

	changedHash, err := auth.HashPassword("Fortuna_User_Changed_123!")
	require.NoError(t, err)
	require.NoError(t, db.Model(&user).Updates(map[string]interface{}{
		"password":             changedHash,
		"must_change_password": false,
		"bootstrap_credential": false,
	}).Error)

	require.NoError(t, CreateDefaultAdmin(db, "admin", DefaultBootstrapAdminPassword, "admin@fortuna.local", true, true))

	require.NoError(t, db.Where("username = ?", "admin").First(&user).Error)
	require.False(t, auth.CheckPasswordHash(DefaultBootstrapAdminPassword, user.Password))
	require.True(t, auth.CheckPasswordHash("Fortuna_User_Changed_123!", user.Password))
	require.False(t, user.MustChangePassword)
	require.False(t, user.BootstrapCredential)
}
