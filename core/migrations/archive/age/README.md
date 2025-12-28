# Apache AGE Graph Extension Files

## Status: ARCHIVED (Not Currently Used)

These files were archived on 2025-12-27 as they are not currently integrated
into the migration system.

## Files:

- `002_install_age.sql` - Installs Apache AGE extension
- `003_age_triggers.sql` - Full trigger implementation
- `003_age_triggers_conditional.sql` - Conditional trigger implementation
- `003_age_triggers_simple.sql` - Simplified trigger implementation
- `004_age_functions_full.sql` - AGE utility functions

## Current State:

- AGE extension is NOT integrated into RunMigrations
- Graph database features are NOT currently used
- Files kept for potential future integration

## Re-integration:

If graph database features are needed in the future:

1. Determine which trigger variant to use (or create new one)
2. Delete unused variants
3. Integrate into RunMigrations as Migration 004-005
4. Test thoroughly in staging before production

## Investigation:

To check if AGE is actually used:
```sql
-- Check if extension is installed
SELECT * FROM pg_extension WHERE extname = 'age';

-- Check if triggers exist
SELECT trigger_name, event_object_table
FROM information_schema.triggers
WHERE trigger_name LIKE '%age%';
```

Last updated: 2025-12-27
