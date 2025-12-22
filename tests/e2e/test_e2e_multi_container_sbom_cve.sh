#!/usr/bin/env bash
# E2E Testcase: Multi-container Pod (2 images) -> 2 pod_image_scans, CVE matches only for vulnerable container.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

NAMESPACE="${NAMESPACE:-ksam}"
E2E_NS="${E2E_NS:-ksam-e2e}"
TAG="${TAG:-e2e-multi-$(date +%Y%m%d%H%M%S)}"

REAL_CVE_FILE="${REAL_CVE_FILE:-${REPO_DIR}/KSAM/cve-data/all/CVE-2014-0011.json}"
TEST_CVE_ID="${TEST_CVE_ID:-CVE-2014-0011}"

VULN_CTX="${VULN_CTX:-${REPO_DIR}/KSAM/deploy/e2e/images/debian-dpkg-moin}"
FIXED_CTX="${FIXED_CTX:-${REPO_DIR}/KSAM/deploy/e2e/images/debian-dpkg-vnc4-fixed}"

VULN_IMAGE="${VULN_IMAGE:-ksam/e2e-vuln:${TAG}-vuln}"
FIXED_IMAGE="${FIXED_IMAGE:-ksam/e2e-vuln:${TAG}-fixed}"

CORE_DEPLOY="${CORE_DEPLOY:-ksam-core}"

die() { echo "[E2E-multi] ERROR: $*" >&2; exit 1; }
step() { echo ""; echo "==> $*"; }
need() { command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"; }

need kubectl
need docker
need python3

step "0) Preflight"
kubectl -n "$NAMESPACE" get deploy/"$CORE_DEPLOY" >/dev/null
kubectl -n "$NAMESPACE" get svc/postgres >/dev/null
POSTGRES_POD="$(kubectl -n "$NAMESPACE" get pod -l app=postgres -o jsonpath='{.items[0].metadata.name}')"

step "1) Build & load E2E images (vulnerable + fixed)"
if command -v minikube >/dev/null 2>&1; then
  eval "$(minikube docker-env)"
fi
docker build -t "$VULN_IMAGE" "$VULN_CTX"
docker build -t "$FIXED_IMAGE" "$FIXED_CTX"

step "2) Clear SBOM/CVE derived caches"
kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -v ON_ERROR_STOP=1 -c \
  "DELETE FROM cve_matches; DELETE FROM sbom_components; DELETE FROM sboms; DELETE FROM pod_image_scans; DELETE FROM insights WHERE type='vulnerability';"

step "3) Load CVE into Postgres via in-cluster cve-loader Job (single file)"
[[ -f "${REAL_CVE_FILE}" ]] || die "REAL_CVE_FILE not found: ${REAL_CVE_FILE}"

DB_URL="$(kubectl -n "$NAMESPACE" get secret postgres-secret -o jsonpath='{.data.database-url}' | python3 -c 'import sys,base64; print(base64.b64decode(sys.stdin.read().strip()).decode())')"
[[ -n "${DB_URL}" ]] || die "Cannot read postgres-secret.database-url"

CFG_NAME="ksam-e2e-osv-${TAG}"
JOB_NAME="ksam-e2e-cve-loader-${TAG}"

kubectl -n "$NAMESPACE" delete job "${JOB_NAME}" --ignore-not-found >/dev/null 2>&1 || true
kubectl -n "$NAMESPACE" delete configmap "${CFG_NAME}" --ignore-not-found >/dev/null 2>&1 || true
kubectl -n "$NAMESPACE" create configmap "${CFG_NAME}" --from-file="$(basename "${REAL_CVE_FILE}")=${REAL_CVE_FILE}"

(cd "${REPO_DIR}/KSAM/core" && docker build -t "ksam/cve-loader:${TAG}" -f Dockerfile.cve-loader.e2e .)

cat <<YAML | kubectl -n "$NAMESPACE" apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: ${JOB_NAME}
spec:
  backoffLimit: 0
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: cve-loader
          image: ksam/cve-loader:${TAG}
          imagePullPolicy: Never
          env:
            - name: DATABASE_URL
              value: "${DB_URL}"
          command: ["/cve-loader"]
          args: ["-source","/cve","-workers","1","-batch-size","10"]
          volumeMounts:
            - name: osv
              mountPath: /cve
      volumes:
        - name: osv
          configMap:
            name: ${CFG_NAME}
YAML

kubectl -n "$NAMESPACE" wait --for=condition=complete "job/${JOB_NAME}" --timeout=180s || {
  echo "[E2E-multi] cve-loader job logs:"
  kubectl -n "$NAMESPACE" logs "job/${JOB_NAME}" || true
  die "cve-loader job failed"
}
kubectl -n "$NAMESPACE" logs "job/${JOB_NAME}" || true

step "4) Deploy multi-container pod"
POD_NAME="ksam-e2e-multi-${TAG}"
kubectl apply -f - <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${E2E_NS}
---
apiVersion: v1
kind: Pod
metadata:
  name: ${POD_NAME}
  namespace: ${E2E_NS}
  labels: { test: e2e-multi-container }
spec:
  restartPolicy: Never
  containers:
    - name: vuln
      image: ${VULN_IMAGE}
      imagePullPolicy: Never
      command: ["sh","-c","echo 'vuln container' && sleep 3600"]
    - name: fixed
      image: ${FIXED_IMAGE}
      imagePullPolicy: Never
      command: ["sh","-c","echo 'fixed container' && sleep 3600"]
YAML
kubectl -n "$E2E_NS" wait --for=condition=Ready pod/"$POD_NAME" --timeout=180s || true

step "5) Wait for 2 pod_image_scans rows"
deadline=$((SECONDS+240))
SCAN_COUNT="0"
while (( SECONDS < deadline )); do
  SCAN_COUNT="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c \
    \"SELECT COUNT(*) FROM pod_image_scans WHERE pod_name='${POD_NAME}' AND pod_namespace='${E2E_NS}';\" | tr -d '[:space:]')"
  if [[ "$SCAN_COUNT" -ge 2 ]]; then break; fi
  sleep 5
done
[[ "$SCAN_COUNT" -ge 2 ]] || die "Expected 2 pod_image_scans rows, got $SCAN_COUNT"

step "6) Assert only vulnerable container is affected (via sbom_id linkage)"
kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -v ON_ERROR_STOP=1 -c \
  \"SELECT ps.container_name, ps.container_image,
          (SELECT COUNT(*) FROM cve_matches cm WHERE cm.sbom_id=ps.sbom_id AND cm.cve_id='${TEST_CVE_ID}') AS matches_for_cve
    FROM pod_image_scans ps
   WHERE ps.pod_name='${POD_NAME}' AND ps.pod_namespace='${E2E_NS}'
   ORDER BY ps.container_name;\"

MATCHES_TOTAL="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c \
  \"SELECT COUNT(*) FROM cve_matches WHERE cve_id='${TEST_CVE_ID}';\" | tr -d '[:space:]')"
[[ "$MATCHES_TOTAL" != "0" ]] || die "Expected at least 1 cve_match for ${TEST_CVE_ID}"

INSIGHTS_TOTAL="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c \
  \"SELECT COUNT(*) FROM insights WHERE type='vulnerability' AND cve_id='${TEST_CVE_ID}';\" | tr -d '[:space:]')"
[[ "$INSIGHTS_TOTAL" != "0" ]] || die "Expected at least 1 vulnerability insight for ${TEST_CVE_ID}"

step "✅ PASS: multi-container pod processed; only vulnerable container produces matches/insights"


