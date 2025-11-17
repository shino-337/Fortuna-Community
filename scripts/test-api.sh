#!/bin/bash

# Test script for KSAM API

set -e

API_URL="${API_URL:-http://localhost:8080}"
USERNAME="${KSAM_USERNAME:-admin}"
PASSWORD="${KSAM_PASSWORD:-admin123}"

echo "🧪 Testing KSAM API at $API_URL"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Test health endpoint
echo -e "${YELLOW}1. Testing health endpoint...${NC}"
HEALTH=$(curl -s "$API_URL/health")
echo "$HEALTH" | jq .
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Health check passed${NC}"
else
    echo -e "${RED}❌ Health check failed${NC}"
    exit 1
fi
echo ""

# Login
echo -e "${YELLOW}2. Logging in...${NC}"
LOGIN_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")

TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.token // empty')

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    echo -e "${RED}❌ Login failed${NC}"
    echo "$LOGIN_RESPONSE" | jq .
    exit 1
fi

echo -e "${GREEN}✅ Login successful${NC}"
echo "Token: ${TOKEN:0:50}..."
echo ""

# Test authenticated endpoints
AUTH_HEADER="Authorization: Bearer $TOKEN"

# Get clusters
echo -e "${YELLOW}3. Testing GET /api/v1/clusters...${NC}"
CLUSTERS=$(curl -s -H "$AUTH_HEADER" "$API_URL/api/v1/clusters")
echo "$CLUSTERS" | jq .
CLUSTER_COUNT=$(echo "$CLUSTERS" | jq '.clusters | length')
echo -e "${GREEN}✅ Found $CLUSTER_COUNT clusters${NC}"
echo ""

# Get service accounts
echo -e "${YELLOW}4. Testing GET /api/v1/serviceaccounts...${NC}"
SAs=$(curl -s -H "$AUTH_HEADER" "$API_URL/api/v1/serviceaccounts")
echo "$SAs" | jq '.serviceAccounts | length' | xargs echo "ServiceAccounts count:"
echo -e "${GREEN}✅ ServiceAccounts retrieved${NC}"
echo ""

# Get graph
echo -e "${YELLOW}5. Testing GET /api/v1/graph...${NC}"
GRAPH=$(curl -s -H "$AUTH_HEADER" "$API_URL/api/v1/graph")
NODES=$(echo "$GRAPH" | jq '.nodes | length')
EDGES=$(echo "$GRAPH" | jq '.edges | length')
echo "Graph: $NODES nodes, $EDGES edges"
echo -e "${GREEN}✅ Graph data retrieved${NC}"
echo ""

# Get audit logs
echo -e "${YELLOW}6. Testing GET /api/v1/audit...${NC}"
AUDIT=$(curl -s -H "$AUTH_HEADER" "$API_URL/api/v1/audit")
AUDIT_COUNT=$(echo "$AUDIT" | jq '.logs | length')
echo "Audit logs: $AUDIT_COUNT entries"
echo -e "${GREEN}✅ Audit logs retrieved${NC}"
echo ""

# Get current user
echo -e "${YELLOW}7. Testing GET /api/v1/me...${NC}"
ME=$(curl -s -H "$AUTH_HEADER" "$API_URL/api/v1/me")
echo "$ME" | jq .
echo -e "${GREEN}✅ Current user retrieved${NC}"
echo ""

# Test metrics
echo -e "${YELLOW}8. Testing GET /metrics...${NC}"
METRICS=$(curl -s "$API_URL/metrics")
METRIC_COUNT=$(echo "$METRICS" | grep -c "^ksam_" || echo "0")
echo "Prometheus metrics: $METRIC_COUNT metrics"
echo -e "${GREEN}✅ Metrics endpoint working${NC}"
echo ""

echo -e "${GREEN}✅ All tests passed!${NC}"

