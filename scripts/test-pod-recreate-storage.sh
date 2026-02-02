#!/bin/bash

# Test Pod Recreate Storage Behavior
# Tests how database stores data when:
# 1. Same pod deployed twice (same UID)
# 2. Pod deleted and recreated (new UID)

set -e

POD_NAME="test-pod-recreate"
POD_NAMESPACE="default"
CORE_URL="${CORE_URL:-http://fortuna-core.fortuna.svc.cluster.local:8080}"

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
    kubectl -n fortuna exec postgres-7858fc8764-kgccd -- psql -U postgres -d fortuna -t -A -F '|' -c "$query" 2>/dev/null || echo ""
}

# Function to count records
count_records() {
    local table="$1"
    local pod_uid="$2"
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
sleep 30

# Inject runtime event
echo "Injecting runtime event for pod $POD_UID_1..."
(kubectl -n fortuna port-forward svc/fortuna-core 28080:8080 >/tmp/pf-core.log 2>&1 & PF_PID=$!; sleep 2; curl -s -X POST "http://127.0.0.1:28080/api/v1/runtime-events" -H "Content-Type: application/json" -d "[{\"pod_uid\":\"$POD_UID_1\",\"namespace\":\"$POD_NAMESPACE\",\"syscall\":\"openat\",\"target_path\":\"/proc/1/root\",\"capability\":\"SYS_ADMIN\",\"timestamp\":$(date +%s)}]" >/dev/null; kill $PF_PID >/dev/null 2>&1)
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
sleep 10

# Check if pod is soft-deleted in DB
POD_DELETED=$(query_db "SELECT deleted_at IS NOT NULL FROM pods WHERE uid = '$POD_UID_1' LIMIT 1;" | tr -d ' ')
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
sleep 30

# Inject runtime event for new pod
echo "Injecting runtime event for pod $POD_UID_2..."
(kubectl -n fortuna port-forward svc/fortuna-core 28080:8080 >/tmp/pf-core.log 2>&1 & PF_PID=$!; sleep 2; curl -s -X POST "http://127.0.0.1:28080/api/v1/runtime-events" -H "Content-Type: application/json" -d "[{\"pod_uid\":\"$POD_UID_2\",\"namespace\":\"$POD_NAMESPACE\",\"syscall\":\"openat\",\"target_path\":\"/proc/1/root\",\"capability\":\"SYS_ADMIN\",\"timestamp\":$(date +%s)}]" >/dev/null; kill $PF_PID >/dev/null 2>&1)
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
