#!/bin/bash

# Script to view Policy Templates in Kubernetes database
# Usage: ./view_templates_k8s.sh [postgres-pod-name]

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

NAMESPACE="${NAMESPACE:-ksam}"
POSTGRES_POD="${1:-postgres-6d84b5b778-n2cvk}"

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_header() {
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}\n"
}

# Check if pod exists
check_pod() {
    if ! kubectl get pod -n "$NAMESPACE" "$POSTGRES_POD" &>/dev/null; then
        echo -e "${YELLOW}[WARN]${NC} Pod $POSTGRES_POD not found. Finding postgres pod..."
        POSTGRES_POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
        if [ -z "$POSTGRES_POD" ]; then
            echo "Available pods:"
            kubectl get pods -n "$NAMESPACE" | grep postgres
            exit 1
        fi
        log_info "Using pod: $POSTGRES_POD"
    fi
}

# Query database
db_query() {
    kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U postgres -d ksam -c "$1" 2>&1
}

# Main
main() {
    log_header "Policy Templates in Kubernetes Database"
    
    check_pod
    
    log_info "Querying database from pod: $POSTGRES_POD"
    
    # Get count
    log_header "Template Count"
    db_query "SELECT COUNT(*) as total FROM policy_templates;"
    
    # Get templates
    log_header "Template List"
    db_query "
        SELECT 
            template_id as \"Template ID\",
            version as \"Version\",
            name as \"Name\",
            category as \"Category\",
            default_severity as \"Severity\",
            default_action as \"Action\",
            is_system as \"System\",
            created_at::date as \"Created\"
        FROM policy_templates 
        ORDER BY template_id, version;
    "
    
    # Get summary by category
    log_header "Summary by Category"
    db_query "
        SELECT 
            category as \"Category\",
            COUNT(*) as \"Count\"
        FROM policy_templates 
        GROUP BY category 
        ORDER BY category;
    "
    
    # Get summary by action
    log_header "Summary by Action"
    db_query "
        SELECT 
            COALESCE(default_action, 'Not Set') as \"Action\",
            COUNT(*) as \"Count\"
        FROM policy_templates 
        GROUP BY default_action 
        ORDER BY default_action;
    "
    
    log_info "\nQuery completed!"
}

main

