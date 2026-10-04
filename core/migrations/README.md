# Database migrations

Core runs every migration in this directory at startup (`storage.Migrate` in `core/internal/storage`). There is no separate migration command.

## How versions are recorded

`RunMigrations` in `migrations.go` holds one ordered slice of migration functions. Each entry's **version is its position in that slice** (`i + 1`), not the number in its file or function name, and it is recorded in the `schema_migrations` table once it succeeds. Function numbers have gaps (004–007, 017, 085) and some entries are listed out of numeric order (082 before 080 and 081, 150 before 149), so do not infer the recorded version from a file name.

Because of this, the slice is **append-only** ([security invariant 7](../../docs/06-reference/SECURITY_INVARIANTS.md#invariant-7--migration-history-is-append-only)):

- Add new migrations only at the end of the slice.
- Never reorder, remove or insert entries before already-shipped ones. That would shift every later version and make Core skip or re-run migrations on existing databases.

A migration that returns an error is logged and not recorded, and the loop continues. It runs again on the next start. Check Core startup logs for `Migration N failed` after an upgrade.

## Guards that run after the slice

After `RunMigrations`, `storage.Migrate` runs schema guards that must hold for security-critical ownership (for example `EnsureClusterResourceIdentityFoundation`, `EnsureClusterQualifiedPodUniqueness`, `EnsureSBOMContentIdentity`, `EnsureAgentCompositeIdentity`). These fail startup instead of continuing. They live in the unnumbered files in this directory.

## Post-migrations and the bootstrap admin

`RunPostMigrations` creates or updates the admin account:

| Variable | Default | Effect |
|---|---|---|
| `FORTUNA_ADMIN_USERNAME` | `admin` | Admin username |
| `FORTUNA_ADMIN_PASSWORD` | unset | When set, the admin is created with it, and an existing admin is re-synced to it whenever it no longer matches. Rotate it through the Secret, not only in the UI |
| *(unset password)* | `Fortuna_ChangeMe_123!` | Bootstrap credential: the admin must change it at first login, and it never overwrites an existing account |
| `FORTUNA_ADMIN_EMAIL` | `<username>@fortuna.local` | Admin email |
| `FORTUNA_ALLOW_WEAK_BOOTSTRAP_PASSWORD` | `false` | Allows a password below the policy (12+ chars, upper, lower, digit, special). Non-production only |

Seed migrations (050, 051, 061) insert reference data only when `FORTUNA_ENABLE_SEED_DATA=true`.

## Adding a migration

1. Copy `MIGRATION_TEMPLATE.go` to `NNN_short_name.go`, using the next free number.
2. Make it safe to re-run against a partially migrated database: check for tables, columns and indexes before creating them. Return errors instead of logging and continuing.
3. Append the function to the end of the slice in `RunMigrations`.
4. Add a test. Changes to ownership or identity schema also need a PostgreSQL test in the populated-migration CI job.

Back up the database before upgrading. An image rollback does not reverse migrations.
