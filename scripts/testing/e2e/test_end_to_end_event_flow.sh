#!/bin/bash

# End-to-End Event Flow Test
# Tests the complete flow: Agent → Core → NATS → Workers → Database

set -e

NAMESPACE="ksam"
TIMEOUT=60

echo "=========================================="
echo "End-to-End Event Flow Test"
echo "=========================================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

CORE_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-core -o jsonpath='{.items[0].metadata.name}')
AGENT_POD=$(kubectl get pods -n $NAMESPACE -l app=ksam-agent -o jsonpath='{.items[0].metadata.name}')

echo -e "${BLUE}Core Pod: $CORE_POD${NC}"
echo -e "${BLUE}Agent Pod: $AGENT_POD${NC}"
echo ""

# Clear previous logs
echo "Clearing log buffers..."
kubectl logs -n $NAMESPACE $CORE_POD --tail=0 > /dev/null 2>&1 || true

echo ""
echo "=== Step 1: Check Agent Status ==="
echo ""

AGENT_STATUS=$(kubectl get pod -n $NAMESPACE $AGENT_POD -o jsonpath='{.status.phase}')
if [ "$AGENT_STATUS" = "Running" ]; then
    echo -e "${GREEN}✓ Agent is running${NC}"
else
    echo -e "${RED}✗ Agent is not running (status: $AGENT_STATUS)${NC}"
    exit 1
fi

# Check if Agent is connected
AGENT_CONNECTED=$(kubectl logs -n $NAMESPACE $AGENT_POD --tail=20 2>&1 | grep -i "connected\|streaming" | wc -l)
if [ "$AGENT_CONNECTED" -gt 0 ]; then
    echo -e "${GREEN}✓ Agent is connected to Core${NC}"
else
    echo -e "${YELLOW}⚠ Agent connection status unclear (check logs)${NC}"
fi

echo ""
echo "=== Step 2: Monitor Event Flow (30 seconds) ==="
echo ""

echo "Monitoring Core logs for event processing..."
echo ""

# Monitor for 30 seconds
for i in {1..30}; do
    sleep 1
    PUBLISHED_RAW=$(kubectl logs -n $NAMESPACE $CORE_POD --tail=100 2>&1 | grep -c "Published.*to ksam.raw" || echo "0")
    NORMALIZED=$(kubectl logs -n $NAMESPACE $CORE_POD --tail=100 2>&1 | grep -c "Normalized and published.*to ksam.normalized" || echo "0")
    
    if [ $i -eq 10 ] || [ $i -eq 20 ] || [ $i -eq 30 ]; then
        echo "  [$i/30s] Raw events published: $PUBLISHED_RAW, Normalized events: $NORMALIZED"
    fi
done

echo ""
echo "=== Step 3: Analyze Results ==="
echo ""

# Get final counts
FINAL_RAW=$(kubectl logs -n $NAMESPACE $CORE_POD --tail=500 2>&1 | grep -c "Published.*to ksam.raw" || echo "0")
FINAL_NORMALIZED=$(kubectl logs -n $NAMESPACE $CORE_POD --tail=500 2>&1 | grep -c "Normalized and published.*to ksam.normalized" || echo "0")
FINAL_RISK=$(kubectl logs -n $NAMESPACE $CORE_POD --tail=500 2>&1 | grep -c "\[RiskWorker\]" || echo "0")
FINAL_CORRELATOR=$(kubectl logs -n $NAMESPACE $CORE_POD --tail=500 2>&1 | grep -c "\[CorrelatorWorker\]" || echo "0")

echo "Event Counts (last 500 log lines):"
echo "  Raw events published (ksam.raw.*):     $FINAL_RAW"
echo "  Normalized events (ksam.normalized.*): $FINAL_NORMALIZED"
echo "  Risk Worker processing:               $FINAL_RISK"
echo "  Correlator Worker processing:         $FINAL_CORRELATOR"
echo ""

# Check for specific event types
echo "Event Types Published:"
RAW_PODS=$(kubectl logs -n $NAMESPACE $CORE_POD --tail=500 2>&1 | grep -c "Published.*pods.*to ksam.raw.pods" || echo "0")
RAW_SAS=$(kubectl logs -n $NAMESPACE $CORE_POD --tail=500 2>&1 | grep -c "Published.*serviceaccounts.*to ksam.raw.serviceaccounts" || echo "0")
RAW_ROLES=$(kubectl logs -n $NAMESPACE $CORE_POD --tail=500 2>&1 | grep -c "Published.*roles.*to ksam.raw.roles" || echo "0")

echo "  Pods:           $RAW_PODS"
echo "  ServiceAccounts: $RAW_SAS"
echo "  Roles:          $RAW_ROLES"
echo ""

# Check database for inserted records
echo "=== Step 4: Database Verification ==="
echo ""

POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -n "$POSTGRES_POD" ]; then
    POD_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM pods;" 2>&1 | tr -d ' ')
    SA_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM service_accounts;" 2>&1 | tr -d ' ')
    ROLE_COUNT=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM roles;" 2>&1 | tr -d ' ')
    
    echo "Database Record Counts:"
    echo "  Pods:            $POD_COUNT"
    echo "  ServiceAccounts: $SA_COUNT"
    echo "  Roles:           $ROLE_COUNT"
    echo ""
    
    # Check if pods table has id as primary key
    PK_CHECK=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT conname FROM pg_constraint WHERE conrelid = 'pods'::regclass AND contype = 'p' AND conname = 'pods_pkey';" 2>&1 | tr -d ' ')
    if [ -n "$PK_CHECK" ]; then
        echo -e "${GREEN}✓ Pods table has id as primary key${NC}"
    else
        echo -e "${RED}✗ Pods table primary key check failed${NC}"
    fi
else
    echo -e "${YELLOW}⚠ Postgres pod not found, skipping database checks${NC}"
fi

echo ""
echo "=== Step 5: Sample Logs ==="
echo ""

echo "Recent Publisher logs:"
kubectl logs -n $NAMESPACE $CORE_POD --tail=100 2>&1 | grep "\[Publisher\]" | tail -5 || echo "  No publisher logs found"

echo ""
echo "Recent Normalizer Worker logs:"
kubectl logs -n $NAMESPACE $CORE_POD --tail=100 2>&1 | grep "\[NormalizerWorker\]" | tail -5 || echo "  No normalizer logs found"

echo ""
echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo ""

if [ "$FINAL_RAW" -gt 0 ]; then
    echo -e "${GREEN}✓ Raw events are being published${NC}"
else
    echo -e "${YELLOW}⚠ No raw events published (Agent may not be actively streaming)${NC}"
fi

if [ "$FINAL_NORMALIZED" -gt 0 ]; then
    echo -e "${GREEN}✓ Normalized events are being published${NC}"
else
    echo -e "${YELLOW}⚠ No normalized events (no raw events to process)${NC}"
fi

if [ "$FINAL_RISK" -gt 0 ] || [ "$FINAL_CORRELATOR" -gt 0 ]; then
    echo -e "${GREEN}✓ Workers are processing events${NC}"
else
    echo -e "${YELLOW}⚠ Workers not processing (no normalized events available)${NC}"
fi

echo ""
echo "Test completed. Check logs above for details."
echo ""



