#!/bin/bash
# File: scripts/clean/cleanup-orphaned-migrations.sh
# Purpose: Archive orphaned migration files identified in audit report

set -e

MIGRATIONS_DIR="core/migrations"
MVP2_DIR="$MIGRATIONS_DIR/mvp2"
ARCHIVE_DIR="$MIGRATIONS_DIR/archive"

echo "========================================="
echo "Orphaned Migration Files Cleanup Script"
echo "========================================="
echo ""

# Create archive directories
mkdir -p "$ARCHIVE_DIR/mvp2"
mkdir -p "$ARCHIVE_DIR/mvp3"
mkdir -p "$ARCHIVE_DIR/age"

# Function to archive file
archive_file() {
    local file=$1
    local dest=$2
    local reason=$3
    if [ -f "$file" ]; then
        echo "Archiving: $file -> $dest"
        echo "  Reason: $reason"
        mv "$file" "$dest"
    else
        echo "File not found (already cleaned?): $file"
    fi
}

# Archive MVP2 orphaned files
echo "========================================="
echo "Archiving MVP2 orphaned SQL files..."
echo "========================================="
archive_file "$MVP2_DIR/004_policy_instances.sql" "$ARCHIVE_DIR/mvp2/" "Migration 015 uses AutoMigrate instead"
archive_file "$MVP2_DIR/005_policy_violations.sql" "$ARCHIVE_DIR/mvp2/" "Migration 016 uses AutoMigrate instead"
archive_file "$MVP2_DIR/005_add_cve_tables.sql" "$ARCHIVE_DIR/mvp2/" "Migration 019 uses inline SQL instead"

# Archive MVP3 schema file
echo ""
echo "========================================="
echo "Archiving MVP3 schema file..."
echo "========================================="
archive_file "$MIGRATIONS_DIR/mvp3_agent_based_schema.sql" "$ARCHIVE_DIR/mvp3/" "Not yet integrated into RunMigrations"

# Archive AGE files (if not used)
echo ""
echo "========================================="
echo "Archiving AGE extension files..."
echo "========================================="
echo "Note: AGE files are archived as they are not integrated into RunMigrations"
echo "If graph database features are needed, these can be re-integrated"
archive_file "$MIGRATIONS_DIR/002_install_age.sql" "$ARCHIVE_DIR/age/" "Not integrated into RunMigrations"
archive_file "$MIGRATIONS_DIR/003_age_triggers.sql" "$ARCHIVE_DIR/age/" "Not integrated, duplicate variants exist"
archive_file "$MIGRATIONS_DIR/003_age_triggers_conditional.sql" "$ARCHIVE_DIR/age/" "Not integrated, duplicate variant"
archive_file "$MIGRATIONS_DIR/003_age_triggers_simple.sql" "$ARCHIVE_DIR/age/" "Not integrated, duplicate variant"
archive_file "$MIGRATIONS_DIR/004_age_functions_full.sql" "$ARCHIVE_DIR/age/" "Not integrated into RunMigrations"

# Create archive README
cat > "$ARCHIVE_DIR/README.md" << 'EOF'
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
EOF

# Create AGE-specific README
cat > "$ARCHIVE_DIR/age/README.md" << 'EOF'
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
EOF

echo ""
echo "========================================="
echo "Cleanup completed successfully!"
echo "========================================="
echo ""
echo "Summary:"
echo "  - Archived files are in: $ARCHIVE_DIR"
echo "  - Active migrations remain in: $MIGRATIONS_DIR"
echo ""
echo "Next steps:"
echo "  1. Review archived files in: $ARCHIVE_DIR"
echo "  2. Commit changes: git add core/migrations"
echo "  3. Commit changes: git commit -m 'chore: archive orphaned migration files per audit report'"
echo ""

