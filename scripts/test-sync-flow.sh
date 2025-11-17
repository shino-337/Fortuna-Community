#!/bin/bash

# Test script to verify ServiceAccount sync flow from K8s to Dashboard
# This script tests:
# 1. Creating a ServiceAccount in K8s
# 2. Verifying it appears in database
# 3. Verifying it appears in Dashboard API
# 4. Deleting the ServiceAccount
# 5. Verifying it's removed from database and Dashboard

set -e

NAMESPACE="${NAMESPACE:-default}"
TEST_SA_NAME="test-sync-$(date +%s)"
CORE_API="${CORE_API:-http://localhost:8080/api/v1}"
TOKEN="${TOKEN:-}"

echo "=========================================="
echo "Testing ServiceAccount Sync Flow"
echo "=========================================="
echo "Test ServiceAccount: $TEST_SA_NAME"
echo "Namespace: $NAMESPACE"
echo ""

# Function to get auth token if needed
get_token() {
  if [ -z "$TOKEN" ]; then
    echo "Getting auth token..."
    TOKEN=$(curl -s -X POST "$CORE_API/auth/login" \
      -H "Content-Type: application/json" \
      -d '{"username":"admin","password":"admin123"}' | jq -r '.token')
    if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
      echo "ERROR: Failed to get auth token"
      exit 1
    fi
    echo "Token obtained"
  fi
}

# Function to make authenticated API call
api_call() {
  local method=$1
  local endpoint=$2
  local data=$3
  
  if [ -n "$data" ]; then
    curl -s -X "$method" "$CORE_API$endpoint" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $TOKEN" \
      -d "$data"
  else
    curl -s -X "$method" "$CORE_API$endpoint" \
      -H "Authorization: Bearer $TOKEN"
  fi
}

# Step 1: Get auth token
get_token

# Step 2: Create ServiceAccount in K8s
echo ""
echo "Step 1: Creating ServiceAccount in Kubernetes..."
kubectl create serviceaccount "$TEST_SA_NAME" -n "$NAMESPACE"
echo "✓ ServiceAccount created in K8s"

# Step 3: Wait for Agent to sync (max 60 seconds)
echo ""
echo "Step 2: Waiting for Agent to sync (checking every 5 seconds, max 60s)..."
MAX_WAIT=60
ELAPSED=0
FOUND=false

while [ $ELAPSED -lt $MAX_WAIT ]; do
  sleep 5
  ELAPSED=$((ELAPSED + 5))
  
  # Check if ServiceAccount appears in API
  SA_LIST=$(api_call GET "/serviceaccounts?namespace=$NAMESPACE" | jq -r '.serviceAccounts[] | select(.name == "'"$TEST_SA_NAME"'") | .id')
  
  if [ -n "$SA_LIST" ]; then
    SA_ID=$(echo "$SA_LIST" | head -1)
    FOUND=true
    echo "✓ ServiceAccount found in database (ID: $SA_ID) after ${ELAPSED}s"
    break
  fi
  
  echo "  Waiting... (${ELAPSED}s)"
done

if [ "$FOUND" = false ]; then
  echo "✗ ERROR: ServiceAccount not found in database after ${MAX_WAIT}s"
  echo "  This indicates the Agent is not syncing properly"
  kubectl delete serviceaccount "$TEST_SA_NAME" -n "$NAMESPACE" 2>/dev/null || true
  exit 1
fi

# Step 4: Verify ServiceAccount details
echo ""
echo "Step 3: Verifying ServiceAccount details..."
SA_DETAILS=$(api_call GET "/serviceaccounts/$SA_ID")
SA_NAME=$(echo "$SA_DETAILS" | jq -r '.name')
SA_NS=$(echo "$SA_DETAILS" | jq -r '.namespace')

if [ "$SA_NAME" = "$TEST_SA_NAME" ] && [ "$SA_NS" = "$NAMESPACE" ]; then
  echo "✓ ServiceAccount details verified"
  echo "  Name: $SA_NAME"
  echo "  Namespace: $SA_NS"
else
  echo "✗ ERROR: ServiceAccount details mismatch"
  echo "  Expected: name=$TEST_SA_NAME, namespace=$NAMESPACE"
  echo "  Got: name=$SA_NAME, namespace=$SA_NS"
  kubectl delete serviceaccount "$TEST_SA_NAME" -n "$NAMESPACE" 2>/dev/null || true
  exit 1
fi

# Step 5: Delete ServiceAccount from K8s
echo ""
echo "Step 4: Deleting ServiceAccount from Kubernetes..."
kubectl delete serviceaccount "$TEST_SA_NAME" -n "$NAMESPACE"
echo "✓ ServiceAccount deleted from K8s"

# Step 6: Wait for deletion to sync (max 60 seconds)
echo ""
echo "Step 5: Waiting for deletion to sync (checking every 5 seconds, max 60s)..."
ELAPSED=0
DELETED=false

while [ $ELAPSED -lt $MAX_WAIT ]; do
  sleep 5
  ELAPSED=$((ELAPSED + 5))
  
  # Check if ServiceAccount is removed from API
  SA_LIST=$(api_call GET "/serviceaccounts?namespace=$NAMESPACE" | jq -r '.serviceAccounts[] | select(.name == "'"$TEST_SA_NAME"'") | .id')
  
  if [ -z "$SA_LIST" ]; then
    DELETED=true
    echo "✓ ServiceAccount removed from database after ${ELAPSED}s"
    break
  fi
  
  echo "  Waiting... (${ELAPSED}s)"
done

if [ "$DELETED" = false ]; then
  echo "✗ WARNING: ServiceAccount still in database after ${MAX_WAIT}s"
  echo "  This may indicate the Agent is not syncing deletions properly"
  echo "  Note: The ServiceAccount may be soft-deleted in the database"
  exit 1
fi

# Summary
echo ""
echo "=========================================="
echo "✓ All tests passed!"
echo "=========================================="
echo "ServiceAccount sync flow verified:"
echo "  1. ✓ Created in K8s"
echo "  2. ✓ Synced to database"
echo "  3. ✓ Visible in API"
echo "  4. ✓ Deleted from K8s"
echo "  5. ✓ Removed from database"
echo ""

