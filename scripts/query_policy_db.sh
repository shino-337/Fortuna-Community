#!/bin/bash

# Quick database query script for Policy Engine
# Usage: ./query_policy_db.sh [templates|instances|both|count]

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Database configuration
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-ksam}"
DB_USER="${DB_USER:-ksam}"
DB_PASSWORD="${DB_PASSWORD:-ksam}"

export PGPASSWORD="$DB_PASSWORD"

query_type="${1:-both}"

# Quick query function
quick_query() {
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$1" 2>/dev/null
}

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Policy Engine Database Query${NC}"
echo -e "${BLUE}========================================${NC}\n"

case "$query_type" in
    "templates"|"template"|"t")
        echo -e "${GREEN}Policy Templates:${NC}\n"
        quick_query "
            SELECT 
                template_id,
                version,
                name,
                category,
                default_severity,
                default_action,
                is_system,
                created_at::date as created
            FROM policy_templates 
            WHERE deleted_at IS NULL 
            ORDER BY template_id, version;
        "
        ;;
    "instances"|"instance"|"i")
        echo -e "${GREEN}Policy Instances:${NC}\n"
        quick_query "
            SELECT 
                instance_name,
                template_id,
                template_version,
                enabled,
                action,
                severity,
                created_at::date as created
            FROM policy_instances 
            WHERE deleted_at IS NULL 
            ORDER BY instance_name;
        "
        ;;
    "count"|"c")
        echo -e "${GREEN}Count Summary:${NC}\n"
        quick_query "
            SELECT 
                'Templates' as type,
                COUNT(*) FILTER (WHERE deleted_at IS NULL) as active,
                COUNT(*) FILTER (WHERE deleted_at IS NOT NULL) as deleted
            FROM policy_templates
            UNION ALL
            SELECT 
                'Instances' as type,
                COUNT(*) FILTER (WHERE deleted_at IS NULL) as active,
                COUNT(*) FILTER (WHERE deleted_at IS NOT NULL) as deleted
            FROM policy_instances;
        "
        ;;
    "both"|"all"|*)
        echo -e "${GREEN}Policy Templates:${NC}\n"
        quick_query "
            SELECT 
                template_id,
                version,
                name,
                category,
                default_severity,
                default_action
            FROM policy_templates 
            WHERE deleted_at IS NULL 
            ORDER BY template_id, version
            LIMIT 10;
        "
        echo -e "\n${GREEN}Policy Instances:${NC}\n"
        quick_query "
            SELECT 
                instance_name,
                template_id,
                template_version,
                enabled,
                action,
                severity
            FROM policy_instances 
            WHERE deleted_at IS NULL 
            ORDER BY instance_name
            LIMIT 10;
        "
        ;;
esac

echo ""

