#!/bin/bash
# SBOM diagnostic for a pod UID – determines why GET /inventory/pods/:uid/sbom returns 404.
# Usage: ./scripts/verify/verify-sbom-pod.sh [pod_uid]
# Example: ./scripts/verify/verify-sbom-pod.sh e945678b-e332-4bd9-9a7a-8683e560b3b3

set -e

POD_UID="${1:-e945678b-e332-4bd9-9a7a-8683e560b3b3}"
NAMESPACE="${FORTUNA_NAMESPACE:-fortuna}"

echo "=========================================="
echo "SBOM diagnostic – pod_uid: $POD_UID"
echo "=========================================="
echo ""

run_psql() {
  local sql="$1"
  if [ -n "$POSTGRES_POD" ] && [ -n "$NAMESPACE" ]; then
    kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U postgres -d fortuna -t -A -c "$sql" 2>/dev/null || echo "QUERY_FAILED"
  elif [ -n "$DATABASE_URL" ]; then
    psql "$DATABASE_URL" -t -A -c "$sql" 2>/dev/null || echo "QUERY_FAILED"
  else
    echo "QUERY_FAILED"
  fi
}

run_psql_multiline() {
  local sql="$1"
  if [ -n "$POSTGRES_POD" ] && [ -n "$NAMESPACE" ]; then
    kubectl exec -n "$NAMESPACE" "$POSTGRES_POD" -- psql -U postgres -d fortuna -c "$sql" 2>/dev/null || echo "❌ Query failed"
  elif [ -n "$DATABASE_URL" ]; then
    psql "$DATABASE_URL" -c "$sql" 2>/dev/null || echo "❌ Query failed"
  else
    echo "❌ Set POSTGRES_POD + NAMESPACE (e.g. fortuna) or DATABASE_URL"
  fi
}

# Resolve Postgres pod if running in K8s
POSTGRES_POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

echo "=== 1. SBOM rows for this pod_uid ==="
run_psql_multiline "
  SELECT id, pod_uid, pod_name, namespace, container_name, image_digest, package_count, created_at, deleted_at
  FROM sboms
  WHERE pod_uid = '$POD_UID';
"
echo ""

echo "=== 2. Pod row(s) in Core (pods table) for this UID ==="
run_psql_multiline "
  SELECT id, name, uid, namespace, cluster_id, node_name, created_at, deleted_at
  FROM pods
  WHERE uid = '$POD_UID';
"
echo ""

echo "=== 3. Recent SBOM activity (any pod, last 10) ==="
run_psql_multiline "
  SELECT id, pod_uid, pod_name, namespace, package_count, created_at
  FROM sboms
  WHERE deleted_at IS NULL
  ORDER BY created_at DESC
  LIMIT 10;
"
echo ""

echo "=== 4. Agent pods (DaemonSet) – which nodes have Agent ==="
AGENT_NODES=$(kubectl get pods -n "$NAMESPACE" -l 'app.kubernetes.io/name=fortuna,app.kubernetes.io/component=agent' -o jsonpath='{.items[*].spec.nodeName}' 2>/dev/null || echo "")
if [ -z "$AGENT_NODES" ]; then
  AGENT_NODES=$(kubectl get pods -n "$NAMESPACE" -l app=fortuna-agent -o jsonpath='{.items[*].spec.nodeName}' 2>/dev/null || echo "")
fi
if [ -n "$AGENT_NODES" ]; then
  echo "Agent running on nodes: $AGENT_NODES"
else
  echo "No Agent pods found (labels: app.kubernetes.io/name=fortuna,app.kubernetes.io/component=agent or app=fortuna-agent)"
fi
echo ""

echo "=== 5. Pod in cluster (if we have a name) – which node it runs on ==="
POD_NAME=$(run_psql "SELECT name FROM pods WHERE uid = '$POD_UID' AND deleted_at IS NULL LIMIT 1;")
POD_NS=$(run_psql "SELECT namespace FROM pods WHERE uid = '$POD_UID' AND deleted_at IS NULL LIMIT 1;")
if [ -n "$POD_NAME" ] && [ "$POD_NAME" != "QUERY_FAILED" ] && [ -n "$POD_NS" ]; then
  NODE=$(kubectl get pod -n "$POD_NS" "$POD_NAME" -o jsonpath='{.spec.nodeName}' 2>/dev/null || echo "")
  if [ -n "$NODE" ]; then
    echo "Pod $POD_NS/$POD_NAME runs on node: $NODE"
    if [ -n "$AGENT_NODES" ]; then
      if echo "$AGENT_NODES" | grep -q "$NODE"; then
        echo "✅ Node $NODE has an Agent – SBOM can be produced if Agent sent it."
      else
        echo "⚠️  Node $NODE has NO Agent – this is a common cause: Agent only runs on certain nodes."
      fi
    fi
  else
    echo "Pod $POD_NS/$POD_NAME not found in cluster (deleted or different context)."
  fi
else
  echo "Could not resolve pod name/namespace from DB for UID $POD_UID (or DB not reachable)."
fi
echo ""

echo "=========================================="
echo "Summary: If sboms table has no row for pod_uid, Agent has not sent SBOM for this pod."
echo "See docs/03-components/sbom/SBOM-Not-Loading-Checklist.md for full checklist."
echo "=========================================="
