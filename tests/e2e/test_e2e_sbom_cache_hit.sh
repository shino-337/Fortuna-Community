#!/usr/bin/env bash
# E2E Testcase: SBOM cache-hit behavior (digest-based)
#
# Flow:
# 1) Run the full E2E once (build core + load CVE + create pod) -> SBOM is created
# 2) Clear only derived artifacts (pod_image_scans, cve_matches, vulnerability insights)
#    but KEEP sboms + sbom_components
# 3) Re-create the same pod image -> SBOM must be reused (generated_at unchanged, use_count increments)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

NAMESPACE="${NAMESPACE:-ksam}"
E2E_NS="${E2E_NS:-ksam-e2e}"
TEST_POD_NAME="${TEST_POD_NAME:-ksam-e2e-cachehit}"

TAG="${TAG:-e2e-cachehit-$(date +%Y%m%d%H%M%S)}"
export TAG

export REAL_CVE_FILE="${REAL_CVE_FILE:-${REPO_DIR}/KSAM/cve-data/all/CVE-2014-0011.json}"
export TEST_CVE_ID="${TEST_CVE_ID:-CVE-2014-0011}"
export E2E_IMAGE_REPO="${E2E_IMAGE_REPO:-ksam/e2e-vuln}"
export E2E_IMAGE_CONTEXT="${E2E_IMAGE_CONTEXT:-${REPO_DIR}/KSAM/deploy/e2e/images/debian-dpkg-moin}"

die() { echo "[E2E-cachehit] ERROR: $*" >&2; exit 1; }
step() { echo ""; echo "==> $*"; }

step "0) First run: ensure SBOM exists (reuse existing full E2E script)"
export E2E_NS
export TEST_POD_NAME
chmod +x "${REPO_DIR}/KSAM/test_e2e_cve_insights_v2.sh"
"${REPO_DIR}/KSAM/test_e2e_cve_insights_v2.sh"

POSTGRES_POD="$(kubectl -n "$NAMESPACE" get pod -l app=postgres -o jsonpath='{.items[0].metadata.name}')"

step "1) Capture SBOM baseline (generated_at + use_count)"
BASELINE="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -A -F $'\t' -c \
  "SELECT s.id, s.image_digest, s.generated_at, s.use_count
   FROM sboms s
   JOIN pod_image_scans ps ON ps.sbom_id = s.id
   ORDER BY ps.created_at DESC
   LIMIT 1;")"
[[ -n "$BASELINE" ]] || die "Cannot determine baseline SBOM"
SBOM_ID="$(echo "$BASELINE" | cut -f1)"
SBOM_DIGEST="$(echo "$BASELINE" | cut -f2)"
GEN_AT="$(echo "$BASELINE" | cut -f3)"
USE_COUNT="$(echo "$BASELINE" | cut -f4)"
echo "[E2E-cachehit] baseline sbom_id=$SBOM_ID digest=$SBOM_DIGEST generated_at=$GEN_AT use_count=$USE_COUNT"

step "2) Clear derived artifacts ONLY (keep sboms + sbom_components)"
kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -v ON_ERROR_STOP=1 -c \
  "DELETE FROM cve_matches;
   DELETE FROM pod_image_scans;
   DELETE FROM insights WHERE type='vulnerability';"

step "3) Re-create the same pod to trigger cache-hit"
kubectl -n "$E2E_NS" delete pod "$TEST_POD_NAME" --ignore-not-found >/dev/null 2>&1 || true

# Use the SAME image tag as the first run (same digest expected)
E2E_IMAGE="${E2E_IMAGE_REPO}:${TAG}"
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
    test: e2e-sbom-cache-hit
spec:
  restartPolicy: Never
  containers:
    - name: app
      image: ${E2E_IMAGE}
      imagePullPolicy: Never
      command: ["sh","-c","echo 'cache hit run' && sleep 3600"]
YAML
kubectl -n "$E2E_NS" wait --for=condition=Ready pod/"$TEST_POD_NAME" --timeout=180s

step "4) Wait for pod_image_scans to be populated again"
deadline=$((SECONDS+180))
while (( SECONDS < deadline )); do
  scan_count="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c \
    "SELECT COUNT(*) FROM pod_image_scans WHERE pod_name='${TEST_POD_NAME}' AND pod_namespace='${E2E_NS}';" | tr -d '[:space:]')"
  if [[ "$scan_count" != "0" ]]; then
    break
  fi
  sleep 5
done
[[ "$scan_count" != "0" ]] || die "Timed out waiting for pod_image_scans for cache-hit run"

step "5) Assert SBOM reused (same digest, generated_at unchanged, use_count increments)"
AFTER="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -A -F $'\t' -c \
  "SELECT id, image_digest, generated_at, use_count
   FROM sboms
   WHERE id=${SBOM_ID};")"
[[ -n "$AFTER" ]] || die "Cannot read SBOM after cache-hit run"
AFTER_DIGEST="$(echo "$AFTER" | cut -f2)"
AFTER_GEN_AT="$(echo "$AFTER" | cut -f3)"
AFTER_USE_COUNT="$(echo "$AFTER" | cut -f4)"
echo "[E2E-cachehit] after sbom_id=$SBOM_ID digest=$AFTER_DIGEST generated_at=$AFTER_GEN_AT use_count=$AFTER_USE_COUNT"

[[ "$AFTER_DIGEST" == "$SBOM_DIGEST" ]] || die "Digest changed; expected cache key stable"
[[ "$AFTER_GEN_AT" == "$GEN_AT" ]] || die "generated_at changed; expected SBOM reuse"
if [[ "$AFTER_USE_COUNT" -le "$USE_COUNT" ]]; then
  die "use_count did not increase (before=$USE_COUNT after=$AFTER_USE_COUNT)"
fi

step "✅ PASS: SBOM cache-hit verified (digest-based reuse)"


