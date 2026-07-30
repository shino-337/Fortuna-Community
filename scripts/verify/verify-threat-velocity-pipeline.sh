#!/usr/bin/env bash
# ============================================================================
# Verify Threat Velocity pipeline step-by-step: Pod → SBOM → CVE match → Insights → API
# ============================================================================
# Run: NAMESPACE=fortuna scripts/verify/verify-threat-velocity-pipeline.sh
# Or for E2E vuln pod: VULN_NS=fortuna-e2e VULN_POD=fortuna-e2e-vuln-debian10 scripts/verify/verify-threat-velocity-pipeline.sh
# ============================================================================

set -euo pipefail

NAMESPACE="${NAMESPACE:-fortuna}"
DB_NAME="${DB_NAME:-fortuna}"
VULN_NS="${VULN_NS:-fortuna-e2e}"
VULN_POD="${VULN_POD:-fortuna-e2e-vuln-debian10}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'
ok()  { echo -e "${GREEN}[OK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
err()  { echo -e "${RED}[ERR]${NC} $1"; }
info() { echo -e "${BLUE}[INFO]${NC} $1"; }

PG_POD=$(kubectl get pods -n "$NAMESPACE" -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -z "$PG_POD" ]; then
  err "Postgres pod not found in namespace $NAMESPACE"
  exit 1
fi

run_sql() {
  kubectl exec -n "$NAMESPACE" "$PG_POD" -- psql -U postgres -d "$DB_NAME" -t -A "$@" 2>/dev/null || echo ""
}

echo "=========================================="
echo "Threat Velocity pipeline diagnostic"
echo "=========================================="
echo ""

# --- Step 0: Vuln pod UID from Kubernetes ---
info "Step 0: Get vuln pod UID from Kubernetes..."
VULN_UID=$(kubectl get pod -n "$VULN_NS" "$VULN_POD" -o jsonpath='{.metadata.uid}' 2>/dev/null || echo "")
if [ -z "$VULN_UID" ]; then
  err "Pod $VULN_NS/$VULN_POD not found in cluster (kubectl get pod)"
  exit 1
fi
ok "Vuln pod UID (from K8s): $VULN_UID"
echo ""

# --- Step 1: Pod in Core DB ---
info "Step 1: Pod in Core DB (pods table)?"
POD_ROW=$(run_sql -c "SELECT uid, cluster_id, name, namespace, deleted_at IS NOT NULL as deleted FROM pods WHERE uid = '$VULN_UID' LIMIT 1;")
if [ -z "$POD_ROW" ]; then
  err "Pod UID $VULN_UID not found in pods table. Agent may not have synced this pod (namespace $VULN_NS)."
else
  ok "Pod found in DB: $POD_ROW"
fi
# Also check if uid format might differ (e.g. lowercase)
POD_UID_DB=$(run_sql -c "SELECT uid FROM pods WHERE uid = '$VULN_UID' LIMIT 1;")
if [ -z "$POD_UID_DB" ]; then
  warn "Checking pods.uid format - sample uids:"
  run_sql -c "SELECT uid, name, namespace FROM pods WHERE deleted_at IS NULL LIMIT 3;"
fi
echo ""

# --- Step 2: SBOM for this pod ---
info "Step 2: SBOM for this pod (sboms.pod_uid)?"
SBOM_IDS=$(run_sql -c "SELECT id FROM sboms WHERE pod_uid = '$VULN_UID' AND deleted_at IS NULL;")
if [ -z "$SBOM_IDS" ]; then
  err "No SBOM found with pod_uid = $VULN_UID. Agent must sync pod and extract SBOM (fortuna.sbom.created not yet sent or SBOM worker did not run)."
  warn "All sboms with pod_uid like this pod:"
  run_sql -c "SELECT id, pod_uid, image_name, namespace FROM sboms WHERE deleted_at IS NULL ORDER BY id DESC LIMIT 5;"
else
  ok "SBOM id(s): $SBOM_IDS"
  for sid in $SBOM_IDS; do
    run_sql -c "SELECT id, pod_uid, image_name, namespace, package_count FROM sboms WHERE id = $sid;"
    COMP_COUNT=$(run_sql -c "SELECT COUNT(*) FROM sbom_components WHERE sbom_id = $sid AND deleted_at IS NULL;")
    if [ "${COMP_COUNT:-0}" -eq 0 ]; then
      err "SBOM id=$sid has 0 components (sbom_components). Agent likely sent Packages=[] — CVE matcher has nothing to match. Check agent logs for 'Extracted N packages' for this image."
    else
      ok "SBOM id=$sid components: $COMP_COUNT"
    fi
  done
fi
echo ""

# --- Step 3: CVE matches for SBOM ---
info "Step 3: CVE matches for this SBOM (cve_matches)?"
FIRST_SBOM_ID=$(echo "$SBOM_IDS" | awk '{print $1}')
if [ -n "$FIRST_SBOM_ID" ]; then
  CVE_MATCH_COUNT=$(run_sql -c "SELECT COUNT(*) FROM cve_matches WHERE sbom_id = $FIRST_SBOM_ID;")
  if [ "${CVE_MATCH_COUNT:-0}" -eq 0 ]; then
    warn "No cve_matches for sbom_id = $FIRST_SBOM_ID. CVE matcher may not have run, or no package_vulnerabilities match (debian:10 packages)."
  else
    ok "cve_matches for sbom_id $FIRST_SBOM_ID: $CVE_MATCH_COUNT"
    run_sql -c "SELECT cve_id, package_name, severity FROM cve_matches WHERE sbom_id = $FIRST_SBOM_ID LIMIT 5;"
  fi
else
  warn "Skip (no SBOM id)"
fi
echo ""

# --- Step 4: Vulnerability insights for this pod ---
info "Step 4: Vulnerability insights (resource_uid = pod UID)?"
INSIGHT_COUNT=$(run_sql -c "SELECT COUNT(*) FROM insights WHERE insight_type = 'vulnerability' AND resource_uid = '$VULN_UID' AND deleted_at IS NULL;")
if [ "${INSIGHT_COUNT:-0}" -eq 0 ]; then
  err "No vulnerability insights with resource_uid = $VULN_UID. CVE matcher creates insights with ResourceUID = ev.PodUID; if cve_matches exist but insights don't, check onlySeverities (CRITICAL/HIGH/MEDIUM) or InsightManager/BatchCreateOrUpdateInsights."
  warn "Sample vulnerability insights (any pod):"
  run_sql -c "SELECT id, resource_uid, severity, detected_at, deleted_at IS NOT NULL FROM insights WHERE insight_type = 'vulnerability' AND deleted_at IS NULL ORDER BY id DESC LIMIT 5;"
else
  ok "Vulnerability insights for this pod: $INSIGHT_COUNT"
  run_sql -c "SELECT id, resource_uid, severity, detected_at FROM insights WHERE insight_type = 'vulnerability' AND resource_uid = '$VULN_UID' AND deleted_at IS NULL LIMIT 5;"
fi
echo ""

# --- Step 5: Threat Velocity query (exact logic) ---
info "Step 5: Threat Velocity query (insight_type=vulnerability, pod filter, last 7 days)..."
START_SQL="(NOW() - INTERVAL '6 days')::date"
# Count insights that would be included
VEL_COUNT=$(run_sql -c "
SELECT COUNT(*) FROM insights i
WHERE i.insight_type = 'vulnerability' AND i.detected_at >= $START_SQL AND i.deleted_at IS NULL
AND (i.resource_type != 'Pod' OR i.resource_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL));
")
ok "Insights matching Threat Velocity filter (all clusters, last 7 days): ${VEL_COUNT:-0}"
if [ "${VEL_COUNT:-0}" -gt 0 ]; then
  info "By day and severity:"
  run_sql -c "
  SELECT date_trunc('day', detected_at)::date AS d, LOWER(severity) AS sev, COUNT(*)
  FROM insights i
  WHERE i.insight_type = 'vulnerability' AND i.detected_at >= $START_SQL AND i.deleted_at IS NULL
  AND (i.resource_type != 'Pod' OR i.resource_uid IN (SELECT uid FROM pods WHERE deleted_at IS NULL))
  GROUP BY date_trunc('day', detected_at), LOWER(severity)
  ORDER BY d, sev;
  "
else
  warn "No rows → API returns 7 points with all zeros."
fi
echo ""

# --- Step 6: Cluster ID and scoped query ---
info "Step 6: If pod is in DB, cluster_id and scoped Threat Velocity..."
if [ -n "$POD_UID_DB" ]; then
  CLUSTER_ID=$(run_sql -c "SELECT cluster_id FROM pods WHERE uid = '$VULN_UID' AND deleted_at IS NULL LIMIT 1;")
  if [ -n "$CLUSTER_ID" ]; then
    ok "Pod cluster_id: $CLUSTER_ID"
    VEL_SCOPED=$(run_sql -c "
    SELECT COUNT(*) FROM insights i
    WHERE i.insight_type = 'vulnerability' AND i.detected_at >= $START_SQL AND i.deleted_at IS NULL
    AND i.resource_uid IN (SELECT uid FROM pods WHERE cluster_id = '$CLUSTER_ID' AND deleted_at IS NULL);
    ")
    ok "Insights in Threat Velocity when clusterId=$CLUSTER_ID: ${VEL_SCOPED:-0}"
  fi
fi
echo ""

echo "=========================================="
echo "Summary"
echo "=========================================="
echo "1. Pod in K8s: $VULN_NS/$VULN_POD uid=$VULN_UID"
echo "2. Pod in Core DB: $( [ -n "$POD_UID_DB" ] && echo 'yes' || echo 'no' )"
echo "3. SBOM for pod: $( [ -n "$SBOM_IDS" ] && echo "yes (id $SBOM_IDS)" || echo 'no' )"
echo "4. CVE matches: ${CVE_MATCH_COUNT:-0}"
echo "5. Vulnerability insights for this pod: ${INSIGHT_COUNT:-0}"
echo "6. Threat Velocity total (last 7d, all clusters): ${VEL_COUNT:-0}"
echo ""
echo "If velocity still zero: ensure pod is in pods table (agent sync), SBOM exists, cve_matches exist, and onlySeverities in CVE matcher (CRITICAL/HIGH/MEDIUM) include your CVE severities."
echo ""
