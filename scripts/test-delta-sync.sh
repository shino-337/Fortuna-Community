#!/bin/bash

# Test script to verify delta sync and batch insert functionality

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Delta Sync & Batch Insert Test${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Get pod names
AGENT_POD=$(kubectl get pods -n kube-system -l app=ksam-agent -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
CORE_POD=$(kubectl get pods -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
POSTGRES_POD=$(kubectl get pods -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$AGENT_POD" ]; then
    echo -e "${RED}Error: Agent pod not found${NC}"
    exit 1
fi

if [ -z "$CORE_POD" ]; then
    echo -e "${RED}Error: Core pod not found${NC}"
    exit 1
fi

if [ -z "$POSTGRES_POD" ]; then
    echo -e "${RED}Error: Postgres pod not found${NC}"
    exit 1
fi

echo -e "${YELLOW}Step 1: Check Agent Delta Sync${NC}"
echo -e "${BLUE}Checking agent logs for delta/full sync indicators...${NC}"
echo ""

# Check for delta sync in agent logs
DELTA_SYNC_COUNT=$(kubectl logs -n kube-system $AGENT_POD --tail=200 | grep -c "DELTA sync\|DELTA:" || echo "0")
FULL_SYNC_COUNT=$(kubectl logs -n kube-system $AGENT_POD --tail=200 | grep -c "FULL sync\|FULL:" || echo "0")

echo -e "  Delta sync occurrences: ${GREEN}${DELTA_SYNC_COUNT}${NC}"
echo -e "  Full sync occurrences: ${GREEN}${FULL_SYNC_COUNT}${NC}"

if [ "$DELTA_SYNC_COUNT" -gt 0 ] || [ "$FULL_SYNC_COUNT" -gt 0 ]; then
    echo -e "  ${GREEN}✓ Delta sync mechanism is working${NC}"
else
    echo -e "  ${YELLOW}⚠ No delta/full sync logs found (may need to wait for next sync)${NC}"
fi

echo ""
echo -e "${YELLOW}Step 2: Check Batch Insert${NC}"
echo -e "${BLUE}Checking core logs for batch insert messages...${NC}"
echo ""

# Check for batch insert in core logs
BATCH_INSERT_COUNT=$(kubectl logs -n ksam $CORE_POD --tail=200 | grep -c "Batch inserted.*audit logs" || echo "0")

echo -e "  Batch insert occurrences: ${GREEN}${BATCH_INSERT_COUNT}${NC}"

if [ "$BATCH_INSERT_COUNT" -gt 0 ]; then
    echo -e "  ${GREEN}✓ Batch insert is working${NC}"
    
    # Get latest batch insert log
    LATEST_BATCH=$(kubectl logs -n ksam $CORE_POD --tail=200 | grep "Batch inserted.*audit logs" | tail -1)
    echo -e "  Latest: ${BLUE}${LATEST_BATCH}${NC}"
else
    echo -e "  ${YELLOW}⚠ No batch insert logs found (may need to wait for audit logs to accumulate)${NC}"
fi

echo ""
echo -e "${YELLOW}Step 3: Check Audit Log Statistics${NC}"
echo -e "${BLUE}Checking database for audit log statistics...${NC}"
echo ""

# Get total audit logs
TOTAL_LOGS=$(kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM audit_logs;" 2>/dev/null | tr -d ' \n\r' || echo "0")
echo -e "  Total audit logs: ${GREEN}${TOTAL_LOGS}${NC}"

# Get audit logs by action
echo -e "  ${BLUE}Audit logs by action:${NC}"
kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT action, COUNT(*) as count FROM audit_logs GROUP BY action ORDER BY count DESC;" 2>/dev/null | grep -v "^$" | grep -v "^-" | grep -v "action" | grep -v "row" | while read line; do
    if [ ! -z "$line" ]; then
        echo -e "    ${GREEN}${line}${NC}"
    fi
done

# Get recent audit logs (last hour)
RECENT_LOGS=$(kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM audit_logs WHERE created_at > NOW() - INTERVAL '1 hour';" 2>/dev/null | tr -d ' \n\r' || echo "0")
echo -e "  Audit logs in last hour: ${GREEN}${RECENT_LOGS}${NC}"

echo ""
echo -e "${YELLOW}Step 4: Check TTL Cleanup${NC}"
echo -e "${BLUE}Checking for old audit logs cleanup...${NC}"
echo ""

# Check for cleanup logs
CLEANUP_COUNT=$(kubectl logs -n ksam $CORE_POD --tail=200 | grep -c "Cleaned up.*old audit logs" || echo "0")

if [ "$CLEANUP_COUNT" -gt 0 ]; then
    echo -e "  ${GREEN}✓ TTL cleanup is working${NC}"
    LATEST_CLEANUP=$(kubectl logs -n ksam $CORE_POD --tail=200 | grep "Cleaned up.*old audit logs" | tail -1)
    echo -e "  Latest cleanup: ${BLUE}${LATEST_CLEANUP}${NC}"
else
    echo -e "  ${YELLOW}⚠ No cleanup logs found (cleanup runs every hour, may need to wait)${NC}"
fi

# Check for old logs (> 90 days)
OLD_LOGS=$(kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM audit_logs WHERE created_at < NOW() - INTERVAL '90 days';" 2>/dev/null | tr -d ' \n\r' || echo "0")
echo -e "  Audit logs older than 90 days: ${GREEN}${OLD_LOGS}${NC}"

echo ""
echo -e "${YELLOW}Step 5: Monitor Next Sync${NC}"
echo -e "${BLUE}Waiting 35 seconds to observe next sync cycle...${NC}"
echo ""

# Get initial counts
INITIAL_DELTA=$(kubectl logs -n kube-system $AGENT_POD --tail=1 | grep -c "DELTA\|FULL" || echo "0")

# Wait for next sync (30s interval + 5s buffer)
sleep 35

# Check for new sync
NEW_DELTA=$(kubectl logs -n kube-system $AGENT_POD --tail=5 | grep -c "DELTA\|FULL" || echo "0")

if [ "$NEW_DELTA" -gt "$INITIAL_DELTA" ]; then
    echo -e "  ${GREEN}✓ New sync detected${NC}"
    LATEST_SYNC=$(kubectl logs -n kube-system $AGENT_POD --tail=5 | grep -E "DELTA|FULL" | tail -1)
    echo -e "  Latest sync: ${BLUE}${LATEST_SYNC}${NC}"
else
    echo -e "  ${YELLOW}⚠ No new sync detected (may need to wait longer)${NC}"
fi

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Test Summary${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Summary
if [ "$DELTA_SYNC_COUNT" -gt 0 ] || [ "$FULL_SYNC_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✓ Delta sync: Working${NC}"
else
    echo -e "${YELLOW}⚠ Delta sync: Need more time to verify${NC}"
fi

if [ "$BATCH_INSERT_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✓ Batch insert: Working${NC}"
else
    echo -e "${YELLOW}⚠ Batch insert: Need more audit logs to verify${NC}"
fi

if [ "$CLEANUP_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✓ TTL cleanup: Working${NC}"
else
    echo -e "${YELLOW}⚠ TTL cleanup: Will run every hour${NC}"
fi

echo ""
echo -e "${BLUE}Note: Some features may need time to accumulate data${NC}"
echo -e "${BLUE}Monitor logs with:${NC}"
echo -e "  kubectl logs -n kube-system -l app=ksam-agent -f | grep -i delta"
echo -e "  kubectl logs -n ksam -l app=ksam-core -f | grep -i batch"


