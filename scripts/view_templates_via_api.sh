#!/bin/bash

# Script to view Policy Templates via API (alternative to direct DB query)

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m'

# API configuration
API_URL="${API_URL:-http://localhost:8080/api/v1}"

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_header() {
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}\n"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if API is accessible
check_api() {
    log_info "Checking API availability..."
    response=$(curl -s -o /dev/null -w "%{http_code}" "$API_URL/policies/templates" 2>/dev/null || echo "000")
    
    if [ "$response" = "200" ] || [ "$response" = "401" ]; then
        log_info "API is accessible (HTTP $response)"
        return 0
    else
        log_error "API is not accessible (HTTP $response)"
        return 1
    fi
}

# Get templates via API
get_templates() {
    log_header "Policy Templates via API"
    
    response=$(curl -s -w "\n%{http_code}" "$API_URL/policies/templates" 2>/dev/null)
    http_code=$(echo "$response" | tail -n 1)
    body=$(echo "$response" | head -n -1)
    
    if [ "$http_code" = "200" ]; then
        count=$(echo "$body" | jq -r '.count // (.templates | length) // 0' 2>/dev/null || echo "0")
        
        echo -e "${GREEN}Total Templates:${NC} $count"
        echo ""
        
        if [ "$count" -gt 0 ]; then
            echo "$body" | jq '.templates[] | {
                templateId: .templateId,
                version: .version,
                name: .name,
                category: .category,
                defaultSeverity: .defaultSeverity,
                defaultAction: .defaultAction,
                isSystem: .isSystem,
                created_at: .created_at
            }' 2>/dev/null || echo "$body" | jq '.templates' 2>/dev/null || echo "$body"
        else
            echo -e "${YELLOW}No templates found${NC}"
        fi
        
        # Save to file
        echo "$body" | jq . > "/tmp/policy_templates_$(date +%Y%m%d_%H%M%S).json" 2>/dev/null && \
            log_info "Full response saved to /tmp/policy_templates_*.json"
        
        return 0
    elif [ "$http_code" = "401" ]; then
        log_error "Authentication required. Please provide token:"
        echo "export API_TOKEN='your-token'"
        echo "Then run: curl -H 'Authorization: Bearer \$API_TOKEN' $API_URL/policies/templates"
        return 1
    else
        log_error "Failed to get templates (HTTP $http_code)"
        echo "$body"
        return 1
    fi
}

# Main
main() {
    if check_api; then
        get_templates
    else
        log_error "Cannot connect to API at $API_URL"
        echo ""
        echo "Please check:"
        echo "1. Core service is running"
        echo "2. API_URL is correct (current: $API_URL)"
        echo "3. Port forwarding is set up (if using Kubernetes)"
        echo ""
        echo "For Kubernetes:"
        echo "  kubectl port-forward -n ksam svc/core 8080:8080"
        echo ""
        echo "For Docker:"
        echo "  docker-compose up -d"
        exit 1
    fi
}

main

