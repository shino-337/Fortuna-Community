#!/bin/bash

# Demo script to test ServiceAccount operations and verify KSAM sync

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

NAMESPACE="default"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}KSAM ServiceAccount Test Demo${NC}"
echo -e "${BLUE}========================================${NC}\n"

# Step 1: Create a test ServiceAccount
echo -e "${YELLOW}Step 1: Creating test ServiceAccount...${NC}"
"$SCRIPT_DIR/test-sa-operations.sh" -a create -n demo-test-sa -s "$NAMESPACE"
sleep 3

# Step 2: List ServiceAccounts
echo -e "\n${YELLOW}Step 2: Listing ServiceAccounts...${NC}"
"$SCRIPT_DIR/test-sa-operations.sh" -a list -s "$NAMESPACE"

# Step 3: Edit the ServiceAccount
echo -e "\n${YELLOW}Step 3: Editing ServiceAccount...${NC}"
"$SCRIPT_DIR/test-sa-operations.sh" -a edit -n demo-test-sa -s "$NAMESPACE"
sleep 3

# Step 4: Wait for sync
echo -e "\n${YELLOW}Step 4: Waiting for KSAM agent to sync (30 seconds)...${NC}"
echo "   This gives the agent time to detect and sync changes..."
for i in {30..1}; do
    echo -ne "\r   Waiting... ${i}s remaining"
    sleep 1
done
echo -e "\r   Waiting... Done!    \n"

# Step 5: Check agent logs
echo -e "${YELLOW}Step 5: Checking agent logs for sync activity...${NC}"
echo "   Recent agent logs:"
kubectl logs -n ksam -l app=ksam-agent --tail=10 2>/dev/null | grep -i "serviceaccount\|sync" | tail -5 || echo "   (No relevant logs found)"

# Step 6: Instructions
echo -e "\n${GREEN}========================================${NC}"
echo -e "${GREEN}Test Complete!${NC}"
echo -e "${GREEN}========================================${NC}\n"
echo -e "${BLUE}Next steps to verify:${NC}"
echo "  1. Open dashboard: http://localhost:3000/serviceaccounts"
echo "  2. Look for 'demo-test-sa' in the list"
echo "  3. Check audit logs: http://localhost:3000/audit"
echo "  4. View graph: http://localhost:3000/graph"
echo ""
echo -e "${YELLOW}To clean up the test ServiceAccount:${NC}"
echo "  ./scripts/test-sa-operations.sh -a delete -n demo-test-sa -s $NAMESPACE"

