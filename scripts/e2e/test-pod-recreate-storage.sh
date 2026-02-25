#!/usr/bin/env bash

# Test Pod Recreate Storage Behavior
# Tests how database stores data when:
# 1. Same pod deployed twice (same UID)
# 2. Pod deleted and recreated (new UID)

set -euo pipefail

POD_NAME="test-pod-recreate"
POD_NAMESPACE="default"
NAMESPACE="${NAMESPACE:-fortuna}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./common.sh
source "${SCRIPT_DIR}/common.sh"

echo "=========================================="
echo "Pod Recreate Storage Test"
echo "=========================================="
echo ""

# Function to get pod UID
get_pod_uid() {
    kubectl get pod -n "$POD_NAMESPACE" "$POD_NAME" -o jsonpath='{.metadata.uid}' 2>/dev/null || echo ""
}

# Function to query database
query_db() {
    local query="$1"
    local pg_pod
    pg_pod="$(require_postgres_pod)"
    kubectl -n "$NAMESPACE" exec "$pg_pod" -- psql -U postgres -d fortuna -t -A -F '|' -c "$query" 2>/dev/null || echo ""
}

# Function to count records
count_records() {
    local table="$1"
    local pod_uid="$2"
    if [ "$table" = "pods" ]; then
      query_db "SELECT COUNT(*) FROM pods WHERE uid = '$pod_uid' AND deleted_at IS NULL;" | tr -d ' '
      return
    fi
    query_db "SELECT COUNT(*) FROM $table WHERE pod_uid = '$pod_uid';" | tr -d ' '
}

echo "Step 1: Create initial pod"
echo "----------------------------------------"
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $POD_NAME
  namespace: $POD_NAMESPACE
spec:
  containers:
  - name: test
    image: alpine:latest
    command: ["sleep", "3600"]
    securityContext:
      privileged: true
  restartPolicy: Never
EOF

sleep 5
POD_UID_1=$(get_pod_uid)
echo "✅ Pod created with UID: $POD_UID_1"
echo ""

# Wait for agent to sync
echo "Waiting for agent sync (30s)..."
if WAITED_1="$(wait_for_pod_in_db "$POD_UID_1" 360 "$(require_postgres_pod)")"; then
    echo "✅ Pod UID1 appeared in DB after ${WAITED_1}s"
else
    echo "❌ Pod UID1 not found in DB after 360s"
    exit 1
fi

# Inject runtime event
echo "Injecting runtime event for pod $POD_UID_1..."
CORE_POD="$(require_core_pod)"
TOKEN="$(require_jwt_token "$CORE_POD")"
core_api_post_json "runtime-events" "$TOKEN" "$CORE_POD" "[{\"pod_uid\":\"$POD_UID_1\",\"namespace\":\"$POD_NAMESPACE\",\"syscall\":\"openat\",\"target_path\":\"/proc/1/root\",\"capability\":\"SYS_ADMIN\",\"timestamp\":$(date +%s)}]" >/dev/null
sleep 3

echo ""
echo "Step 2: Check data for UID 1"
echo "----------------------------------------"
PODS_COUNT_1=$(count_records "pods" "$POD_UID_1")
CAPS_COUNT_1=$(count_records "pod_capabilities" "$POD_UID_1")
EVENTS_COUNT_1=$(count_records "runtime_events" "$POD_UID_1")
PROFILES_COUNT_1=$(count_records "pod_risk_profiles" "$POD_UID_1")

echo "Pods: $PODS_COUNT_1"
echo "Capabilities: $CAPS_COUNT_1"
echo "Runtime Events: $EVENTS_COUNT_1"
echo "Risk Profiles: $PROFILES_COUNT_1"
echo ""

echo "Step 3: Delete pod"
echo "----------------------------------------"
kubectl delete pod -n "$POD_NAMESPACE" "$POD_NAME" --wait=false
kubectl wait --for=delete "pod/$POD_NAME" -n "$POD_NAMESPACE" --timeout=120s >/dev/null 2>&1 || sleep 10

# Check if pod is soft-deleted in DB
POD_DELETED=$(query_db "SELECT CASE WHEN deleted_at IS NULL THEN 'false' ELSE 'true' END FROM pods WHERE uid = '$POD_UID_1' ORDER BY updated_at DESC LIMIT 1;" | tr -d ' ')
echo "Pod soft-deleted in DB: $POD_DELETED"
echo ""

