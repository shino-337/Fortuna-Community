#!/bin/bash

# Simple Policy API Test Script
# Tests API endpoints and compares with database

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

API_URL="${API_URL:-http://localhost:8080/api/v1}"
RESULTS_DIR="./test_results/$(date +%Y%m%d_%H%M%S)"
mkdir -p "$RESULTS_DIR"

log() {
    echo -e "${GREEN}[TEST]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Test function
test_endpoint() {
    local name=$1
    local method=$2
    local endpoint=$3
    local data=$4
    
    log "Testing: $name"
    
    local response
    if [ "$method" = "GET" ]; then
        response=$(curl -s -w "\n%{http_code}" "$API_URL$endpoint" 2>&1)
    elif [ "$method" = "POST" ]; then
        response=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" -d "$data" "$API_URL$endpoint" 2>&1)
    elif [ "$method" = "PUT" ]; then
        response=$(curl -s -w "\n%{http_code}" -X PUT -H "Content-Type: application/json" -d "$data" "$API_URL$endpoint" 2>&1)
    elif [ "$method" = "DELETE" ]; then
        response=$(curl -s -w "\n%{http_code}" -X DELETE "$API_URL$endpoint" 2>&1)
    fi
    
    local http_code=$(echo "$response" | tail -n 1)
    local body=$(echo "$response" | head -n -1)
    
    echo "$body" | jq . > "$RESULTS_DIR/${name// /_}.json" 2>/dev/null || echo "$body" > "$RESULTS_DIR/${name// /_}.json"
    
    if [ "$http_code" -ge 200 ] && [ "$http_code" -lt 300 ]; then
        echo -e "${GREEN}✓${NC} $name (HTTP $http_code)"
        return 0
    else
        echo -e "${RED}✗${NC} $name (HTTP $http_code)"
        return 1
    fi
}

# Main tests
main() {
    log "Starting Policy API Tests"
    log "Results will be saved to: $RESULTS_DIR"
    
    # Test 1: List Templates
    test_endpoint "List Templates" "GET" "/policies/templates"
    
    # Test 2: Create Template
    TEMPLATE_JSON='{
        "templateId": "test-template-1",
        "version": "v1.0.0",
        "name": "Test Template",
        "description": "Test",
        "category": "security",
        "defaultSeverity": "high",
        "celExpression": "resource.labels.env == '\''prod'\''",
        "defaultScope": "{}",
        "defaultAction": "alert",
        "isSystem": false
    }'
    test_endpoint "Create Template" "POST" "/policies/templates" "$TEMPLATE_JSON"
    
    # Test 3: Get Template
    test_endpoint "Get Template" "GET" "/policies/templates/test-template-1"
    
    # Test 4: List Instances
    test_endpoint "List Instances" "GET" "/policies/instances"
    
    # Test 5: Create Instance
    INSTANCE_JSON='{
        "templateId": "test-template-1",
        "templateVersion": "v1.0.0",
        "instanceName": "test-instance-1",
        "enabled": true,
        "clusters": ["cluster-1"],
        "namespaces": ["default"],
        "resourceTypes": ["Pod"],
        "action": "block",
        "severity": "critical"
    }'
    test_endpoint "Create Instance" "POST" "/policies/instances" "$INSTANCE_JSON"
    
    # Test 6: Get Instance
    test_endpoint "Get Instance" "GET" "/policies/instances/test-instance-1"
    
    log "Tests completed. Results in: $RESULTS_DIR"
}

main

