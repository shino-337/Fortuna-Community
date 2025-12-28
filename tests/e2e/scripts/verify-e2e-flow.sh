#!/bin/bash

# E2E Flow Verification Script
# This script queries database and API to verify the complete processing flow

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="${NAMESPACE:-fortuna}"
TEST_POD_NAME="${TEST_POD_NAME:-test-pod-e2e-1766897554}"
POD_UID="${POD_UID:-9caa6290-5471-46de-9a0b-a43ba7937d9d}"

# Get pod names
POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
CORE_POD=$(kubectl get pods -n $NAMESPACE -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=core' -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [ -z "$POSTGRES_POD" ]; then
    echo -e "${RED}❌ PostgreSQL pod not found${NC}"
    exit 1
fi

if [ -z "$CORE_POD" ]; then
    echo -e "${RED}❌ Core pod not found${NC}"
    exit 1
fi

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}E2E Flow Verification${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo "Test Pod: $TEST_POD_NAME"
echo "Pod UID: $POD_UID"
echo ""

# Function to execute SQL query
db_query() {
    kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -A -F'|' -c "$1" 2>/dev/null
}

# Function to execute SQL query with headers
db_query_header() {
    kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "$1" 2>/dev/null
}

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}1. POD INFORMATION${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""
echo "Checking if pod exists in database..."
POD_INFO=$(db_query_header "SELECT uid, name, namespace, created_at FROM pods WHERE uid = '$POD_UID' OR name = '$TEST_POD_NAME' LIMIT 1;")
if [ -n "$POD_INFO" ] && echo "$POD_INFO" | grep -q "uid"; then
    echo -e "${GREEN}✅ Pod found in database:${NC}"
    echo "$POD_INFO"
else
    echo -e "${YELLOW}⚠️  Pod not found in database (may be normal if not yet synced)${NC}"
fi
echo ""

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}2. SBOM VERIFICATION${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""
echo "Querying SBOM for test pod..."
SBOM_QUERY="SELECT id, pod_uid, pod_name, namespace, image_name, image_digest, created_at FROM sboms WHERE pod_uid = '$POD_UID' OR pod_name = '$TEST_POD_NAME' ORDER BY created_at DESC LIMIT 5;"
SBOM_RESULT=$(db_query_header "$SBOM_QUERY")

if echo "$SBOM_RESULT" | grep -q "id"; then
    echo -e "${GREEN}✅ SBOM found:${NC}"
    echo "$SBOM_RESULT"
    SBOM_ID=$(db_query "SELECT id FROM sboms WHERE pod_uid = '$POD_UID' OR pod_name = '$TEST_POD_NAME' ORDER BY created_at DESC LIMIT 1;")
    SBOM_ID=$(echo "$SBOM_ID" | tr -d ' ' | head -1)
    echo ""
    echo "SBOM ID: $SBOM_ID"
    export SBOM_ID
else
    echo -e "${YELLOW}⚠️  SBOM not found yet (may still be processing)${NC}"
    SBOM_ID=""
fi
echo ""

if [ -n "$SBOM_ID" ] && [ "$SBOM_ID" != "" ]; then
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW}3. SBOM COMPONENTS VERIFICATION${NC}"
    echo -e "${YELLOW}========================================${NC}"
    echo ""
    echo "Querying SBOM components..."
    COMPONENTS_QUERY="SELECT COUNT(*) as total_components, COUNT(DISTINCT name) as unique_packages FROM sbom_components WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL;"
    COMPONENTS_RESULT=$(db_query_header "$COMPONENTS_QUERY")
    echo -e "${GREEN}✅ Components:${NC}"
    echo "$COMPONENTS_RESULT"
    echo ""
    
    echo "Sample components (first 10):"
    SAMPLE_COMPONENTS=$(db_query_header "SELECT name, version, package_type FROM sbom_components WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL LIMIT 10;")
    echo "$SAMPLE_COMPONENTS"
    echo ""
    
    echo -e "${YELLOW}========================================${NC}"
    echo -e "${YELLOW}4. CVE MATCHES VERIFICATION${NC}"
    echo -e "${YELLOW}========================================${NC}"
    echo ""
    echo "Querying CVE matches..."
    CVE_QUERY="SELECT COUNT(*) as total_matches, COUNT(DISTINCT cve_id) as unique_cves FROM cve_matches WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL;"
    CVE_RESULT=$(db_query_header "$CVE_QUERY")
    echo -e "${GREEN}✅ CVE Matches:${NC}"
    echo "$CVE_RESULT"
    echo ""
    
    if echo "$CVE_RESULT" | grep -q "total_matches.*[1-9]"; then
        echo "Sample CVE matches (first 10):"
        SAMPLE_CVES=$(db_query_header "SELECT cve_id, package_name, package_version, cvss_score, matched_by FROM cve_matches WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL LIMIT 10;")
        echo "$SAMPLE_CVES"
        echo ""
    fi
