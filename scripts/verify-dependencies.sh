#!/bin/bash
# verify-dependencies.sh
# Verify database dependency versions and compatibility

set -e

echo "=========================================="
echo "Database Dependency Verification"
echo "=========================================="
echo ""

cd KSAM/core

echo "Step 1: Checking go.mod for database dependencies..."
echo ""

# Check GORM version
GORM_VERSION=$(grep -E "gorm.io/gorm" go.mod | head -1 | awk '{print $2}' || echo "NOT FOUND")
echo "GORM Version: $GORM_VERSION"

# Check PostgreSQL driver version
PG_DRIVER_VERSION=$(grep -E "gorm.io/driver/postgres" go.mod | head -1 | awk '{print $2}' || echo "NOT FOUND")
echo "PostgreSQL Driver Version: $PG_DRIVER_VERSION"

# Check lib/pq version
PQ_VERSION=$(grep -E "github.com/lib/pq" go.mod | head -1 | awk '{print $2}' || echo "NOT FOUND")
if [ "$PQ_VERSION" != "NOT FOUND" ]; then
    echo "lib/pq Version: $PQ_VERSION"
fi

# Check pgx version
PGX_VERSION=$(grep -E "github.com/jackc/pgx/v5" go.mod | head -1 | awk '{print $2}' || echo "NOT FOUND")
if [ "$PGX_VERSION" != "NOT FOUND" ]; then
    echo "pgx/v5 Version: $PGX_VERSION"
fi

echo ""
echo "Step 2: Checking for version pinning..."
echo ""

# Check if versions are pinned (no ^ or ~)
if echo "$GORM_VERSION" | grep -qE '\^|~'; then
    echo "⚠️  WARNING: GORM version is not pinned: $GORM_VERSION"
    echo "   Recommendation: Pin to exact version (remove ^ or ~)"
else
    echo "✅ GORM version is pinned: $GORM_VERSION"
fi

if echo "$PG_DRIVER_VERSION" | grep -qE '\^|~'; then
    echo "⚠️  WARNING: PostgreSQL driver version is not pinned: $PG_DRIVER_VERSION"
    echo "   Recommendation: Pin to exact version (remove ^ or ~)"
else
    echo "✅ PostgreSQL driver version is pinned: $PG_DRIVER_VERSION"
fi

echo ""
echo "Step 3: Checking for compatibility issues..."
echo ""

# Known compatible versions
KNOWN_GORM_VERSIONS=("v1.25.0" "v1.25.1" "v1.25.2" "v1.25.3" "v1.25.4" "v1.25.5" "v1.30.0")
KNOWN_PG_VERSIONS=("v1.5.0" "v1.5.1" "v1.5.2" "v1.5.3" "v1.5.4")

GORM_OK=false
for v in "${KNOWN_GORM_VERSIONS[@]}"; do
    if [[ "$GORM_VERSION" == "$v" ]] || [[ "$GORM_VERSION" == "$v"* ]]; then
        GORM_OK=true
        break
    fi
done

if [ "$GORM_OK" = true ]; then
    echo "✅ GORM version is in known compatible range"
else
    echo "⚠️  WARNING: GORM version may not be tested: $GORM_VERSION"
    echo "   Known compatible versions: ${KNOWN_GORM_VERSIONS[*]}"
fi

PG_OK=false
for v in "${KNOWN_PG_VERSIONS[@]}"; do
    if [[ "$PG_DRIVER_VERSION" == "$v" ]] || [[ "$PG_DRIVER_VERSION" == "$v"* ]]; then
        PG_OK=true
        break
    fi
done

if [ "$PG_OK" = true ]; then
    echo "✅ PostgreSQL driver version is in known compatible range"
else
    echo "⚠️  WARNING: PostgreSQL driver version may not be tested: $PG_DRIVER_VERSION"
    echo "   Known compatible versions: ${KNOWN_PG_VERSIONS[*]}"
fi

echo ""
echo "Step 4: Checking migration code for best practices..."
echo ""

# Check if migrations check table existence
if grep -q "SELECT EXISTS.*information_schema.tables" migrations/migrations.go; then
    echo "✅ Migrations check table existence"
else
    echo "⚠️  WARNING: Migrations may not check table existence"
fi

# Check if migrations handle errors
if grep -q "insufficient arguments" migrations/migrations.go; then
    echo "✅ Migrations handle known compatibility errors"
else
    echo "⚠️  WARNING: Migrations may not handle known errors"
fi

# Check if migrations verify after creation
if grep -q "verifyExists\|verify.*exists" migrations/migrations.go; then
    echo "✅ Migrations verify table creation"
else
    echo "⚠️  WARNING: Migrations may not verify table creation"
fi

echo ""
echo "Step 5: Checking model definitions..."
echo ""

# Check for TableName methods
TABLE_NAME_COUNT=$(grep -r "func.*TableName" pkg/models/ | wc -l | tr -d ' ')
echo "Models with TableName() method: $TABLE_NAME_COUNT"

# Check for gorm.DeletedAt usage
DELETED_AT_COUNT=$(grep -r "gorm.DeletedAt" pkg/models/ | wc -l | tr -d ' ')
echo "Models using gorm.DeletedAt: $DELETED_AT_COUNT"

echo ""
echo "=========================================="
echo "Verification Complete"
echo "=========================================="
echo ""

cd ../..
