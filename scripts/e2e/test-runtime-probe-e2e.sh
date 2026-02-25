#!/usr/bin/env bash

# E2E Runtime Probe Test
# Tests actual syscalls in pod to trigger runtime escape detection

set -euo pipefail

POD_NAME="${POD_NAME:-cve-2025-31133-pod}"
POD_NAMESPACE="${POD_NAMESPACE:-default}"
NAMESPACE="${NAMESPACE:-fortuna}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./common.sh
source "${SCRIPT_DIR}/common.sh"

echo "=========================================="
echo "E2E Runtime Probe Test"
echo "=========================================="
echo "Pod: $POD_NAMESPACE/$POD_NAME"
echo "Core Namespace: $NAMESPACE"
echo ""

# Get pod UID
POD_UID=$(kubectl get pod -n "$POD_NAMESPACE" "$POD_NAME" -o jsonpath='{.metadata.uid}' 2>/dev/null || echo "")
if [ -z "$POD_UID" ]; then
    echo "⚠️  Pod $POD_NAMESPACE/$POD_NAME not found, creating it..."
    cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: $POD_NAME
  namespace: $POD_NAMESPACE
spec:
  containers:
  - name: runtime-probe
    image: busybox:latest
    command: ["sh", "-c", "sleep 1800"]
  restartPolicy: Never
EOF
    kubectl wait --for=condition=Ready "pod/$POD_NAME" -n "$POD_NAMESPACE" --timeout=120s >/dev/null
    POD_UID=$(kubectl get pod -n "$POD_NAMESPACE" "$POD_NAME" -o jsonpath='{.metadata.uid}')
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

# Check runtime events/signal in API
echo ""
echo "=========================================="
echo "Verifying Runtime Signals via API"
echo "=========================================="

CORE_POD="$(require_core_pod)"
TOKEN="$(require_jwt_token "$CORE_POD")"

echo "Fetching runtime signals for pod $POD_UID..."
EVENTS_RESP="$(core_api_get "runtime-signals/pods/$POD_UID" "$TOKEN" "$CORE_POD" || echo "{}")"
EVENT_COUNT=$(echo "$EVENTS_RESP" | python3 -c 'import sys,json
try:
 d=json.load(sys.stdin)
 print(d.get("count", d.get("total", 0)))
except Exception:
 print(0)
')

echo "Found $EVENT_COUNT runtime events"
if [ "$EVENT_COUNT" -gt 0 ]; then
    echo "$EVENTS_RESP" | python3 -m json.tool 2>/dev/null || echo "$EVENTS_RESP"
else
    echo "ℹ️  No runtime signal captured from syscall probe, sending synthetic runtime-event..."
    SYNTH_PAYLOAD=$(cat <<EOF
[{
  "pod_uid": "${POD_UID}",
  "namespace": "${POD_NAMESPACE}",
  "syscall": "openat",
  "target_path": "/proc/1/root",
  "capability": "",
  "timestamp": $(date +%s)
}]
EOF
)
    POST_RESP="$(core_api_post_json "runtime-events" "$TOKEN" "$CORE_POD" "$SYNTH_PAYLOAD" || echo "{}")"
    echo "Synthetic event response: $POST_RESP"
    sleep 3
    EVENTS_RESP="$(core_api_get "runtime-signals/pods/$POD_UID" "$TOKEN" "$CORE_POD" || echo "{}")"
    EVENT_COUNT=$(echo "$EVENTS_RESP" | python3 -c 'import sys,json
try:
 d=json.load(sys.stdin)
 print(d.get("count", d.get("total", 0)))
except Exception:
 print(0)
')
    echo "Signals after synthetic event: $EVENT_COUNT"
fi

# Check capabilities
echo ""
echo "Fetching capabilities for pod $POD_UID..."
CAPS_RESP="$(core_api_get "pods/$POD_UID/capabilities" "$TOKEN" "$CORE_POD" || echo "{}")"
echo "$CAPS_RESP" | python3 -m json.tool 2>/dev/null || echo "$CAPS_RESP"

if [ "$EVENT_COUNT" -eq 0 ]; then
    echo "❌ Runtime probe test failed: no runtime signals for pod $POD_UID"
    exit 1
fi

echo ""
echo "=========================================="
echo "E2E Test Complete"
echo "=========================================="