# Check data after deletion
echo "Step 4: Check data after deletion (UID 1)"
echo "----------------------------------------"
CAPS_COUNT_1_AFTER=$(count_records "pod_capabilities" "$POD_UID_1")
EVENTS_COUNT_1_AFTER=$(count_records "runtime_events" "$POD_UID_1")
PROFILES_COUNT_1_AFTER=$(count_records "pod_risk_profiles" "$POD_UID_1")

echo "Capabilities (after delete): $CAPS_COUNT_1_AFTER"
echo "Runtime Events (after delete): $EVENTS_COUNT_1_AFTER"
echo "Risk Profiles (after delete): $PROFILES_COUNT_1_AFTER"
echo ""

echo "Step 5: Recreate pod (new UID)"
echo "----------------------------------------"
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $POD_NAME
  namespace: $POD_NAMESPACE
spec:
  containers:
  - name: test
    image: alpine:latest
    command: ["sleep", "3600"]
    securityContext:
      privileged: true
  restartPolicy: Never
EOF

sleep 5
POD_UID_2=$(get_pod_uid)
echo "✅ Pod recreated with NEW UID: $POD_UID_2"
echo ""

if [ "$POD_UID_1" = "$POD_UID_2" ]; then
    echo "⚠️  WARNING: UIDs are the same! This shouldn't happen for a new pod."
else
    echo "✅ UIDs are different (expected for new pod)"
fi
echo ""

# Wait for agent sync
echo "Waiting for agent sync (30s)..."
if WAITED_2="$(wait_for_pod_in_db "$POD_UID_2" 360 "$(require_postgres_pod)")"; then
    echo "✅ Pod UID2 appeared in DB after ${WAITED_2}s"
else
    echo "❌ Pod UID2 not found in DB after 360s"
    exit 1
fi

# Inject runtime event for new pod
echo "Injecting runtime event for pod $POD_UID_2..."
core_api_post_json "runtime-events" "$TOKEN" "$CORE_POD" "[{\"pod_uid\":\"$POD_UID_2\",\"namespace\":\"$POD_NAMESPACE\",\"syscall\":\"openat\",\"target_path\":\"/proc/1/root\",\"capability\":\"SYS_ADMIN\",\"timestamp\":$(date +%s)}]" >/dev/null
sleep 3

echo ""
echo "Step 6: Check data for UID 2"
echo "----------------------------------------"
PODS_COUNT_2=$(count_records "pods" "$POD_UID_2")
CAPS_COUNT_2=$(count_records "pod_capabilities" "$POD_UID_2")
EVENTS_COUNT_2=$(count_records "runtime_events" "$POD_UID_2")
PROFILES_COUNT_2=$(count_records "pod_risk_profiles" "$POD_UID_2")

echo "Pods: $PODS_COUNT_2"
echo "Capabilities: $CAPS_COUNT_2"
echo "Runtime Events: $EVENTS_COUNT_2"
echo "Risk Profiles: $PROFILES_COUNT_2"
echo ""

echo "Step 7: Final Summary"
echo "=========================================="
echo "OLD POD (UID: $POD_UID_1)"
echo "  - Pods: $PODS_COUNT_1 (soft-deleted: $POD_DELETED)"
echo "  - Capabilities: $CAPS_COUNT_1_AFTER (still exists)"
echo "  - Runtime Events: $EVENTS_COUNT_1_AFTER (still exists)"
echo "  - Risk Profiles: $PROFILES_COUNT_1_AFTER (still exists)"
echo ""
echo "NEW POD (UID: $POD_UID_2)"
echo "  - Pods: $PODS_COUNT_2"
echo "  - Capabilities: $CAPS_COUNT_2"
echo "  - Runtime Events: $EVENTS_COUNT_2"
echo "  - Risk Profiles: $PROFILES_COUNT_2"
echo ""

# Cleanup
echo "Cleaning up test pod..."
kubectl delete pod -n "$POD_NAMESPACE" "$POD_NAME" --wait=false 2>/dev/null || true

echo ""
echo "=========================================="
echo "Test Complete"
echo "=========================================="

if [ "${PODS_COUNT_1:-0}" -eq 0 ] || [ "${PODS_COUNT_2:-0}" -eq 0 ]; then
    echo "❌ Storage recreate assertions failed: pod rows missing in DB"
    exit 1
fi
