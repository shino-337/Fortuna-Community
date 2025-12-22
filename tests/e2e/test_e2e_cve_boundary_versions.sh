#!/usr/bin/env bash
# E2E Testcase: CVE version boundary (below fixed vs at fixed)
#
# Uses a real CVE from cve-data/all:
# - CVE-2014-0011 (Debian:10, package=vnc4, fixed=4.1.1+X4.3.0+t-1, has CVSS)
# Then tests:
# - vulnerable version: 4.1.1+X4.3.0+t-0  -> MUST produce cve_match + vulnerability insight
# - fixed version:      4.1.1+X4.3.0+t-1  -> MUST NOT produce cve_match/insight for this CVE
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

NAMESPACE="${NAMESPACE:-ksam}"
E2E_NS="${E2E_NS:-ksam-e2e}"
TAG="${TAG:-e2e-boundary-$(date +%Y%m%d%H%M%S)}"

REAL_CVE_FILE="${REAL_CVE_FILE:-${REPO_DIR}/KSAM/cve-data/all/CVE-2014-0011.json}"
TEST_CVE_ID="${TEST_CVE_ID:-CVE-2014-0011}"

VULN_CTX="${VULN_CTX:-${REPO_DIR}/KSAM/deploy/e2e/images/debian-dpkg-moin}"
FIXED_CTX="${FIXED_CTX:-${REPO_DIR}/KSAM/deploy/e2e/images/debian-dpkg-vnc4-fixed}"

CORE_DEPLOY="${CORE_DEPLOY:-ksam-core}"
CORE_CONTAINER="${CORE_CONTAINER:-core}"
CORE_IMAGE_REPO="${CORE_IMAGE_REPO:-ksam/core}"

VULN_IMAGE="${VULN_IMAGE:-ksam/e2e-vuln:${TAG}-vuln}"
FIXED_IMAGE="${FIXED_IMAGE:-ksam/e2e-vuln:${TAG}-fixed}"

die() { echo "[E2E-boundary] ERROR: $*" >&2; exit 1; }
step() { echo ""; echo "==> $*"; }
need() { command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"; }

need kubectl
need docker
need curl
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
  echo "[E2E-boundary] cve-loader job logs:"
  kubectl -n "$NAMESPACE" logs "job/${JOB_NAME}" || true
  die "cve-loader job failed"
}
kubectl -n "$NAMESPACE" logs "job/${JOB_NAME}" || true

step "4) Deploy vulnerable pod (expect MATCH + INSIGHT)"
VULN_POD="ksam-e2e-vuln-${TAG}"
kubectl apply -f - <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${E2E_NS}
---
apiVersion: v1
kind: Pod
metadata:
  name: ${VULN_POD}
  namespace: ${E2E_NS}
  labels: { test: e2e-cve-boundary }
spec:
  restartPolicy: Never
  containers:
    - name: app
      image: ${VULN_IMAGE}
      imagePullPolicy: Never
      command: ["sh","-c","echo 'vuln' && sleep 3600"]
YAML
kubectl -n "$E2E_NS" wait --for=condition=Ready pod/"$VULN_POD" --timeout=180s

deadline=$((SECONDS+240))
MATCH_VULN="0"
while (( SECONDS < deadline )); do
  MATCH_VULN="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c \"SELECT COUNT(*) FROM cve_matches WHERE cve_id='${TEST_CVE_ID}';\" | tr -d '[:space:]')"
  if [[ "$MATCH_VULN" != "0" ]]; then break; fi
  sleep 5
done
[[ "$MATCH_VULN" != "0" ]] || die "Expected cve_matches for vulnerable version, got 0"

INSIGHT_VULN="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c \"SELECT COUNT(*) FROM insights WHERE type='vulnerability' AND cve_id='${TEST_CVE_ID}';\" | tr -d '[:space:]')"
[[ "$INSIGHT_VULN" != "0" ]] || die "Expected vulnerability insight for vulnerable version, got 0"

step "5) Clear derived artifacts and deploy fixed pod (expect NO MATCH/INSIGHT)"
kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -v ON_ERROR_STOP=1 -c \
  "DELETE FROM cve_matches; DELETE FROM sbom_components; DELETE FROM sboms; DELETE FROM pod_image_scans; DELETE FROM insights WHERE type='vulnerability';"

FIXED_POD="ksam-e2e-fixed-${TAG}"
kubectl apply -f - <<YAML
apiVersion: v1
kind: Pod
metadata:
  name: ${FIXED_POD}
  namespace: ${E2E_NS}
  labels: { test: e2e-cve-boundary }
spec:
  restartPolicy: Never
  containers:
    - name: app
      image: ${FIXED_IMAGE}
      imagePullPolicy: Never
      command: ["sh","-c","echo 'fixed' && sleep 3600"]
YAML
kubectl -n "$E2E_NS" wait --for=condition=Ready pod/"$FIXED_POD" --timeout=180s

sleep 30
MATCH_FIXED="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c \"SELECT COUNT(*) FROM cve_matches WHERE cve_id='${TEST_CVE_ID}';\" | tr -d '[:space:]')"
INSIGHT_FIXED="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c \"SELECT COUNT(*) FROM insights WHERE type='vulnerability' AND cve_id='${TEST_CVE_ID}';\" | tr -d '[:space:]')"

if [[ "$MATCH_FIXED" != "0" || "$INSIGHT_FIXED" != "0" ]]; then
  die "Expected NO match/insight for fixed version. matches=$MATCH_FIXED insights=$INSIGHT_FIXED"
fi

step "✅ PASS: boundary version behavior verified (below fixed matches; at fixed does not)"


