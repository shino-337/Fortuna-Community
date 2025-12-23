#!/bin/bash

# Complete test suite for V2 scorer
set -e

echo "=========================================="
echo "V2 SCORER COMPLETE TEST SUITE"
echo "=========================================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

POD_NAME=$(kubectl get pod -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -z "$POD_NAME" ]; then
    echo -e "${RED}❌ Pod not found${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Pod: $POD_NAME${NC}"
echo ""

# Test 1: Health check
echo -e "${YELLOW}Test 1: Health Check...${NC}"
kubectl port-forward -n ksam pod/$POD_NAME 8080:8080 > /tmp/pf-test.log 2>&1 &
PF_PID=$!
sleep 3

HEALTH=$(curl -s http://localhost:8080/health 2>&1)
if echo "$HEALTH" | grep -q "ok\|healthy"; then
    echo -e "${GREEN}✅ Health check passed${NC}"
else
    echo -e "${RED}❌ Health check failed: $HEALTH${NC}"
fi
echo ""

# Test 2: Risk scores API
echo -e "${YELLOW}Test 2: Risk Scores API...${NC}"
SCORES=$(curl -s http://localhost:8080/api/v1/risk/scores?limit=5 2>&1)
if echo "$SCORES" | grep -q "scores\|total"; then
    echo -e "${GREEN}✅ Risk scores API accessible${NC}"
    # Check for V2 fields
    if echo "$SCORES" | grep -q "scorerVersion\|exploitabilityScore\|businessImpactScore"; then
        echo -e "${GREEN}✅ V2 fields present in response${NC}"
    else
        echo -e "${YELLOW}⚠️  V2 fields not found in response${NC}"
    fi
else
    echo -e "${YELLOW}⚠️  Risk scores API response: $SCORES${NC}"
fi
echo ""

# Test 3: Calculate risk score
echo -e "${YELLOW}Test 3: Calculate Risk Score...${NC}"
# Get a resource UID from database or use a test one
CALC=$(curl -s -X POST http://localhost:8080/api/v1/risk/scores/test-uid-123/calculate 2>&1)
if echo "$CALC" | grep -q "totalScore\|scorerVersion"; then
    echo -e "${GREEN}✅ Calculate risk score API works${NC}"
    if echo "$CALC" | grep -q "v2"; then
        echo -e "${GREEN}✅ V2 scorer is being used${NC}"
    fi
else
    echo -e "${YELLOW}⚠️  Calculate response: $CALC${NC}"
fi
echo ""

# Test 4: Metrics
echo -e "${YELLOW}Test 4: Metrics Endpoint...${NC}"
METRICS=$(curl -s http://localhost:8080/metrics 2>&1 | head -20)
if [ -n "$METRICS" ]; then
    echo -e "${GREEN}✅ Metrics endpoint accessible${NC}"
else
    echo -e "${YELLOW}⚠️  Metrics endpoint not accessible${NC}"
fi
echo ""

# Cleanup
kill $PF_PID 2>/dev/null || true

# Test 5: Pod logs check
echo -e "${YELLOW}Test 5: Pod Logs Check...${NC}"
LOGS=$(kubectl logs -n ksam $POD_NAME --tail=100 2>&1 | grep -i "v2\|scorer\|error" | head -10)
if [ -n "$LOGS" ]; then
    echo "Recent logs:"
    echo "$LOGS"
else
    echo -e "${YELLOW}⚠️  No relevant logs found${NC}"
fi
echo ""

echo "=========================================="
echo -e "${GREEN}✅ TEST SUITE COMPLETE${NC}"
echo "=========================================="


