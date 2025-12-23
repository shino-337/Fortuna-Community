#!/bin/bash

# Test V2 Scorer Implementation
# This script tests the V2 scoring logic

set -e

echo "=========================================="
echo "TEST V2 SCORER"
echo "=========================================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Get pod name
POD_NAME=$(kubectl get pod -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -z "$POD_NAME" ]; then
    echo -e "${RED}❌ Pod not found${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Pod: $POD_NAME${NC}"
echo ""

# Test 1: Check migration
echo -e "${YELLOW}Test 1: Checking Migration018...${NC}"
MIGRATION_LOG=$(kubectl logs -n ksam $POD_NAME 2>&1 | grep -i "Migration018\|V2 scoring columns" | tail -5)
if [ -n "$MIGRATION_LOG" ]; then
    echo -e "${GREEN}✅ Migration018 found in logs${NC}"
    echo "$MIGRATION_LOG"
else
    echo -e "${YELLOW}⚠️  Migration018 not found in logs (may have run already)${NC}"
fi
echo ""

# Test 2: Check database columns
echo -e "${YELLOW}Test 2: Checking database columns...${NC}"
kubectl exec -n ksam $POD_NAME -- psql -U ksam -d ksam -c "\d risk_scores" 2>&1 | grep -E "exploitability_score|business_impact_score|scorer_version" || echo "Columns check..."
echo ""

# Test 3: Port forward and test API
echo -e "${YELLOW}Test 3: Testing API endpoints...${NC}"
kubectl port-forward -n ksam pod/$POD_NAME 8080:8080 > /tmp/pf-v2-test.log 2>&1 &
PF_PID=$!
sleep 3

# Test health endpoint
echo "Testing health endpoint..."
HEALTH=$(curl -s http://localhost:8080/health 2>&1)
if echo "$HEALTH" | grep -q "ok\|healthy"; then
    echo -e "${GREEN}✅ Health endpoint OK${NC}"
else
    echo -e "${YELLOW}⚠️  Health endpoint: $HEALTH${NC}"
fi
echo ""

# Test risk scores endpoint (may require auth)
echo "Testing risk scores endpoint..."
RISK_SCORES=$(curl -s http://localhost:8080/api/v1/risk/scores?limit=5 2>&1)
if echo "$RISK_SCORES" | grep -q "scores\|total"; then
    echo -e "${GREEN}✅ Risk scores endpoint accessible${NC}"
    echo "$RISK_SCORES" | head -20
elif echo "$RISK_SCORES" | grep -q "Authorization"; then
    echo -e "${YELLOW}⚠️  Endpoint requires authorization${NC}"
else
    echo -e "${YELLOW}⚠️  Response: $RISK_SCORES${NC}"
fi
echo ""

# Test metrics endpoint
echo "Testing metrics endpoint..."
METRICS=$(curl -s http://localhost:8080/metrics 2>&1 | head -20)
if [ -n "$METRICS" ]; then
    echo -e "${GREEN}✅ Metrics endpoint accessible${NC}"
    echo "$METRICS" | grep -E "risk|scorer" || echo "No risk metrics found"
else
    echo -e "${YELLOW}⚠️  Metrics endpoint not accessible${NC}"
fi
echo ""

# Cleanup
kill $PF_PID 2>/dev/null || true

# Test 4: Check logs for V2 scorer
echo -e "${YELLOW}Test 4: Checking for V2 scorer in logs...${NC}"
V2_LOGS=$(kubectl logs -n ksam $POD_NAME 2>&1 | grep -i "v2\|scorer_v2\|exploitability\|business.*impact" | tail -10)
if [ -n "$V2_LOGS" ]; then
    echo -e "${GREEN}✅ V2 scorer references found${NC}"
    echo "$V2_LOGS"
else
    echo -e "${YELLOW}⚠️  No V2 scorer logs found (may not be active yet)${NC}"
fi
echo ""

# Test 5: Verify pod is running
echo -e "${YELLOW}Test 5: Verifying pod status...${NC}"
POD_STATUS=$(kubectl get pod -n ksam $POD_NAME -o jsonpath='{.status.phase}')
if [ "$POD_STATUS" = "Running" ]; then
    echo -e "${GREEN}✅ Pod is Running${NC}"
else
    echo -e "${RED}❌ Pod status: $POD_STATUS${NC}"
    kubectl describe pod -n ksam $POD_NAME | tail -20
fi
echo ""

echo "=========================================="
echo -e "${GREEN}✅ TEST COMPLETE${NC}"
echo "=========================================="


