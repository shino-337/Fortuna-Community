#!/bin/bash

# Script to view Policy Templates in database

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
    log_header "Policy Templates in Database"
    
    # Check if table exists
    log_info "Checking if policy_templates table exists..."
    table_exists=$(db_query "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'policy_templates');")
    
    if [ "$table_exists" != "t" ]; then
        echo -e "${YELLOW}[WARN]${NC} policy_templates table does not exist!"
        echo "Please run migrations first."
        exit 1
    fi
    
    log_info "Table exists. Querying templates...\n"
    
    # Get total count
    total_count=$(db_query "SELECT COUNT(*) FROM policy_templates WHERE deleted_at IS NULL;")
    deleted_count=$(db_query "SELECT COUNT(*) FROM policy_templates WHERE deleted_at IS NOT NULL;")
    
    echo -e "${GREEN}Active Templates:${NC} $total_count"
    echo -e "${YELLOW}Deleted Templates:${NC} $deleted_count"
    echo ""
    
    if [ "$total_count" -eq 0 ]; then
        echo -e "${YELLOW}[WARN]${NC} No active templates found in database."
        exit 0
    fi
    
    # Display templates in table format
    log_header "Template Details"
    db_query_formatted "
        SELECT 
            template_id as \"Template ID\",
            version as \"Version\",
            name as \"Name\",
            category as \"Category\",
            default_severity as \"Severity\",
            default_action as \"Action\",
            is_system as \"System\",
            created_at::text as \"Created\"
        FROM policy_templates 
        WHERE deleted_at IS NULL 
        ORDER BY template_id, version;
    "
    
    # Display full details for each template
    log_header "Full Template Details (JSON)"
    
    templates=$(db_query "
        SELECT json_agg(row_to_json(t)) 
        FROM (
            SELECT 
                template_id,
                version,
                name,
                description,
                category,
                default_severity,
                cel_expression,
                default_scope,
                default_action,
                supports_remediation,
                remediation_template,
                rationale,
                references,
                examples,
                is_system,
                created_at,
                updated_at
            FROM policy_templates 
            WHERE deleted_at IS NULL 
            ORDER BY template_id, version
        ) t;
    ")
    
    if [ -n "$templates" ] && [ "$templates" != "NULL" ]; then
        echo "$templates" | jq '.' 2>/dev/null || echo "$templates"
    else
        echo "No templates found."
    fi
    
    # Display summary by category
    log_header "Summary by Category"
    db_query_formatted "
        SELECT 
            category as \"Category\",
            COUNT(*) as \"Count\"
        FROM policy_templates 
        WHERE deleted_at IS NULL 
        GROUP BY category 
        ORDER BY category;
    "
    
    # Display summary by action
    log_header "Summary by Action"
    db_query_formatted "
        SELECT 
            default_action as \"Action\",
            COUNT(*) as \"Count\"
        FROM policy_templates 
        WHERE deleted_at IS NULL 
        GROUP BY default_action 
        ORDER BY default_action;
    "
    
    # Display summary by severity
    log_header "Summary by Severity"
    db_query_formatted "
        SELECT 
            default_severity as \"Severity\",
            COUNT(*) as \"Count\"
        FROM policy_templates 
        WHERE deleted_at IS NULL 
        GROUP BY default_severity 
        ORDER BY default_severity;
    "
    
    # Display system vs non-system
    log_header "System vs Non-System Templates"
    db_query_formatted "
        SELECT 
            CASE 
                WHEN is_system THEN 'System'
                ELSE 'User'
            END as \"Type\",
            COUNT(*) as \"Count\"
        FROM policy_templates 
        WHERE deleted_at IS NULL 
        GROUP BY is_system 
        ORDER BY is_system DESC;
    "
    
    log_info "\nQuery completed successfully!"
}

# Run main
main