else
    echo -e "${YELLOW}⚠️  Skipping components and CVE verification (SBOM not found)${NC}"
    echo ""
fi

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}5. INSIGHTS VERIFICATION${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""
echo "Querying insights for test pod..."
INSIGHTS_QUERY="SELECT COUNT(*) as total_insights, COUNT(DISTINCT cve_id) as unique_cves, string_agg(DISTINCT insight_type, ', ') as types FROM insights WHERE resource_uid = '$POD_UID' AND deleted_at IS NULL;"
INSIGHTS_RESULT=$(db_query_header "$INSIGHTS_QUERY")
echo -e "${GREEN}✅ Insights Summary:${NC}"
echo "$INSIGHTS_RESULT"
echo ""

INSIGHT_COUNT=$(db_query "SELECT COUNT(*) FROM insights WHERE resource_uid = '$POD_UID' AND deleted_at IS NULL;")
INSIGHT_COUNT=$(echo "$INSIGHT_COUNT" | tr -d ' ')

if [ -n "$INSIGHT_COUNT" ] && [ "$INSIGHT_COUNT" -gt 0 ]; then
    echo "Sample insights (first 10):"
    SAMPLE_INSIGHTS=$(db_query_header "SELECT id, resource_uid, resource_type, resource_name, insight_type, severity, cvss, created_at FROM insights WHERE resource_uid = '$POD_UID' AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 10;")
    echo "$SAMPLE_INSIGHTS"
    echo ""
fi

echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}6. API VERIFICATION${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""
echo "Setting up port-forward..."
kubectl port-forward -n $NAMESPACE $CORE_POD 8080:8080 > /tmp/port-forward.log 2>&1 &
PORT_FORWARD_PID=$!
sleep 3

echo "Querying API for insights..."
API_URL="http://localhost:8080/api/v1/insights?resource_uid=$POD_UID"
echo "URL: $API_URL"
echo ""

API_RESPONSE=$(curl -s "$API_URL" 2>&1 || echo "{}")

if echo "$API_RESPONSE" | grep -q "insights"; then
    echo -e "${GREEN}✅ API Response:${NC}"
    echo "$API_RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$API_RESPONSE"
    echo ""
    
    # Extract total count
    TOTAL=$(echo "$API_RESPONSE" | grep -o '"total":[0-9]*' | cut -d: -f2 || echo "0")
    echo "Total insights from API: $TOTAL"
    
    if [ "$TOTAL" -gt 0 ]; then
        echo ""
        echo "Sample insight from API:"
        echo "$API_RESPONSE" | python3 -m json.tool 2>/dev/null | grep -A 20 '"insights"' | head -30 || echo "$API_RESPONSE" | head -50
    fi
else
    echo -e "${YELLOW}⚠️  API response:${NC}"
    echo "$API_RESPONSE"
fi

# Cleanup port-forward
kill $PORT_FORWARD_PID 2>/dev/null || true
sleep 1

echo ""
echo -e "${YELLOW}========================================${NC}"
echo -e "${YELLOW}7. PROCESSING TIMELINE${NC}"
echo -e "${YELLOW}========================================${NC}"
echo ""

if [ -n "$SBOM_ID" ] && [ "$SBOM_ID" != "" ]; then
    echo "SBOM Timeline:"
    SBOM_TIMELINE=$(db_query_header "SELECT created_at as sbom_created FROM sboms WHERE id = $SBOM_ID;")
    echo "$SBOM_TIMELINE"
    echo ""
    
    CVE_TIMELINE=$(db_query_header "SELECT MIN(created_at) as first_cve_match FROM cve_matches WHERE sbom_id = $SBOM_ID;")
    if echo "$CVE_TIMELINE" | grep -q "[0-9]"; then
        echo "CVE Matching Timeline:"
        echo "$CVE_TIMELINE"
        echo ""
    fi
fi

INSIGHT_TIMELINE=$(db_query_header "SELECT MIN(created_at) as first_insight FROM insights WHERE resource_uid = '$POD_UID';")
if echo "$INSIGHT_TIMELINE" | grep -q "[0-9]"; then
    echo "Insight Generation Timeline:"
    echo "$INSIGHT_TIMELINE"
    echo ""
fi

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Verification Complete${NC}"
echo -e "${BLUE}========================================${NC}"

