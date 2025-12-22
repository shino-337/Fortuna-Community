#!/usr/bin/env bash
# KSAM End-to-End CVE → Insight verification (v2, event-driven pipeline)
#
# Verifies:
# - Core rebuilt + redeployed (new image tag) and binary is actually updated (imageID changes)
# - DB cache cleared (sboms/sbom_components/cve_matches/pod_image_scans + vulnerability insights)
# - CVE data loaded from OSV JSON into PostgreSQL via cve-loader
# - Creating a pod triggers SBOMWorker → ksam.sbom.created → CVEMatcherWorker
# - Vulnerability Insight appears via /api/v1/insights (no dashboard required)
#
# Requirements:
# - kubectl context points to a cluster with namespace `ksam` (minikube recommended)
# - postgres + nats + ksam-core + ksam-agent are deployed
# - go1.21+ wrapper installed (recommended): `go install golang.org/dl/go1.21.13@latest && go1.21.13 download`
# - curl installed
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

NAMESPACE="${NAMESPACE:-ksam}"
E2E_NS="${E2E_NS:-ksam-e2e}"
TEST_POD_NAME="${TEST_POD_NAME:-ksam-e2e-vuln-debian10}"
TEST_POD_MANIFEST="${TEST_POD_MANIFEST:-${REPO_DIR}/KSAM/deploy/e2e/ksam-e2e-vuln-pod.yaml}"
CLEAR_SQL_FILE="${CLEAR_SQL_FILE:-${REPO_DIR}/KSAM/deploy/e2e/clear_sbom_cve_cache.sql}"
# Default real CVE used for demo:
# - Debian:10 ecosystem (normalized to "debian")
# - Has CVSS severity and a fixed version range (so matching is deterministic)
REAL_CVE_FILE="${REAL_CVE_FILE:-${REPO_DIR}/KSAM/cve-data/all/CVE-2014-0011.json}"
TEST_CVE_ID="${TEST_CVE_ID:-CVE-2014-0011}"

CORE_DEPLOY="${CORE_DEPLOY:-ksam-core}"
CORE_CONTAINER="${CORE_CONTAINER:-core}"
CORE_IMAGE_REPO="${CORE_IMAGE_REPO:-ksam/core}"
CORE_SERVICE="${CORE_SERVICE:-ksam-core}"

E2E_IMAGE_REPO="${E2E_IMAGE_REPO:-ksam/e2e-vuln}"
E2E_IMAGE_CONTEXT="${E2E_IMAGE_CONTEXT:-${REPO_DIR}/KSAM/deploy/e2e/images/debian-dpkg-moin}"

HTTP_LOCAL_PORT="${HTTP_LOCAL_PORT:-18080}"

die() { echo "[E2E] ERROR: $*" >&2; exit 1; }
step() { echo ""; echo "==> $*"; }

