#!/bin/bash
# Applies initial DB schema (migration 001) so Core and agent sync work.
# Run this if Core logs "relation clusters does not exist" or agent gets sync status=500.
# Usage: ./scripts/utils/ensure-initial-schema.sh

set -euo pipefail

NAMESPACE="${NAMESPACE:-fortuna}"
DB_NAME="${DB_NAME:-fortuna}"
PROJECT_ROOT="${PROJECT_ROOT:-$(cd "$(dirname "$0")/../.." && pwd)}"
SQL_FILE="${PROJECT_ROOT}/core/migrations/001_initial_schema.sql"

if [ ! -f "$SQL_FILE" ]; then
    echo "[ERROR] SQL file not found: $SQL_FILE" >&2
    exit 1
fi

pod=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -z "$pod" ]; then
    echo "[ERROR] No postgres pod found in namespace $NAMESPACE" >&2
    exit 1
fi

echo "[INFO] Applying initial schema (001) to DB (pod=$pod)..."
kubectl cp -n "$NAMESPACE" "$SQL_FILE" "$pod:/tmp/001_initial_schema.sql"
kubectl exec -n "$NAMESPACE" "$pod" -- psql -U postgres -d "$DB_NAME" -f /tmp/001_initial_schema.sql
kubectl exec -n "$NAMESPACE" "$pod" -- rm -f /tmp/001_initial_schema.sql
echo "[SUCCESS] Initial schema applied. Restart Core so remaining migrations run: kubectl rollout restart deployment/fortuna-core -n $NAMESPACE"
