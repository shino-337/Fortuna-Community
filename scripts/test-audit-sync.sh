#!/bin/bash

# Test script to verify audit log sync functionality
# This script creates and deletes a ServiceAccount and verifies audit logs are created

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Configuration
SA_NAME="test-audit-$(date +%s)"
NAMESPACE="default"
WAIT_TIME=90  # Wait time for agent sync (seconds)

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Audit Log Sync Test${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}ServiceAccount: ${SA_NAME}${NC}"
echo -e "${BLUE}Namespace: ${NAMESPACE}${NC}"
echo ""

# Get postgres pod name
POSTGRES_POD=$(kubectl get pods -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}')

if [ -z "$POSTGRES_POD" ]; then
    echo -e "${RED}Error: Postgres pod not found${NC}"
    exit 1
fi

# Function to check audit logs
check_audit_log() {
    local action=$1
    local sa_name=$2
    
    echo -e "${BLUE}Checking for audit log: action=${action}, name=${sa_name}${NC}"
    
    result=$(kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c \
        "SELECT COUNT(*) FROM audit_logs WHERE action = '${action}' AND details::text LIKE '%${sa_name}%' AND \"user\" = 'system' AND created_at > NOW() - INTERVAL '10 minutes';" 2>/dev/null | tr -d ' \n\r')
    
    if [ -z "$result" ]; then
        result=0
    fi
    
    if [ "$result" -gt 0 ]; then
        echo -e "${GREEN}✓ Audit log found for ${action} action${NC}"
        return 0
    else
        echo -e "${RED}✗ Audit log NOT found for ${action} action${NC}"
        return 1
    fi
}

# Function to get audit log details
get_audit_log_details() {
    local sa_name=$1
    
    echo -e "\n${BLUE}Audit logs for ${sa_name}:${NC}"
    kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -c \
        "SELECT id, action, resource, \"user\", ip, details::text, created_at FROM audit_logs WHERE details::text LIKE '%${sa_name}%' ORDER BY created_at DESC;" 2>&1
}

# Step 1: Create ServiceAccount
echo -e "${YELLOW}Step 1: Creating ServiceAccount...${NC}"
./scripts/test-sa-operations.sh -a create -n "$SA_NAME" -s "$NAMESPACE" > /dev/null 2>&1

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ ServiceAccount created${NC}"
else
    echo -e "${RED}✗ Failed to create ServiceAccount${NC}"
    exit 1
fi

# Wait for agent sync
echo -e "\n${YELLOW}Waiting ${WAIT_TIME} seconds for agent sync...${NC}"
sleep "$WAIT_TIME"

# Check for create audit log
echo -e "\n${YELLOW}Step 2: Checking for CREATE audit log...${NC}"
if check_audit_log "create" "$SA_NAME"; then
    CREATE_PASSED=true
else
    CREATE_PASSED=false
fi

# Step 3: Delete ServiceAccount
echo -e "\n${YELLOW}Step 3: Deleting ServiceAccount...${NC}"
./scripts/test-sa-operations.sh -a delete -n "$SA_NAME" -s "$NAMESPACE" > /dev/null 2>&1

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ ServiceAccount deleted${NC}"
else
    echo -e "${RED}✗ Failed to delete ServiceAccount${NC}"
    exit 1
fi

# Wait for agent sync and full sync
echo -e "\n${YELLOW}Waiting ${WAIT_TIME} seconds for agent sync and full sync...${NC}"
sleep "$WAIT_TIME"

# Check for delete audit log
echo -e "\n${YELLOW}Step 4: Checking for DELETE audit log...${NC}"
if check_audit_log "delete" "$SA_NAME"; then
    DELETE_PASSED=true
else
    DELETE_PASSED=false
fi

# Show audit log details
get_audit_log_details "$SA_NAME"

# Summary
echo -e "\n${BLUE}========================================${NC}"
echo -e "${BLUE}Test Summary${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "ServiceAccount: ${SA_NAME}"
echo -e "Namespace: ${NAMESPACE}"
echo ""

if [ "$CREATE_PASSED" = true ]; then
    echo -e "${GREEN}✓ CREATE audit log: PASSED${NC}"
else
    echo -e "${RED}✗ CREATE audit log: FAILED${NC}"
fi

if [ "$DELETE_PASSED" = true ]; then
    echo -e "${GREEN}✓ DELETE audit log: PASSED${NC}"
else
    echo -e "${RED}✗ DELETE audit log: FAILED${NC}"
fi

echo ""

if [ "$CREATE_PASSED" = true ] && [ "$DELETE_PASSED" = true ]; then
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}All tests PASSED!${NC}"
    echo -e "${GREEN}========================================${NC}"
    exit 0
else
    echo -e "${RED}========================================${NC}"
    echo -e "${RED}Some tests FAILED!${NC}"
    echo -e "${RED}========================================${NC}"
    exit 1
fi