need() { command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"; }
need kubectl
need curl
need python3

TAG="${TAG:-e2e-$(date +%Y%m%d%H%M%S)}"
CORE_IMAGE="${CORE_IMAGE_REPO}:${TAG}"
E2E_IMAGE="${E2E_IMAGE_REPO}:${TAG}"
BUILD_TIME_UTC="${BUILD_TIME_UTC:-$(date -u +%Y%m%dT%H%M%SZ)}"
BUILD_COMMIT="${BUILD_COMMIT:-local}"

step "0) Sanity check cluster + components"
kubectl config current-context
kubectl -n "$NAMESPACE" get deploy/"$CORE_DEPLOY" >/dev/null
kubectl -n "$NAMESPACE" get svc/postgres >/dev/null
kubectl -n "$NAMESPACE" get pods -l app=ksam-agent >/dev/null || die "KSAM agent daemonset not found (required to emit pod events)"

step "0.1) Ensure E2E namespace exists AND agent is watching it"
# NOTE: agent (collector.go) starts per-namespace watchers only for namespaces present at agent startup.
# If we create a brand new namespace, we must restart the agent so it re-lists namespaces and starts watchers.
kubectl get ns "$E2E_NS" >/dev/null 2>&1 || kubectl create ns "$E2E_NS" >/dev/null
kubectl -n "$NAMESPACE" rollout restart ds/ksam-agent >/dev/null 2>&1 || true
kubectl -n "$NAMESPACE" rollout status ds/ksam-agent --timeout=180s >/dev/null 2>&1 || true

step "1) Rebuild + deploy Core with a NEW image tag (${CORE_IMAGE})"
# Minikube-friendly build: build directly into minikube's docker daemon.
# This avoids issues with spaces in the project path that can break `minikube image build`.
if command -v minikube >/dev/null 2>&1; then
  need docker
  step "1.1) Building image into minikube docker daemon (imagePullPolicy: Never)"
  eval "$(minikube docker-env)"
  (cd "${REPO_DIR}/KSAM/core" && docker build \
    --build-arg "KSAM_BUILD_VERSION=${TAG}" \
    --build-arg "KSAM_BUILD_COMMIT=${BUILD_COMMIT}" \
    --build-arg "KSAM_BUILD_TIME=${BUILD_TIME_UTC}" \
    -t "$CORE_IMAGE" .)
else
  # Support kind clusters by loading local docker image into kind node cache.
  # Detect kind by current context prefix or explicit KIND_CLUSTER_NAME.
  KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-}"
  if command -v kind >/dev/null 2>&1; then
    CTX="$(kubectl config current-context || true)"
    if [[ -n "${KIND_CLUSTER_NAME}" || "${CTX}" == kind-* ]]; then
      need docker
      if [[ -z "${KIND_CLUSTER_NAME}" ]]; then
        KIND_CLUSTER_NAME="${CTX#kind-}"
      fi
      step "1.1) Building image locally and loading into kind cluster (${KIND_CLUSTER_NAME})"
      (cd "${REPO_DIR}/KSAM/core" && docker build \
        --build-arg "KSAM_BUILD_VERSION=${TAG}" \
        --build-arg "KSAM_BUILD_COMMIT=${BUILD_COMMIT}" \
        --build-arg "KSAM_BUILD_TIME=${BUILD_TIME_UTC}" \
        -t "$CORE_IMAGE" .)
      kind load docker-image --name "${KIND_CLUSTER_NAME}" "$CORE_IMAGE"
    else
      step "1.1) Building image via local docker (non-minikube)"
      need docker
      (cd "${REPO_DIR}/KSAM/core" && docker build \
        --build-arg "KSAM_BUILD_VERSION=${TAG}" \
        --build-arg "KSAM_BUILD_COMMIT=${BUILD_COMMIT}" \
        --build-arg "KSAM_BUILD_TIME=${BUILD_TIME_UTC}" \
        -t "$CORE_IMAGE" .)
    fi
  else
  step "1.1) Building image via local docker (non-minikube)"
  need docker
  (cd "${REPO_DIR}/KSAM/core" && docker build \
    --build-arg "KSAM_BUILD_VERSION=${TAG}" \
    --build-arg "KSAM_BUILD_COMMIT=${BUILD_COMMIT}" \
    --build-arg "KSAM_BUILD_TIME=${BUILD_TIME_UTC}" \
    -t "$CORE_IMAGE" .)
  fi
fi

kubectl -n "$NAMESPACE" set image deploy/"$CORE_DEPLOY" "$CORE_CONTAINER"="$CORE_IMAGE"
kubectl -n "$NAMESPACE" rollout status deploy/"$CORE_DEPLOY" --timeout=180s

# Force pod restart to ensure new image is picked up (fixes image caching issue with imagePullPolicy: Never)
echo "[E2E] Forcing pod restart to pick up new image..."
kubectl -n "$NAMESPACE" rollout restart deploy/"$CORE_DEPLOY"
kubectl -n "$NAMESPACE" rollout status deploy/"$CORE_DEPLOY" --timeout=180s

CORE_POD="$(kubectl -n "$NAMESPACE" get pods -l app=ksam-core -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.spec.containers[0].image}{"\t"}{.status.phase}{"\t"}{.status.containerStatuses[0].imageID}{"\n"}{end}' \
  | awk -v img="$CORE_IMAGE" '$2==img && $3=="Running" {print $1; exit}')"
[[ -n "${CORE_POD}" ]] || die "Cannot find Running ksam-core pod for image ${CORE_IMAGE}"

CORE_IMAGE_ID="$(kubectl -n "$NAMESPACE" get pod "$CORE_POD" -o jsonpath='{.status.containerStatuses[0].imageID}')"
echo "[E2E] Core pod=${CORE_POD} image=${CORE_IMAGE} imageID=${CORE_IMAGE_ID}"

