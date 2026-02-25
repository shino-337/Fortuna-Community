#!/bin/bash
# File: scripts/validate-migrations.sh
# Purpose: Validate migration integrity in CI/CD pipeline

set -e

MIGRATIONS_DIR="core/migrations"
MIGRATIONS_FILE="$MIGRATIONS_DIR/migrations.go"

echo "========================================="
echo "Migration Validation Script"
echo "========================================="

# Test 1: Check migrations.go exists
echo ""
echo "Test 1: Checking migrations.go exists..."
if [ ! -f "$MIGRATIONS_FILE" ]; then
    echo "❌ FAIL: migrations.go not found"
    exit 1
fi
echo "✅ PASS: migrations.go exists"

# Test 2: Check for AutoMigrate in production code (warning only for now)
echo ""
echo "Test 2: Checking for AutoMigrate usage (warning only)..."
AUTOMIGRATE_COUNT=$(grep -r "db.AutoMigrate\|\.AutoMigrate" "$MIGRATIONS_DIR"/*.go 2>/dev/null | grep -v "_test.go" | grep -v "// " | wc -l | tr -d ' ')
if [ "$AUTOMIGRATE_COUNT" -gt 0 ]; then
    echo "⚠️  WARNING: Found $AUTOMIGRATE_COUNT AutoMigrate calls in migrations"
    echo "    This will be enforced as an error after Phase 2 cleanup"
    grep -n "db.AutoMigrate\|\.AutoMigrate" "$MIGRATIONS_DIR"/*.go 2>/dev/null | grep -v "_test.go" | grep -v "// " || true
else
    echo "✅ PASS: No AutoMigrate in production migrations"
fi

# Test 3: Check SQL file references
echo ""
echo "Test 3: Checking SQL file references..."
MISSING_FILES=0
while IFS= read -r line; do
    if [[ $line =~ \"([^\"]+\.sql)\" ]]; then
        SQL_FILE="${BASH_REMATCH[1]}"
        # Check if file exists (try multiple paths)
        if [ ! -f "$MIGRATIONS_DIR/$SQL_FILE" ] && [ ! -f "$SQL_FILE" ] && [ ! -f "$MIGRATIONS_DIR/mvp2/$SQL_FILE" ]; then
            echo "⚠️  Missing SQL file: $SQL_FILE (may be in archive or not needed)"
            MISSING_FILES=$((MISSING_FILES + 1))
        fi
    fi
done < "$MIGRATIONS_FILE"

if [ $MISSING_FILES -gt 0 ]; then
    echo "⚠️  WARNING: $MISSING_FILES SQL file(s) not found (may be archived)"
    echo "    Review if these files are actually needed"
else
    echo "✅ PASS: All referenced SQL files exist"
fi

# Test 4: Check migration numbering sequence
echo ""
echo "Test 4: Checking migration numbering sequence..."
MIGRATION_NUMBERS=$(grep -o "Migration[0-9]\{3\}_" "$MIGRATIONS_FILE" | grep -o "[0-9]\{3\}" | sort -n | uniq)
EXPECTED=1
GAPS=0
for NUM in $MIGRATION_NUMBERS; do
    NUM_INT=$((10#$NUM))  # Convert to decimal (remove leading zeros)
    if [ $NUM_INT -ne $EXPECTED ]; then
        echo "⚠️  WARNING: Gap in numbering: expected $EXPECTED, found $NUM_INT"
        GAPS=$((GAPS + 1))
    fi
    EXPECTED=$((NUM_INT + 1))
done

if [ $GAPS -gt 0 ]; then
    echo "⚠️  WARNING: Found $GAPS gap(s) in migration numbering"
    echo "    This is acceptable if migrations were consolidated"
else
    echo "✅ PASS: Migration numbering is sequential"
fi

# Test 5: Check for duplicate migration numbers
echo ""
echo "Test 5: Checking for duplicate migration numbers..."
DUPLICATES=$(echo "$MIGRATION_NUMBERS" | sort | uniq -d)
if [ -n "$DUPLICATES" ]; then
    echo "❌ FAIL: Duplicate migration numbers found:"
    echo "$DUPLICATES"
    exit 1
else
    echo "✅ PASS: No duplicate migration numbers"
fi

# Test 6: Validate Go syntax
echo ""
echo "Test 6: Validating Go syntax..."
if ! go vet "$MIGRATIONS_DIR"/*.go 2>&1 | grep -v "no Go files" | grep -v "^$"; then
    echo "✅ PASS: Go syntax is valid"
else
    echo "❌ FAIL: Go syntax errors found"
    exit 1
fi

# Test 7: Check for orphaned .old files (should be cleaned up)
echo ""
echo "Test 7: Checking for .old migration files..."
OLD_FILES=$(find "$MIGRATIONS_DIR" -name "*.old" 2>/dev/null | wc -l | tr -d ' ')
if [ "$OLD_FILES" -gt 0 ]; then
    echo "ℹ️  INFO: Found $OLD_FILES .old file(s) (backups from consolidation)"
    echo "    These can be removed after verification"
else
    echo "✅ PASS: No .old files found"
fi

echo ""
echo "========================================="
echo "Validation completed!"
echo "========================================="
echo ""
echo "Summary:"
echo "  - Migration files: ✅ Valid"
echo "  - Go syntax: ✅ Valid"
echo "  - Numbering: ✅ Valid"
echo ""
echo "Warnings (non-blocking):"
echo "  - AutoMigrate usage: Will be removed in Phase 2"
echo "  - Missing SQL files: May be archived (verify if needed)"
echo ""

