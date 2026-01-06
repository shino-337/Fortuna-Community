# Database Migrations

This directory contains database migration files for Fortuna Core.

## Migration System

The migration system uses GORM's AutoMigrate feature to automatically create and update database tables.

## Migration Files

### Migration 001: Initial Schema
Creates the initial database schema:
- `clusters` - Kubernetes cluster information
- `service_accounts` - ServiceAccount data
- `role_bindings` - RoleBinding data
- `cluster_role_bindings` - ClusterRoleBinding data
- `roles` - Role data
- `cluster_roles` - ClusterRole data
- `pods` - Pod data
- `audit_logs` - Audit log entries

### Migration 002: Add Users
Creates the `users` table for authentication:
- `id` - Primary key
- `username` - Unique username
- `email` - Unique email
- `password` - Hashed password
- `role` - User role (admin, user, viewer)
- `active` - Active status
- `last_login` - Last login timestamp
- `created_at`, `updated_at`, `deleted_at` - Timestamps

### Migration 003: Add User to Audit Logs
Adds `user_id` column to `audit_logs` table and creates foreign key constraint.

## Running Migrations

Migrations are automatically run when the application starts. They are executed in order:

1. `Migration001_InitialSchema`
2. `Migration002_AddUsers`
3. `Migration003_AddUserToAuditLogs`

## Post-Migrations

After schema migrations, post-migrations are run:
- `CreateDefaultAdmin` - Creates a default admin user if environment variables are set

### Environment Variables for Default Admin

- `FORTUNA_ADMIN_USERNAME` - Admin username (required)
- `FORTUNA_ADMIN_PASSWORD` - Admin password (required)
- `FORTUNA_ADMIN_EMAIL` - Admin email (optional, defaults to username@fortuna.local)

Example:
```bash
export FORTUNA_ADMIN_USERNAME=admin
export FORTUNA_ADMIN_PASSWORD=changeme
export FORTUNA_ADMIN_EMAIL=admin@example.com
```

## Manual Migration

To run migrations manually:

```go
import (
    "github.com/fortuna/core/migrations"
    "gorm.io/gorm"
)

// Run migrations
if err := migrations.RunMigrations(db); err != nil {
    log.Fatal(err)
}

// Run post-migrations
if err := migrations.RunPostMigrations(db); err != nil {
    log.Fatal(err)
}
```

## Adding New Migrations

To add a new migration:

1. Create a new migration function in `migrations.go`:
```go
func Migration004_YourMigration(db *gorm.DB) error {
    log.Println("Running migration 004: Your migration")
    // Your migration logic here
    return nil
}
```

2. Add it to the `RunMigrations` function:
```go
migrations := []func(*gorm.DB) error{
    Migration001_InitialSchema,
    Migration002_AddUsers,
    Migration003_AddUserToAuditLogs,
    Migration004_YourMigration, // Add here
}
```

## Notes

- Migrations are idempotent - they can be run multiple times safely
- Use `db.Migrator().HasColumn()` to check if a column exists before adding it
- Always test migrations on a development database first
- Back up your database before running migrations in production