# Strong verification: the running binary logs the build version/time from ldflags.
echo "[E2E] Waiting for core logs to contain build marker..."
found_marker="false"
for i in $(seq 1 20); do
  # Avoid SIGPIPE/pipefail flakiness by capturing logs first (kubectl logs can exit 141 when piped).
  LOGS="$(kubectl -n "$NAMESPACE" logs "$CORE_POD" 2>/dev/null || true)"
  # IMPORTANT: don't pipe into `grep -q` under `set -o pipefail` (can SIGPIPE and fail the check).
  if grep -q "\\[Build\\] version=${TAG}" <<<"$LOGS"; then
    found_marker="true"
    break
  fi
  sleep 2
done

if [[ "$found_marker" != "true" ]]; then
  echo "[E2E] DEBUG: Showing first 120 lines of core logs:"
  echo "$LOGS" | head -120 || true
  die "Did not find build marker in core logs. Expected '[Build] version=${TAG}'. Binary may not be updated."
fi
echo "[E2E] ✅ Build marker verified in core logs"

step "1.2) Build and load E2E vulnerable image (${E2E_IMAGE}) (dpkg status only, offline)"
if command -v minikube >/dev/null 2>&1; then
  need docker
  eval "$(minikube docker-env)"
  docker build -t "$E2E_IMAGE" "$E2E_IMAGE_CONTEXT"
else
  if command -v kind >/dev/null 2>&1; then
    CTX="$(kubectl config current-context || true)"
    KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-}"
    if [[ -n "${KIND_CLUSTER_NAME}" || "${CTX}" == kind-* ]]; then
      need docker
      if [[ -z "${KIND_CLUSTER_NAME}" ]]; then
        KIND_CLUSTER_NAME="${CTX#kind-}"
      fi
      docker build -t "$E2E_IMAGE" "$E2E_IMAGE_CONTEXT"
      kind load docker-image --name "${KIND_CLUSTER_NAME}" "$E2E_IMAGE"
    else
      need docker
      docker build -t "$E2E_IMAGE" "$E2E_IMAGE_CONTEXT"
    fi
  else
    need docker
    docker build -t "$E2E_IMAGE" "$E2E_IMAGE_CONTEXT"
  fi
fi

step "2) Clear SBOM/CVE caches in PostgreSQL (dev only)"
POSTGRES_POD="$(kubectl -n "$NAMESPACE" get pod -l app=postgres -o jsonpath='{.items[0].metadata.name}')"
# Pass local SQL file via stdin into the pod (requires -i), otherwise psql reads nothing.
kubectl -n "$NAMESPACE" exec -i "$POSTGRES_POD" -- psql -U postgres -d ksam -v ON_ERROR_STOP=1 -f "/dev/stdin" < "$CLEAR_SQL_FILE"

step "2.1) Verify cache is cleared"
kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -v ON_ERROR_STOP=1 -c \
  "SELECT
    (SELECT COUNT(*) FROM sboms) AS sboms,
    (SELECT COUNT(*) FROM sbom_components) AS sbom_components,
    (SELECT COUNT(*) FROM cve_matches) AS cve_matches,
    (SELECT COUNT(*) FROM pod_image_scans) AS pod_image_scans,
    (SELECT COUNT(*) FROM insights WHERE type='vulnerability') AS vuln_insights;"

step "3) Load REAL CVE (${TEST_CVE_ID}) from existing dataset into PostgreSQL via cve-loader"
[[ -f "${REAL_CVE_FILE}" ]] || die "REAL_CVE_FILE not found: ${REAL_CVE_FILE}"

# Ensure the CVE ID is cleared before re-loading (idempotent E2E)
kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -v ON_ERROR_STOP=1 -c \
  "DELETE FROM package_vulnerabilities WHERE cve_id='${TEST_CVE_ID}'; DELETE FROM cves WHERE cve_id='${TEST_CVE_ID}';"

# NOTE: Do NOT rely on local Go toolchain (developer machine may be Go 1.20.x).
# Instead, build a tiny cve-loader image into minikube/kind and run it as an in-cluster Job.
DB_URL="$(kubectl -n "$NAMESPACE" get secret postgres-secret -o jsonpath='{.data.database-url}' | python3 -c 'import sys,base64; print(base64.b64decode(sys.stdin.read().strip()).decode())')"
[[ -n "${DB_URL}" ]] || die "Cannot read postgres-secret.database-url"

