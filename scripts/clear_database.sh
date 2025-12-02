#!/bin/bash

# Script to clear/reset the database
# Usage: ./scripts/clear_database.sh [database_url]

set -e

# Default database URL
DEFAULT_DB_URL="postgres://postgres:postgres@localhost:5432/ksam?sslmode=disable"
DB_URL="${1:-$DEFAULT_DB_URL}"

echo "⚠️  WARNING: This will DROP ALL TABLES in the database!"
echo "Database URL: $DB_URL"
read -p "Are you sure you want to continue? (yes/no): " confirm

if [ "$confirm" != "yes" ]; then
    echo "Aborted."
    exit 1
fi

echo "Clearing database..."

# Extract connection details from URL
# Format: postgres://user:password@host:port/database?params
DB_USER=$(echo $DB_URL | sed -n 's|.*://\([^:]*\):.*|\1|p')
DB_PASS=$(echo $DB_URL | sed -n 's|.*://[^:]*:\([^@]*\)@.*|\1|p')
DB_HOST=$(echo $DB_URL | sed -n 's|.*@\([^:]*\):.*|\1|p')
DB_PORT=$(echo $DB_URL | sed -n 's|.*@[^:]*:\([^/]*\)/.*|\1|p')
DB_NAME=$(echo $DB_URL | sed -n 's|.*/\([^?]*\).*|\1|p')

echo "Connecting to: $DB_USER@$DB_HOST:$DB_PORT/$DB_NAME"

# Use PGPASSWORD environment variable for password
export PGPASSWORD="$DB_PASS"

# Drop all tables
psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" <<EOF
-- Drop all tables in public schema
DO \$\$ 
DECLARE 
    r RECORD;
BEGIN
    FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') 
    LOOP
        EXECUTE 'DROP TABLE IF EXISTS ' || quote_ident(r.tablename) || ' CASCADE';
    END LOOP;
END \$\$;

-- Drop all sequences
DO \$\$ 
DECLARE 
    r RECORD;
BEGIN
    FOR r IN (SELECT sequence_name FROM information_schema.sequences WHERE sequence_schema = 'public') 
    LOOP
        EXECUTE 'DROP SEQUENCE IF EXISTS ' || quote_ident(r.sequence_name) || ' CASCADE';
    END LOOP;
END \$\$;
EOF

echo "✅ Database cleared successfully!"
echo "Run migrations to recreate tables: go run core/cmd/main.go"


