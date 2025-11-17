package migrations

import "gorm.io/gorm"

// Migration004_UpdateIndexes updates unique indexes to scope by cluster and ignore soft-deleted rows.
func Migration004_UpdateIndexes(db *gorm.DB) error {
	statements := []string{
		// ServiceAccounts
		"ALTER TABLE service_accounts DROP CONSTRAINT IF EXISTS idx_service_accounts_uid;",
		"DROP INDEX IF EXISTS idx_sa_uid;",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_sa_cluster_uid ON service_accounts (cluster_id, uid) WHERE deleted_at IS NULL;",

		// RoleBindings
		"ALTER TABLE role_bindings DROP CONSTRAINT IF EXISTS idx_role_bindings_uid;",
		"DROP INDEX IF EXISTS idx_rb_uid;",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_rb_cluster_uid ON role_bindings (cluster_id, uid) WHERE deleted_at IS NULL;",

		// ClusterRoleBindings
		"ALTER TABLE cluster_role_bindings DROP CONSTRAINT IF EXISTS idx_cluster_role_bindings_uid;",
		"DROP INDEX IF EXISTS idx_crb_uid;",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_crb_cluster_uid ON cluster_role_bindings (cluster_id, uid) WHERE deleted_at IS NULL;",

		// Roles
		"ALTER TABLE roles DROP CONSTRAINT IF EXISTS idx_roles_uid;",
		"DROP INDEX IF EXISTS idx_role_uid;",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_role_cluster_uid ON roles (cluster_id, uid) WHERE deleted_at IS NULL;",

		// ClusterRoles
		"ALTER TABLE cluster_roles DROP CONSTRAINT IF EXISTS idx_cluster_roles_uid;",
		"DROP INDEX IF EXISTS idx_cr_uid;",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_cr_cluster_uid ON cluster_roles (cluster_id, uid) WHERE deleted_at IS NULL;",

		// Pods
		"ALTER TABLE pods DROP CONSTRAINT IF EXISTS idx_pods_uid;",
		"DROP INDEX IF EXISTS idx_pod_uid;",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_pod_cluster_uid ON pods (cluster_id, uid) WHERE deleted_at IS NULL;",
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}

	return nil
}