CFG_NAME="ksam-e2e-osv-${TAG}"
JOB_NAME="ksam-e2e-cve-loader-${TAG}"

kubectl -n "$NAMESPACE" delete job "${JOB_NAME}" --ignore-not-found >/dev/null 2>&1 || true
kubectl -n "$NAMESPACE" delete configmap "${CFG_NAME}" --ignore-not-found >/dev/null 2>&1 || true
kubectl -n "$NAMESPACE" create configmap "${CFG_NAME}" --from-file="$(basename "${REAL_CVE_FILE}")=${REAL_CVE_FILE}"

need docker
if command -v minikube >/dev/null 2>&1; then
  eval "$(minikube docker-env)"
fi

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
  echo "[E2E] cve-loader job logs:"
  kubectl -n "$NAMESPACE" logs "job/${JOB_NAME}" || true
  die "cve-loader job failed"
}
kubectl -n "$NAMESPACE" logs "job/${JOB_NAME}" || true

step "4) Deploy test pod (custom dpkg status image) to trigger SBOM/CVE pipeline"
kubectl delete -f "$TEST_POD_MANIFEST" --ignore-not-found >/dev/null 2>&1 || true
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
      image: ${E2E_IMAGE}
      imagePullPolicy: Never
      command: ["sh", "-c", "echo 'KSAM E2E pod running' && sleep 3600"]
YAML
kubectl -n "$E2E_NS" wait --for=condition=Ready pod/"$TEST_POD_NAME" --timeout=180s

# CRITICAL: Restart agent AFTER namespace creation to ensure agent watches the namespace
# The agent only lists namespaces at startup, so it won't see newly created namespaces
echo "[E2E] Restarting agent to ensure it watches ${E2E_NS} namespace..."
kubectl -n "$NAMESPACE" rollout restart ds/ksam-agent >/dev/null 2>&1 || true
kubectl -n "$NAMESPACE" rollout status ds/ksam-agent --timeout=60s >/dev/null 2>&1 || true
sleep 10  # Give agent time to start watchers and list namespaces

# CRITICAL: Agent only watches for NEW events, not existing pods.
# Delete and recreate the pod so the agent sees the creation event.
echo "[E2E] Deleting and recreating pod so agent sees the creation event..."
kubectl -n "$E2E_NS" delete pod "$TEST_POD_NAME" --ignore-not-found >/dev/null 2>&1 || true
sleep 2
kubectl apply -f - <<YAML
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
      image: ${E2E_IMAGE}
      imagePullPolicy: Never
      command: ["sh", "-c", "echo 'KSAM E2E pod running' && sleep 3600"]
YAML
kubectl -n "$E2E_NS" wait --for=condition=Ready pod/"$TEST_POD_NAME" --timeout=180s

step "5) Wait for DB artifacts: sbom + pod_image_scans + cve_matches"
deadline=$((SECONDS+240))
while (( SECONDS < deadline )); do
  SBOM_COUNT="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM sboms;" | tr -d '[:space:]')"
  SCAN_COUNT="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM pod_image_scans;" | tr -d '[:space:]')"
  MATCH_COUNT="$(kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM cve_matches WHERE cve_id='${TEST_CVE_ID}';" | tr -d '[:space:]')"
  if [[ "${SBOM_COUNT}" != "0" && "${SCAN_COUNT}" != "0" && "${MATCH_COUNT}" != "0" ]]; then
    echo "[E2E] DB ready: sboms=${SBOM_COUNT} pod_image_scans=${SCAN_COUNT} cve_matches(${TEST_CVE_ID})=${MATCH_COUNT}"
    break
  fi
  sleep 5
done
[[ "${MATCH_COUNT}" != "0" ]] || die "Timed out waiting for CVE matches in DB for ${TEST_CVE_ID}"

