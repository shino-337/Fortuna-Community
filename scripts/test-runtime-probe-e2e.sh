#!/bin/bash

# E2E Runtime Probe Test
# Tests actual syscalls in pod to trigger runtime escape detection

set -e

POD_NAME="${POD_NAME:-cve-2025-31133-pod}"
POD_NAMESPACE="${POD_NAMESPACE:-default}"
CORE_URL="${CORE_URL:-http://fortuna-core.fortuna.svc.cluster.local:8080}"

echo "=========================================="
echo "E2E Runtime Probe Test"
echo "=========================================="
echo "Pod: $POD_NAMESPACE/$POD_NAME"
echo "Core: $CORE_URL"
echo ""

# Get pod UID
POD_UID=$(kubectl get pod -n "$POD_NAMESPACE" "$POD_NAME" -o jsonpath='{.metadata.uid}' 2>/dev/null || echo "")
if [ -z "$POD_UID" ]; then
    echo "❌ Pod $POD_NAMESPACE/$POD_NAME not found"
    exit 1
fi

echo "✅ Pod UID: $POD_UID"
echo ""

# Test 1: ls /proc/1/root (should trigger PROC_ROOT_PIVOT)
echo "Test 1: Executing 'ls /proc/1/root' in pod..."
kubectl exec -n "$POD_NAMESPACE" "$POD_NAME" -- ls /proc/1/root >/dev/null 2>&1 && echo "✅ Command executed" || echo "⚠️  Command failed (may be expected)"

sleep 3

# Test 2: stat /proc/self/exe
echo ""
echo "Test 2: Executing 'stat /proc/self/exe' in pod..."
kubectl exec -n "$POD_NAMESPACE" "$POD_NAME" -- stat /proc/self/exe >/dev/null 2>&1 && echo "✅ Command executed" || echo "⚠️  Command failed"

sleep 3

# Test 3: readlink /proc/1/exe
echo ""
echo "Test 3: Executing 'readlink /proc/1/exe' in pod..."
kubectl exec -n "$POD_NAMESPACE" "$POD_NAME" -- readlink /proc/1/exe >/dev/null 2>&1 && echo "✅ Command executed" || echo "⚠️  Command failed"

sleep 5

# Check runtime events in DB
echo ""
echo "=========================================="
echo "Verifying Runtime Events in Database"
echo "=========================================="

# Port-forward to Core for API access
kubectl -n fortuna port-forward svc/fortuna-core 28080:8080 >/tmp/pf-core.log 2>&1 &
PF_PID=$!
trap "kill $PF_PID >/dev/null 2>&1 || true" EXIT
sleep 2

# Query runtime events via API
echo "Fetching runtime events for pod $POD_UID..."
EVENTS_RESP=$(curl -s "http://127.0.0.1:28080/api/v1/runtime-risk/pods/$POD_UID/events?limit=10" || echo "{}")
EVENT_COUNT=$(echo "$EVENTS_RESP" | grep -o '"total":[0-9]*' | grep -o '[0-9]*' || echo "0")

echo "Found $EVENT_COUNT runtime events"
if [ "$EVENT_COUNT" -gt 0 ]; then
    echo "$EVENTS_RESP" | python3 -m json.tool 2>/dev/null || echo "$EVENTS_RESP"
fi

# Query risk profile
echo ""
echo "Fetching risk profile for pod $POD_UID..."
PROFILE_RESP=$(curl -s "http://127.0.0.1:28080/api/v1/runtime-risk/pods/$POD_UID" || echo "{}")
echo "$PROFILE_RESP" | python3 -m json.tool 2>/dev/null || echo "$PROFILE_RESP"

# Check capabilities
echo ""
echo "Fetching capabilities for pod $POD_UID..."
CAPS_RESP=$(curl -s "http://127.0.0.1:28080/api/v1/pods/$POD_UID/capabilities" || echo "{}")
echo "$CAPS_RESP" | python3 -m json.tool 2>/dev/null || echo "$CAPS_RESP"

echo ""
echo "=========================================="
echo "E2E Test Complete"
echo "=========================================="
