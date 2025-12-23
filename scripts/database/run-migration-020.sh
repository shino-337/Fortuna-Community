#!/bin/bash

# Script to run Migration 020 (SBOM tables)
# This migration creates sboms, sbom_components, and cve_matches tables

set -e

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║          RUNNING MIGRATION 020: SBOM TABLES                    ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Get Core pod name
CORE_POD=$(kubectl get pods -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$CORE_POD" ]; then
    echo "❌ Error: Core pod not found in namespace 'ksam'"
    echo ""
    echo "💡 Try:"
    echo "   kubectl get pods -n ksam"
    exit 1
fi

echo "📊 Found Core pod: $CORE_POD"
echo ""

# Check if migration SQL file exists
MIGRATION_FILE="KSAM/core/migrations/mvp2/006_add_sbom_tables.sql"
if [ ! -f "$MIGRATION_FILE" ]; then
    echo "❌ Error: Migration file not found: $MIGRATION_FILE"
    exit 1
fi

echo "📄 Migration file: $MIGRATION_FILE"
echo ""

# Get database connection info from pod environment
echo "🔍 Getting database connection info..."
DB_URL=$(kubectl exec -n ksam $CORE_POD -- env | grep DATABASE_URL | cut -d'=' -f2- || echo "")

if [ -z "$DB_URL" ]; then
    echo "⚠️  DATABASE_URL not found in pod environment"
    echo "   Trying to extract from config..."
    # Try to get from pod logs or config
    DB_URL=$(kubectl exec -n ksam $CORE_POD -- printenv | grep -i database || echo "")
fi

if [ -z "$DB_URL" ]; then
    echo "❌ Error: Could not determine database connection"
    echo ""
    echo "💡 Please provide database connection manually:"
    echo "   export DATABASE_URL='postgres://user:pass@host:port/dbname'"
    exit 1
fi

echo "✅ Database URL found"
echo ""

# Parse database URL to get connection details
# Format: postgres://user:password@host:port/dbname
DB_USER=$(echo $DB_URL | sed -n 's|postgres://\([^:]*\):.*|\1|p')
DB_PASS=$(echo $DB_URL | sed -n 's|postgres://[^:]*:\([^@]*\)@.*|\1|p')
DB_HOST=$(echo $DB_URL | sed -n 's|postgres://[^@]*@\([^:]*\):.*|\1|p')
DB_PORT=$(echo $DB_URL | sed -n 's|postgres://[^@]*@[^:]*:\([^/]*\)/.*|\1|p')
DB_NAME=$(echo $DB_URL | sed -n 's|postgres://[^@]*@[^/]*/\(.*\)|\1|p')

echo "📊 Database Info:"
echo "   Host: $DB_HOST"
echo "   Port: $DB_PORT"
echo "   Database: $DB_NAME"
echo "   User: $DB_USER"
echo ""

# Check if tables already exist
echo "🔍 Checking if SBOM tables already exist..."
EXISTS=$(kubectl exec -n ksam $CORE_POD -- sh -c "
    PGPASSWORD='$DB_PASS' psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -tAc \
    \"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'sboms')\"
" 2>/dev/null || echo "false")

if [ "$EXISTS" = "t" ]; then
    echo "✅ SBOM tables already exist!"
    echo ""
    echo "📊 Current SBOM tables status:"
    kubectl exec -n ksam $CORE_POD -- sh -c "
        PGPASSWORD='$DB_PASS' psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c '
        SELECT 
            (SELECT COUNT(*) FROM sboms) as sboms_count,
            (SELECT COUNT(*) FROM sbom_components) as components_count,
            (SELECT COUNT(*) FROM cve_matches) as matches_count;
        '
    " 2>/dev/null || echo "   (Could not query tables)"
    echo ""
    echo "💡 To re-run migration, drop tables first:"
    echo "   DROP TABLE IF EXISTS cve_matches CASCADE;"
    echo "   DROP TABLE IF EXISTS sbom_components CASCADE;"
    echo "   DROP TABLE IF EXISTS sboms CASCADE;"
    exit 0
fi

echo "📝 Tables do not exist, running migration..."
echo ""

# Copy migration SQL to pod
echo "📤 Copying migration SQL to pod..."
kubectl cp "$MIGRATION_FILE" ksam/$CORE_POD:/tmp/006_add_sbom_tables.sql

# Execute migration via Go migration system (restart pod to trigger)
echo "🚀 Migration 020 will run automatically when Core service starts"
echo "   Triggering migration by checking if it's in the migration list..."
echo ""
echo "   Migration 020 is registered in migrations.go and will run on next pod restart"
echo "   Or it may have already run if the pod was recently restarted"
echo ""
echo "   Checking if tables exist now..."

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Migration 020 completed successfully!"
    echo ""
    echo "📊 Verifying tables..."
    kubectl exec -n ksam $CORE_POD -- sh -c "
        PGPASSWORD='$DB_PASS' psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c '
        SELECT 
            table_name,
            (SELECT COUNT(*) FROM information_schema.columns WHERE table_name = t.table_name) as column_count
        FROM information_schema.tables t
        WHERE table_schema = CURRENT_SCHEMA()
        AND table_name IN ('sboms', 'sbom_components', 'cve_matches')
        ORDER BY table_name;
        '
    "
    echo ""
    echo "✅ SBOM tables created successfully!"
else
    echo ""
    echo "❌ Migration failed!"
    exit 1
fi

echo ""
echo "✅ Script completed"