step "5.1) Verify digest-based linkage (pod_image_scans -> sboms(image_digest) -> cve_matches -> insights)"
# Assert: pod_image_scans has sbom_id, sboms has image_digest, cve_matches has that sbom_id + TEST_CVE_ID,
# and insights has cveId + sbomId + cveMatchId.
kubectl -n "$NAMESPACE" exec "$POSTGRES_POD" -- psql -U postgres -d ksam -v ON_ERROR_STOP=1 -c \
  "WITH latest_scan AS (
     SELECT * FROM pod_image_scans ORDER BY created_at DESC LIMIT 1
   ),
   linked_sbom AS (
     SELECT s.* FROM sboms s JOIN latest_scan ps ON ps.sbom_id = s.id
   ),
   linked_match AS (
     SELECT m.* FROM cve_matches m JOIN latest_scan ps ON ps.sbom_id = m.sbom_id
     WHERE m.cve_id='${TEST_CVE_ID}'
     ORDER BY m.created_at DESC LIMIT 1
   ),
   linked_insight AS (
     SELECT i.* FROM insights i
     JOIN linked_match m ON i.cve_match_id = m.id
     WHERE i.type='vulnerability' AND i.cve_id='${TEST_CVE_ID}'
     ORDER BY i.created_at DESC LIMIT 1
   )
   SELECT
     (SELECT pod_uid FROM latest_scan) AS pod_uid,
     (SELECT container_image FROM latest_scan) AS container_image,
     (SELECT sbom_id FROM latest_scan) AS sbom_id,
     (SELECT image_digest FROM linked_sbom) AS image_digest,
     (SELECT cve_id FROM linked_match) AS matched_cve_id,
     (SELECT severity FROM linked_match) AS matched_severity,
     (SELECT cve_id FROM linked_insight) AS insight_cve_id,
     (SELECT severity FROM linked_insight) AS insight_severity,
     (SELECT sbom_id FROM linked_insight) AS insight_sbom_id,
     (SELECT cve_match_id FROM linked_insight) AS insight_cve_match_id;"

step "6) Verify Insights via Core API (no dashboard)"
# Port-forward Core service
kubectl -n "$NAMESPACE" port-forward svc/"$CORE_SERVICE" "${HTTP_LOCAL_PORT}:8080" >/tmp/ksam-e2e-core-pf.log 2>&1 &
PF_CORE_PID=$!
trap '[[ -n "${PF_CORE_PID:-}" ]] && kill ${PF_CORE_PID} >/dev/null 2>&1 || true; [[ -n "${PF_PG_PID:-}" ]] && kill ${PF_PG_PID} >/dev/null 2>&1 || true' EXIT
sleep 2
# Verify port-forward is running
if ! kill -0 ${PF_CORE_PID} 2>/dev/null; then
  echo "[E2E] Port-forward failed to start. Logs:"
  cat /tmp/ksam-e2e-core-pf.log || true
  die "Port-forward for Core service failed"
fi

# Wait for Core HTTP to become ready (port-forward can take a moment)
READY_URL="http://localhost:${HTTP_LOCAL_PORT}/ready"
for i in $(seq 1 30); do
  if curl -sSf "${READY_URL}" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

# Login (AUTH_ENABLED=true in deployment) with retry
LOGIN_URL="http://localhost:${HTTP_LOCAL_PORT}/api/v1/auth/login"
LOGIN_BODY='{"username":"admin","password":"admin123"}'
TOKEN=""
for retry in $(seq 1 5); do
  LOGIN_RAW="$(curl -sS -X POST "${LOGIN_URL}" -H "Content-Type: application/json" -d "${LOGIN_BODY}" -w $'\nHTTP_STATUS:%{http_code}\n' 2>&1 || true)"
  LOGIN_HTTP="$(echo "${LOGIN_RAW}" | awk -F: '/^HTTP_STATUS:/{print $2}' | tr -d '[:space:]')"
  LOGIN_RESP="$(echo "${LOGIN_RAW}" | sed '/^HTTP_STATUS:/d')"
  
  if [[ -n "${LOGIN_RESP}" && "${LOGIN_HTTP}" == "200" ]]; then
    TOKEN="$(python3 -c 'import sys,json; raw=sys.stdin.read().strip(); print(json.loads(raw).get("token","") if raw else "")' <<<"${LOGIN_RESP}" 2>/dev/null || true)"
    if [[ -n "${TOKEN}" ]]; then
      break
    fi
  fi
  echo "[E2E] Login retry ${retry}/5: http=${LOGIN_HTTP}, resp_len=${#LOGIN_RESP}"
  sleep 2
done

