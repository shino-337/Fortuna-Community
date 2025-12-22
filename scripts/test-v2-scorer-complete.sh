#!/bin/bash

# Complete Test for V2 Scorer
# Tests migration, database columns, and API endpoints

set -e

echo "=========================================="
echo "COMPLETE V2 SCORER TEST"
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

# Test 1: Check migration logs
echo -e "${YELLOW}Test 1: Checking Migration018 logs...${NC}"
MIGRATION_LOGS=$(kubectl logs -n ksam $POD_NAME 2>&1 | grep -i "Migration018\|V2 scoring columns\|migration 018" | head -10)
if [ -n "$MIGRATION_LOGS" ]; then
    echo -e "${GREEN}✅ Migration018 found in logs${NC}"
    echo "$MIGRATION_LOGS"
else
    echo -e "${YELLOW}⚠️  Migration018 not found (may have run on startup)${NC}"
    # Check if migration ran by checking startup logs
    STARTUP_LOGS=$(kubectl logs -n ksam $POD_NAME 2>&1 | grep -i "Running database migrations\|Total migrations" | head -5)
    if [ -n "$STARTUP_LOGS" ]; then
        echo "Startup migration logs:"
        echo "$STARTUP_LOGS"
    fi
fi
echo ""

# Test 2: Check database columns
echo -e "${YELLOW}Test 2: Checking database columns...${NC}"
DB_COLUMNS=$(kubectl exec -n ksam $POD_NAME -- psql -U ksam -d ksam -c "\d risk_scores" 2>&1)
if echo "$DB_COLUMNS" | grep -q "exploitability_score"; then
    echo -e "${GREEN}✅ exploitability_score column exists${NC}"
else
    echo -e "${RED}❌ exploitability_score column NOT found${NC}"
fi
if echo "$DB_COLUMNS" | grep -q "business_impact_score"; then
    echo -e "${GREEN}✅ business_impact_score column exists${NC}"
else
    echo -e "${RED}❌ business_impact_score column NOT found${NC}"
fi
if echo "$DB_COLUMNS" | grep -q "scorer_version"; then
    echo -e "${GREEN}✅ scorer_version column exists${NC}"
else
    echo -e "${RED}❌ scorer_version column NOT found${NC}"
fi
echo ""

# Test 3: Check if columns have data
echo -e "${YELLOW}Test 3: Checking column data...${NC}"
COLUMN_DATA=$(kubectl exec -n ksam $POD_NAME -- psql -U ksam -d ksam -c "SELECT COUNT(*) as total, COUNT(exploitability_score) as has_exploit, COUNT(business_impact_score) as has_impact, COUNT(scorer_version) as has_version FROM risk_scores LIMIT 5;" 2>&1)
echo "$COLUMN_DATA"
echo ""

# Test 4: Port forward and test API
echo -e "${YELLOW}Test 4: Testing API endpoints...${NC}"
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

# Test metrics endpoint
echo "Testing metrics endpoint..."
METRICS=$(curl -s http://localhost:8080/metrics 2>&1 | head -30)
if [ -n "$METRICS" ]; then
    echo -e "${GREEN}✅ Metrics endpoint accessible${NC}"
    RISK_METRICS=$(echo "$METRICS" | grep -i "risk\|scorer" | head -5)
    if [ -n "$RISK_METRICS" ]; then
        echo "Risk-related metrics:"
        echo "$RISK_METRICS"
    fi
else
    echo -e "${YELLOW}⚠️  Metrics endpoint not accessible${NC}"
fi
echo ""

# Cleanup
kill $PF_PID 2>/dev/null || true

# Test 5: Verify pod is running
echo -e "${YELLOW}Test 5: Verifying pod status...${NC}"
POD_STATUS=$(kubectl get pod -n ksam $POD_NAME -o jsonpath='{.status.phase}')
if [ "$POD_STATUS" = "Running" ]; then
    echo -e "${GREEN}✅ Pod is Running${NC}"
else
    echo -e "${RED}❌ Pod status: $POD_STATUS${NC}"
fi
echo ""

# Test 6: Check for V2 scorer code
echo -e "${YELLOW}Test 6: Checking for V2 scorer references...${NC}"
V2_LOGS=$(kubectl logs -n ksam $POD_NAME 2>&1 | grep -i "scorer_v2\|ScorerV2\|V2 scoring" | head -5)
if [ -n "$V2_LOGS" ]; then
    echo -e "${GREEN}✅ V2 scorer references found${NC}"
    echo "$V2_LOGS"
else
    echo -e "${YELLOW}⚠️  No V2 scorer logs found (may not be active yet)${NC}"
fi
echo ""

echo "=========================================="
echo -e "${GREEN}✅ TEST COMPLETE${NC}"
echo "=========================================="


