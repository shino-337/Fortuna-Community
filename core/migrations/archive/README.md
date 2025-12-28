# Archived Migration Files

This directory contains migration files that were removed from the active
migration system but are kept for historical reference.

## Directory Structure:

- `mvp2/` - Orphaned MVP2 SQL files that were replaced by Go migrations
- `mvp3/` - MVP3 schema files not yet integrated
- `age/` - Apache AGE graph extension files (not currently used)

## Why These Files Were Archived:

### MVP2 Files:
- `004_policy_instances.sql` - Migration 015 uses AutoMigrate instead
- `005_policy_violations.sql` - Migration 016 uses AutoMigrate instead
- `005_add_cve_tables.sql` - Migration 019 uses inline SQL instead

### MVP3 Files:
- `mvp3_agent_based_schema.sql` - Not yet integrated into RunMigrations

### AGE Files:
- `002_install_age.sql` - Apache AGE extension installation (not integrated)
- `003_age_triggers*.sql` - Multiple trigger variants (not integrated)
- `004_age_functions_full.sql` - AGE utility functions (not integrated)

**Note:** AGE (Apache Graph Extension) files are archived as graph database
features are not currently used. If graph database features are needed in the
future, these files can be re-integrated.

## Restoration:

If these files are needed in the future, they can be restored from this archive.

Last updated: 2025-12-27
