#!/bin/bash

# Test script to measure sync timing from agent to frontend

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Configuration
SA_NAME="test-sync-timing-$(date +%s)"
NAMESPACE="default"
MAX_WAIT=300  # Maximum wait time in seconds (5 minutes)

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Sync Timing Test${NC}"
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

# Function to check if ServiceAccount exists in DB
check_sa_in_db() {
    local sa_name=$1
    
    result=$(kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c \
        "SELECT COUNT(*) FROM service_accounts WHERE name = '${sa_name}' AND namespace = '${NAMESPACE}';" 2>/dev/null | tr -d ' \n\r')
    
    if [ -z "$result" ]; then
        result=0
    fi
    
    if [ "$result" -gt 0 ]; then
        return 0
    else
        return 1
    fi
}

# Function to check if ServiceAccount is deleted from DB
check_sa_deleted_from_db() {
    local sa_name=$1
    
    result=$(kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c \
        "SELECT COUNT(*) FROM service_accounts WHERE name = '${sa_name}' AND namespace = '${NAMESPACE}' AND deleted_at IS NULL;" 2>/dev/null | tr -d ' \n\r')
    
    if [ -z "$result" ]; then
        result=0
    fi
    
    if [ "$result" -eq 0 ]; then
        return 0
    else
        return 1
    fi
}

# Step 1: Create ServiceAccount
echo -e "${YELLOW}Step 1: Creating ServiceAccount...${NC}"
START_TIME=$(date +%s)
./scripts/test-sa-operations.sh -a create -n "$SA_NAME" -s "$NAMESPACE" > /dev/null 2>&1

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ ServiceAccount created at $(date +%H:%M:%S)${NC}"
else
    echo -e "${RED}✗ Failed to create ServiceAccount${NC}"
    exit 1
fi

# Step 2: Wait for sync and measure time
echo -e "\n${YELLOW}Step 2: Waiting for agent to sync to database...${NC}"
echo -e "${BLUE}Checking every 5 seconds (max ${MAX_WAIT}s)...${NC}"

SYNC_TIME=0
FOUND=false

for i in $(seq 1 $((MAX_WAIT / 5))); do
    sleep 5
    SYNC_TIME=$((SYNC_TIME + 5))
    
    if check_sa_in_db "$SA_NAME"; then
        FOUND=true
        END_TIME=$(date +%s)
        ELAPSED=$((END_TIME - START_TIME))
        echo -e "${GREEN}✓ ServiceAccount found in database after ${ELAPSED} seconds (${SYNC_TIME}s wait)${NC}"
        break
    fi
    
    if [ $((i % 6)) -eq 0 ]; then
        echo -e "${BLUE}  Still waiting... (${SYNC_TIME}s elapsed)${NC}"
    fi
done

if [ "$FOUND" = false ]; then
    echo -e "${RED}✗ ServiceAccount not found in database after ${MAX_WAIT} seconds${NC}"
    exit 1
fi

# Step 3: Delete ServiceAccount
echo -e "\n${YELLOW}Step 3: Deleting ServiceAccount...${NC}"
DELETE_START_TIME=$(date +%s)
./scripts/test-sa-operations.sh -a delete -n "$SA_NAME" -s "$NAMESPACE" > /dev/null 2>&1

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ ServiceAccount deleted at $(date +%H:%M:%S)${NC}"
else
    echo -e "${RED}✗ Failed to delete ServiceAccount${NC}"
    exit 1
fi

# Step 4: Wait for deletion sync
echo -e "\n${YELLOW}Step 4: Waiting for deletion to sync to database...${NC}"
echo -e "${BLUE}Checking every 5 seconds (max ${MAX_WAIT}s)...${NC}"

DELETE_SYNC_TIME=0
DELETED=false

for i in $(seq 1 $((MAX_WAIT / 5))); do
    sleep 5
    DELETE_SYNC_TIME=$((DELETE_SYNC_TIME + 5))
    
    if check_sa_deleted_from_db "$SA_NAME"; then
        DELETED=true
        DELETE_END_TIME=$(date +%s)
        DELETE_ELAPSED=$((DELETE_END_TIME - DELETE_START_TIME))
        echo -e "${GREEN}✓ ServiceAccount deletion synced to database after ${DELETE_ELAPSED} seconds (${DELETE_SYNC_TIME}s wait)${NC}"
        break
    fi
    
    if [ $((i % 6)) -eq 0 ]; then
        echo -e "${BLUE}  Still waiting... (${DELETE_SYNC_TIME}s elapsed)${NC}"
    fi
done

if [ "$DELETED" = false ]; then
    echo -e "${RED}✗ ServiceAccount deletion not synced after ${MAX_WAIT} seconds${NC}"
    exit 1
fi

# Summary
echo -e "\n${BLUE}========================================${NC}"
echo -e "${BLUE}Sync Timing Summary${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "ServiceAccount: ${SA_NAME}"
echo -e "Namespace: ${NAMESPACE}"
echo ""
echo -e "${GREEN}CREATE sync time: ${ELAPSED} seconds${NC}"
echo -e "${GREEN}DELETE sync time: ${DELETE_ELAPSED} seconds${NC}"
echo ""
echo -e "${BLUE}Note: Frontend may need additional time to refresh${NC}"
echo -e "${BLUE}Frontend refresh depends on React Query cache settings${NC}"

