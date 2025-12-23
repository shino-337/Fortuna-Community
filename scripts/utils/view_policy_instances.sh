#!/bin/bash

# Script to view Policy Instances in database

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

# Export password for psql
export PGPASSWORD="$DB_PASSWORD"

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_header() {
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}\n"
}

# Function to query database
db_query() {
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -A -c "$1" 2>/dev/null
}

# Function to query with formatting
db_query_formatted() {
    psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$1" 2>/dev/null
}

# Main function
main() {
    log_header "Policy Instances in Database"
    
    # Check if table exists
    log_info "Checking if policy_instances table exists..."
    table_exists=$(db_query "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'policy_instances');")
    
    if [ "$table_exists" != "t" ]; then
        echo -e "${YELLOW}[WARN]${NC} policy_instances table does not exist!"
        echo "Please run migrations first."
        exit 1
    fi
    
    log_info "Table exists. Querying instances...\n"
    
    # Get total count
    total_count=$(db_query "SELECT COUNT(*) FROM policy_instances WHERE deleted_at IS NULL;")
    deleted_count=$(db_query "SELECT COUNT(*) FROM policy_instances WHERE deleted_at IS NOT NULL;")
    
    echo -e "${GREEN}Active Instances:${NC} $total_count"
    echo -e "${YELLOW}Deleted Instances:${NC} $deleted_count"
    echo ""
    
    if [ "$total_count" -eq 0 ]; then
        echo -e "${YELLOW}[WARN]${NC} No active instances found in database."
        exit 0
    fi
    
    # Display instances in table format
    log_header "Instance Details"
    db_query_formatted "
        SELECT 
            instance_name as \"Instance Name\",
            template_id as \"Template ID\",
            template_version as \"Version\",
            enabled as \"Enabled\",
            action as \"Action\",
            severity as \"Severity\",
            array_to_string(clusters, ', ') as \"Clusters\",
            array_to_string(namespaces, ', ') as \"Namespaces\",
            array_to_string(resource_types, ', ') as \"Resource Types\",
            created_at::text as \"Created\"
        FROM policy_instances 
        WHERE deleted_at IS NULL 
        ORDER BY instance_name;
    "
    
    # Display full details for each instance
    log_header "Full Instance Details (JSON)"
    
    instances=$(db_query "
        SELECT json_agg(row_to_json(t)) 
        FROM (
            SELECT 
                instance_name,
                template_id,
                template_version,
                description,
                enabled,
                clusters,
                namespaces,
                resource_types,
                label_selectors,
                action,
                severity,
                custom_message,
                auto_remediate,
                remediation_dry_run,
                exemptions,
                created_at,
                updated_at
            FROM policy_instances 
            WHERE deleted_at IS NULL 
            ORDER BY instance_name
        ) t;
    ")
    
    if [ -n "$instances" ] && [ "$instances" != "NULL" ]; then
        echo "$instances" | jq '.' 2>/dev/null || echo "$instances"
    else
        echo "No instances found."
    fi
    
    # Display summary by template
    log_header "Summary by Template"
    db_query_formatted "
        SELECT 
            template_id as \"Template ID\",
            template_version as \"Version\",
            COUNT(*) as \"Instance Count\"
        FROM policy_instances 
        WHERE deleted_at IS NULL 
        GROUP BY template_id, template_version 
        ORDER BY template_id, template_version;
    "
    
    # Display summary by enabled status
    log_header "Summary by Enabled Status"
    db_query_formatted "
        SELECT 
            CASE 
                WHEN enabled THEN 'Enabled'
                ELSE 'Disabled'
            END as \"Status\",
            COUNT(*) as \"Count\"
        FROM policy_instances 
        WHERE deleted_at IS NULL 
        GROUP BY enabled 
        ORDER BY enabled DESC;
    "
    
    # Display summary by action
    log_header "Summary by Action"
    db_query_formatted "
        SELECT 
            COALESCE(action, 'Not Set') as \"Action\",
            COUNT(*) as \"Count\"
        FROM policy_instances 
        WHERE deleted_at IS NULL 
        GROUP BY action 
        ORDER BY action;
    "
    
    # Display summary by severity
    log_header "Summary by Severity"
    db_query_formatted "
        SELECT 
            COALESCE(severity, 'Not Set') as \"Severity\",
            COUNT(*) as \"Count\"
        FROM policy_instances 
        WHERE deleted_at IS NULL 
        GROUP BY severity 
        ORDER BY severity;
    "
    
    log_info "\nQuery completed successfully!"
}

# Run main
main

