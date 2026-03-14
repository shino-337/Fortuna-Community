#!/usr/bin/env bash
# Backfill pod_ip/start_time for one pod by sending a minimal sync payload to Core.
# Use when agent sync has run but DB still has empty pod_ip (e.g. cluster_id mismatch or payload key issue).
#
# Usage:
#   CORE_URL=http://localhost:8080 ./scripts/verify/backfill-pod-ip-via-sync.sh [POD_ID]
#   With port-forward: kubectl port-forward -n fortuna svc/fortuna-core 8080:8080
#   Then: ./scripts/verify/backfill-pod-ip-via-sync.sh 147
#
# If POD_ID is omitted, uses first pod from DB with empty pod_ip.

set -euo pipefail

CORE_URL="${CORE_URL:-http://localhost:8080}"
BASE="${CORE_URL%/}/api/v1/agent/sync"
POD_ID="${1:-}"

NAMESPACE="${NAMESPACE:-fortuna}"
PG_POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -z "$PG_POD" ]; then
  echo "Postgres pod not found in namespace $NAMESPACE"
  exit 1
fi

if [ -z "$POD_ID" ]; then
  echo "Finding a pod with empty pod_ip..."
  ROW=$(kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -A -F'|' -c \
    "SELECT id, uid, name, namespace, cluster_id FROM pods WHERE deleted_at IS NULL AND (pod_ip IS NULL OR pod_ip = '') LIMIT 1;" 2>/dev/null || true)
else
  ROW=$(kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -A -F'|' -c \
    "SELECT id, uid, name, namespace, cluster_id FROM pods WHERE id = $POD_ID AND deleted_at IS NULL;" 2>/dev/null || true)
fi

if [ -z "$ROW" ]; then
  echo "No pod found (or all pods already have pod_ip)."
  exit 0
fi

IFS='|' read -r ID UID NAME NS CLUSTER_ID <<< "$ROW"
CLUSTER_ID=$(echo "$CLUSTER_ID" | tr -d ' ')
echo "Pod: id=$ID uid=$UID name=$NAME namespace=$NS cluster_id=$CLUSTER_ID"

# Get current pod IP and startTime from cluster (any node with kubectl)
POD_IP=""
START_TIME=""
if kubectl get pod -n "$NS" -o json 2>/dev/null | jq -e --arg n "$NAME" '.items[] | select(.metadata.name == $n)' >/dev/null 2>&1; then
  POD_IP=$(kubectl get pod -n "$NS" "$NAME" -o jsonpath='{.status.podIP}' 2>/dev/null || true)
  START_TIME=$(kubectl get pod -n "$NS" "$NAME" -o jsonpath='{.status.startTime}' 2>/dev/null || true)
fi
if [ -z "$POD_IP" ]; then
  echo "Could not get pod IP from cluster for $NS/$NAME (pod may be on another node). Using placeholder."
  POD_IP="0.0.0.0"
fi
if [ -z "$START_TIME" ]; then
  START_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
fi

echo "Sending sync with podIP=$POD_IP startTime=$START_TIME ..."
HTTP=$(curl -s -w "%{http_code}" -o /tmp/backfill-sync-out.json -X POST "$BASE" \
  -H "Content-Type: application/json" \
  -d "{
    \"clusterId\": \"$CLUSTER_ID\",
    \"clusterName\": \"backfill\",
    \"data\": {
      \"isFullSync\": true,
      \"isDeltaSync\": false,
      \"pods\": [{
        \"uid\": \"$UID\",
        \"name\": \"$NAME\",
        \"namespace\": \"$NS\",
        \"phase\": \"Running\",
        \"serviceAccountName\": \"default\",
        \"nodeName\": \"node\",
        \"podIP\": \"$POD_IP\",
        \"startTime\": \"$START_TIME\",
        \"restartCount\": 0,
        \"containers\": [],
        \"volumes\": []
      }]
    }
  }")

if [ "$HTTP" != "200" ]; then
  echo "Sync failed: HTTP $HTTP"
  cat /tmp/backfill-sync-out.json | jq . 2>/dev/null || cat /tmp/backfill-sync-out.json
  exit 1
fi

echo "Sync OK. Checking DB..."
kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d fortuna -t -c \
  "SELECT id, uid, name, pod_ip, start_time FROM pods WHERE id = $ID;"
echo "Done. GET $CORE_URL/api/v1/inventory/pods/<uid> (with auth) to verify API response."