if [[ -z "${TOKEN}" ]]; then
  echo "[E2E] Login failed. http=${LOGIN_HTTP}"
  echo "[E2E] Login response (first 400 bytes):"
  echo "${LOGIN_RESP}" | head -c 400 || true
  echo ""
  die "Failed to login to Core API. Check core logs/auth and port-forward."
fi

# Query insights with retry
INSIGHTS_URL="http://localhost:${HTTP_LOCAL_PORT}/api/v1/insights?type=vulnerability&status=all&pageSize=50"
INSIGHTS_JSON=""
INSIGHTS_HTTP=""
for retry in $(seq 1 5); do
  # Separate stdout (JSON + HTTP_STATUS) from stderr
  INSIGHTS_RAW="$(curl -sS "${INSIGHTS_URL}" -H "Authorization: Bearer ${TOKEN}" -w $'\nHTTP_STATUS:%{http_code}\n' 2>/tmp/ksam-curl-err.log || true)"
  INSIGHTS_HTTP="$(echo "${INSIGHTS_RAW}" | awk -F: '/^HTTP_STATUS:/{print $2}' | tr -d '[:space:]')"
  INSIGHTS_JSON="$(echo "${INSIGHTS_RAW}" | sed '/^HTTP_STATUS:/d' | sed '/^$/d')"
  
  if [[ -n "${INSIGHTS_JSON}" && "${INSIGHTS_HTTP}" == "200" ]]; then
    # Validate JSON - check if it's valid and non-empty
    if python3 -c "import json,sys; data=json.loads(sys.stdin.read()); assert isinstance(data, dict), 'Not a dict'; assert 'insights' in data, 'Missing insights key'" <<<"${INSIGHTS_JSON}" 2>/dev/null; then
      break
    else
      echo "[E2E] Insights API retry ${retry}/5: Invalid JSON structure"
      echo "[E2E] Response preview: ${INSIGHTS_JSON:0:200}"
    fi
  else
    echo "[E2E] Insights API retry ${retry}/5: http=${INSIGHTS_HTTP}, resp_len=${#INSIGHTS_JSON}"
    if [[ -f /tmp/ksam-curl-err.log ]]; then
      echo "[E2E] curl stderr: $(cat /tmp/ksam-curl-err.log)"
    fi
  fi
  sleep 2
done

if [[ "${INSIGHTS_HTTP}" != "200" || -z "${INSIGHTS_JSON}" ]]; then
  echo "[E2E] Insights API failed. http=${INSIGHTS_HTTP}, resp_len=${#INSIGHTS_JSON}"
  echo "[E2E] Response (first 400 bytes):"
  echo "${INSIGHTS_JSON}" | head -c 400 || true
  echo ""
  die "Insights API returned non-200 or empty response"
fi

echo "$INSIGHTS_JSON" | python3 - <<'PY'
import json,sys
try:
  data=json.load(sys.stdin)
  ins=data.get("insights",[])
  print(f"[E2E] insights returned: {len(ins)}")
  for i in ins[:5]:
    print(f"- id={i.get('id')} cveId={i.get('cveId')} severity={i.get('severity')} pkg={i.get('packageName')} installed={i.get('installedVersion')} fixed={i.get('fixedVersion')}")
except Exception as e:
  print(f"[E2E] Error parsing insights JSON: {e}")
  sys.exit(1)
PY

FOUND="$(echo "$INSIGHTS_JSON" | python3 -c "import json,sys; d=json.load(sys.stdin); print(any(i.get('cveId')=='${TEST_CVE_ID}' for i in d.get('insights',[])))" 2>/dev/null || echo "False")"
[[ "${FOUND}" == "True" ]] || die "Did not find vulnerability insight with cveId=${TEST_CVE_ID}"

step "✅ E2E PASS: CVE detected and visible via Insights API"
echo "[E2E] Tip: check DB rows for digest linkage:"
echo "  kubectl -n ${NAMESPACE} exec ${POSTGRES_POD} -- psql -U postgres -d ksam -c \"SELECT pod_uid, container_image, sbom_id FROM pod_image_scans ORDER BY created_at DESC LIMIT 5;\""
echo "  kubectl -n ${NAMESPACE} exec ${POSTGRES_POD} -- psql -U postgres -d ksam -c \"SELECT image_digest, image_name, image_tag FROM sboms ORDER BY created_at DESC LIMIT 5;\""


