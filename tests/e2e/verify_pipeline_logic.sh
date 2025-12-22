#!/usr/bin/env bash
# KSAM Pipeline Logic Verification Script
# Verifies the complete event flow: Pod → Normalizer → SBOM → CVE → Insights
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NAMESPACE="${NAMESPACE:-ksam}"
E2E_NS="${E2E_NS:-ksam-e2e}"
TEST_POD_NAME="${TEST_POD_NAME:-ksam-e2e-vuln-debian10}"
TEST_CVE_ID="${TEST_CVE_ID:-CVE-2014-0011}"

die() { echo "[VERIFY] ERROR: $*" >&2; exit 1; }
step() { echo ""; echo "==> $*"; }
info() { echo "[VERIFY] $*"; }

command -v kubectl >/dev/null 2>&1 || die "kubectl not found"
command -v python3 >/dev/null 2>&1 || die "python3 not found"

POSTGRES_POD="$(kubectl -n "$NAMESPACE" get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}')"
[[ -n "${POSTGRES_POD}" ]] || die "Postgres pod not found"

step "1) Verify Event Flow: Agent → Normalizer → SBOM → CVE → Insights"

info "Checking if E2E pod exists..."
POD_UID="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT uid FROM pods WHERE namespace='${E2E_NS}' AND name='${TEST_POD_NAME}' AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 1;" 2>/dev/null | tr -d '[:space:]' || echo "")"

if [[ -z "${POD_UID}" ]]; then
  info "E2E pod not found in DB. Creating test pod..."
  kubectl apply -f - <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${E2E_NS}
---
apiVersion: v1
kind: Pod
metadata:
  name: ${TEST_POD_NAME}
  namespace: ${E2E_NS}
  labels:
    app: ksam-e2e-vuln
    test: e2e-cve-insight
spec:
  restartPolicy: Never
  containers:
    - name: app
      image: ksam/e2e-vuln:latest
      imagePullPolicy: Never
      command: ["sh", "-c", "echo 'KSAM E2E pod running' && sleep 3600"]
YAML
  kubectl -n "$E2E_NS" wait --for=condition=Ready pod/"$TEST_POD_NAME" --timeout=120s
  sleep 10
  POD_UID="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT uid FROM pods WHERE namespace='${E2E_NS}' AND name='${TEST_POD_NAME}' AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 1;" 2>/dev/null | tr -d '[:space:]' || echo "")"
fi

[[ -n "${POD_UID}" ]] || die "Pod not found in DB after creation"

info "Pod UID: ${POD_UID}"

step "2) Verify NormalizerWorker processed pod"
NORMALIZED_COUNT="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM pods WHERE uid='${POD_UID}' AND deleted_at IS NULL;" | tr -d '[:space:]')"
if [[ "${NORMALIZED_COUNT}" == "0" ]]; then
  die "❌ NormalizerWorker: Pod not in DB"
fi
info "✅ NormalizerWorker: Pod stored in DB"

step "3) Verify SBOMWorker processed pod"
SBOM_COUNT="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(DISTINCT sbom_id) FROM pod_image_scans WHERE pod_uid='${POD_UID}' AND sbom_id IS NOT NULL AND deleted_at IS NULL;" | tr -d '[:space:]')"
if [[ "${SBOM_COUNT}" == "0" ]]; then
  info "⚠️  SBOMWorker: No SBOM linked to pod yet. Checking logs..."
  kubectl -n "$NAMESPACE" logs -l app=ksam-core --tail=500 2>&1 | grep -E "\[SBOMWorker\].*📥.*${E2E_NS}|\[SBOMWorker\].*${TEST_POD_NAME}" | tail -5 || info "No SBOMWorker logs for E2E pod"
  die "❌ SBOMWorker: No SBOM linked to pod"
fi
info "✅ SBOMWorker: ${SBOM_COUNT} SBOM(s) linked to pod"

SBOM_ID="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT sbom_id FROM pod_image_scans WHERE pod_uid='${POD_UID}' AND sbom_id IS NOT NULL AND deleted_at IS NULL LIMIT 1;" | tr -d '[:space:]')"
info "SBOM ID: ${SBOM_ID}"

step "4) Verify SBOM has components"
COMPONENT_COUNT="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM sbom_components WHERE sbom_id=${SBOM_ID} AND deleted_at IS NULL;" | tr -d '[:space:]')"
if [[ "${COMPONENT_COUNT}" == "0" ]]; then
  die "❌ SBOM has no components"
fi
info "✅ SBOM has ${COMPONENT_COUNT} component(s)"

step "5) Verify CVEMatcherWorker processed SBOM"
MATCH_COUNT="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM cve_matches WHERE sbom_id=${SBOM_ID} AND cve_id='${TEST_CVE_ID}' AND deleted_at IS NULL;" | tr -d '[:space:]')"
if [[ "${MATCH_COUNT}" == "0" ]]; then
  info "⚠️  CVEMatcherWorker: No CVE matches found. Checking available CVEs..."
  CVE_COUNT="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM cves WHERE cve_id='${TEST_CVE_ID}';" | tr -d '[:space:]')"
  info "CVE ${TEST_CVE_ID} in DB: ${CVE_COUNT}"
  if [[ "${CVE_COUNT}" == "0" ]]; then
    die "❌ CVE ${TEST_CVE_ID} not in database. Run cve-loader first."
  fi
  die "❌ CVEMatcherWorker: No CVE matches (CVE exists but not matched)"
fi
info "✅ CVEMatcherWorker: ${MATCH_COUNT} CVE match(es) for ${TEST_CVE_ID}"

step "6) Verify Insights created"
INSIGHT_COUNT="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights WHERE type='vulnerability' AND cve_id='${TEST_CVE_ID}' AND sbom_id=${SBOM_ID} AND deleted_at IS NULL;" | tr -d '[:space:]')"
if [[ "${INSIGHT_COUNT}" == "0" ]]; then
  die "❌ No vulnerability insights created"
fi
info "✅ ${INSIGHT_COUNT} vulnerability insight(s) created"

step "7) Verify complete linkage"
LINKAGE="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "
SELECT 
  pis.pod_uid,
  pis.pod_name,
  s.id as sbom_id,
  s.image_digest,
  COUNT(DISTINCT cm.id) as cve_matches,
  COUNT(DISTINCT i.id) as insights
FROM pod_image_scans pis
JOIN sboms s ON pis.sbom_id = s.id
LEFT JOIN cve_matches cm ON s.id = cm.sbom_id AND cm.cve_id='${TEST_CVE_ID}'
LEFT JOIN insights i ON s.id = i.sbom_id AND i.cve_id='${TEST_CVE_ID}'
WHERE pis.pod_uid='${POD_UID}' AND pis.deleted_at IS NULL
GROUP BY pis.pod_uid, pis.pod_name, s.id, s.image_digest;
" 2>/dev/null | head -1)"

if [[ -z "${LINKAGE}" ]]; then
  die "❌ Linkage verification failed"
fi
info "✅ Complete linkage verified:"
echo "   ${LINKAGE}"

step "8) Verify Insights API"
kubectl -n "$NAMESPACE" port-forward svc/ksam-core 18080:8080 >/tmp/ksam-verify-pf.log 2>&1 &
PF_PID=$!
trap 'kill $PF_PID 2>/dev/null || true' EXIT
sleep 3

TOKEN="$(curl -sS -X POST "http://localhost:18080/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null || echo "")"
if [[ -z "${TOKEN}" ]]; then
  die "❌ Failed to get auth token"
fi

INSIGHTS_JSON="$(curl -sS "http://localhost:18080/api/v1/insights?type=vulnerability&status=all&pageSize=50" -H "Authorization: Bearer ${TOKEN}")"
FOUND="$(echo "$INSIGHTS_JSON" | python3 -c "import json,sys; d=json.load(sys.stdin); print('True' if any(i.get('cveId')=='${TEST_CVE_ID}' for i in d.get('insights',[])) else 'False')" 2>/dev/null || echo "False")"

if [[ "${FOUND}" != "True" ]]; then
  die "❌ Insights API: CVE ${TEST_CVE_ID} not found"
fi
info "✅ Insights API: CVE ${TEST_CVE_ID} found"

step "✅ Pipeline Logic Verification: PASS"
info "Complete flow verified: Pod → Normalizer → SBOM → CVE → Insights → API"

