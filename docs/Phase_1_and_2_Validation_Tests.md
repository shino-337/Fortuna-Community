Phase 1 Validation Tests
TC-P1-001: Database Schema Validation
Objective: Verify all database tables, indexes, and constraints are created correctly
Pre-conditions:

PostgreSQL deployed
Migrations executed

Test Steps:
sql-- 1. Verify core tables exist
SELECT table_name FROM information_schema.tables 
WHERE table_schema = 'public' 
ORDER BY table_name;

-- Expected: clusters, service_accounts, pods, roles, role_bindings, 
--           insights, events, baselines, policies, users, audit_logs

-- 2. Verify TimescaleDB extension
SELECT * FROM pg_extension WHERE extname = 'timescaledb';
-- Expected: 1 row

-- 3. Verify Apache AGE extension
SELECT * FROM pg_extension WHERE extname = 'age';
-- Expected: 1 row

-- 4. Verify hypertable for events
SELECT * FROM timescaledb_information.hypertables 
WHERE hypertable_name = 'events';
-- Expected: 1 row

-- 5. Verify indexes
SELECT indexname FROM pg_indexes 
WHERE tablename IN ('service_accounts', 'insights', 'events')
ORDER BY indexname;
-- Expected: Indexes on id, cluster_id, namespace, risk_score, etc.

-- 6. Verify foreign key constraints
SELECT conname, conrelid::regclass, confrelid::regclass 
FROM pg_constraint 
WHERE contype = 'f';
-- Expected: FK constraints between tables
Expected Results:

✅ All tables created
✅ TimescaleDB extension enabled
✅ Apache AGE extension enabled
✅ events table is hypertable
✅ All indexes present
✅ Foreign keys enforced

Pass Criteria: All SQL queries return expected results

TC-P1-002: Component Build Verification
Objective: Verify all components compile without errors
Test Steps:
bash# 1. Build Agent
cd agent
go build -o bin/agent ./cmd/agent
echo "Exit code: $?"
# Expected: Exit code: 0

# 2. Build Core
cd ../core
go build -o bin/core ./cmd/core
echo "Exit code: $?"
# Expected: Exit code: 0

# 3. Build Dashboard
cd ../dashboard
npm run build
echo "Exit code: $?"
# Expected: Exit code: 0

# 4. Run unit tests
cd ../agent && go test ./... -v
cd ../core && go test ./... -v
cd ../dashboard && npm test
Expected Results:

✅ Agent builds successfully
✅ Core builds successfully
✅ Dashboard builds successfully
✅ No compilation errors
✅ Unit tests pass

Pass Criteria: All builds exit with code 0, unit tests pass

TC-P1-003: Component Deployment Verification
Objective: Verify all components deploy and run in Kubernetes
Test Steps:
bash# 1. Check all pods are running
kubectl get pods -n ksam-test

# Expected output:
# NAME                            READY   STATUS    RESTARTS
# postgres-0                      1/1     Running   0
# nats-0                          1/1     Running   0
# ksam-core-xxxxx-yyyyy          1/1     Running   0
# ksam-agent-xxxxx (on each node) 1/1     Running   0
# ksam-dashboard-xxxxx-yyyyy     1/1     Running   0

# 2. Check pod resource usage
kubectl top pods -n ksam-test

# Expected:
# ksam-agent: < 128Mi memory, < 200m CPU
# ksam-core: < 1Gi memory, < 1000m CPU

# 3. Check logs for errors
kubectl logs -n ksam-test -l app=ksam-core --tail=50
kubectl logs -n ksam-test -l app=ksam-agent --tail=50

# Expected: No ERROR level logs on startup
Expected Results:

✅ All pods in Running state
✅ No CrashLoopBackOff
✅ Resource usage within limits
✅ No critical errors in logs

Pass Criteria: All pods healthy for 5+ minutes

TC-P1-004: Database Connectivity
Objective: Verify core can connect to database
Test Steps:
bash# 1. Check core logs for DB connection
kubectl logs -n ksam-test -l app=ksam-core | grep -i "database"

# Expected: "Database connected successfully"

# 2. Execute test query from core
kubectl exec -it ksam-core-xxxxx -n ksam-test -- \
  /bin/sh -c 'psql $DATABASE_URL -c "SELECT 1;"'

# Expected: 
#  ?column? 
# ----------
#        1

# 3. Verify connection pooling
kubectl logs -n ksam-test -l app=ksam-core | grep -i "pool"

# Expected: Connection pool initialized (size: 25)
Expected Results:

✅ Core connects to database on startup
✅ Connection pool initialized
✅ Queries execute successfully
✅ TLS connection established (if enabled)

Pass Criteria: Database operations functional

Phase 2 Validation Tests
TC-P2-001: Agent Data Collection - Pods
Objective: Verify agent collects pod inventory data
Pre-conditions:

Agent running on all nodes
Test pods deployed in test-workloads namespace

Test Steps:
bash# 1. Deploy test pod
kubectl run test-nginx --image=nginx:latest -n test-workloads

# 2. Wait for agent to collect (30 seconds)
sleep 30

# 3. Query database for pod
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT id, name, namespace, image, status 
FROM pods 
WHERE name = 'test-nginx';
EOF

# Expected: 1 row with test-nginx details

# 4. Verify via API
curl -H "Authorization: Bearer $TOKEN" \
  http://ksam-core:8080/api/v1/pods?name=test-nginx | jq

# Expected: JSON with pod details
Expected Results:

✅ Pod appears in database within 30 seconds
✅ All fields populated correctly (name, namespace, image, labels)
✅ API returns pod data
✅ Pod status tracked

Pass Criteria: Pod inventory data collected and accessible
Test Data:
yaml# test-pod.yaml
apiVersion: v1
kind: Pod
metadata:
  name: test-nginx
  namespace: test-workloads
  labels:
    app: nginx
    tier: frontend
spec:
  serviceAccountName: default
  containers:
  - name: nginx
    image: nginx:latest
    ports:
    - containerPort: 80

TC-P2-002: Agent Data Collection - ServiceAccounts
Objective: Verify agent collects ServiceAccount inventory
Test Steps:
bash# 1. Create test ServiceAccount
kubectl create sa test-sa -n test-workloads

# 2. Wait for collection
sleep 30

# 3. Query database
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT id, name, namespace, created_at 
FROM service_accounts 
WHERE name = 'test-sa';
EOF

# Expected: 1 row

# 4. Verify secrets association
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT sa.name, COUNT(s.id) as secret_count
FROM service_accounts sa
LEFT JOIN secrets s ON sa.id = s.service_account_id
WHERE sa.name = 'test-sa'
GROUP BY sa.name;
EOF

# Expected: test-sa with secret_count >= 1 (auto-generated token)
Expected Results:

✅ ServiceAccount collected
✅ Namespace correct
✅ Secrets associated
✅ Timestamps populated

Pass Criteria: Complete SA inventory with relationships

TC-P2-003: Agent Data Collection - RBAC
Objective: Verify agent collects Roles and RoleBindings
Test Steps:
bash# 1. Create Role and RoleBinding
kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: test-editor
  namespace: test-workloads
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "create"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: test-sa-editor
  namespace: test-workloads
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: test-editor
subjects:
- kind: ServiceAccount
  name: test-sa
  namespace: test-workloads
EOF

# 2. Wait for collection
sleep 30

# 3. Verify Role
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT name, namespace, rules 
FROM roles 
WHERE name = 'test-editor';
EOF

# Expected: 1 row with JSON rules

# 4. Verify RoleBinding
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT rb.name, r.name as role_name, sa.name as sa_name
FROM role_bindings rb
JOIN roles r ON rb.role_id = r.id
JOIN service_accounts sa ON rb.subject_id = sa.id
WHERE rb.name = 'test-sa-editor';
EOF

# Expected: 1 row linking test-sa to test-editor
Expected Results:

✅ Role collected with rules
✅ RoleBinding collected
✅ Relationships established (SA → RoleBinding → Role)
✅ Permissions parseable

Pass Criteria: Complete RBAC graph constructed

TC-P2-004: Core Data Processing - Normalization
Objective: Verify core normalizes incoming inventory data
Test Steps:
bash# 1. Check core logs for normalization
kubectl logs -n ksam-test -l app=ksam-core | grep "Normalized"

# Expected: "Normalized Pod: test-nginx"

# 2. Verify normalized data format in DB
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT 
  name,
  jsonb_typeof(spec) as spec_type,
  jsonb_typeof(labels) as labels_type,
  jsonb_typeof(annotations) as annotations_type
FROM pods 
WHERE name = 'test-nginx';
EOF

# Expected: 
# - spec_type: object
# - labels_type: object
# - annotations_type: object

# 3. Check for data validation
kubectl logs -n ksam-test -l app=ksam-core | grep -i "validation"

# Expected: No validation errors
Expected Results:

✅ Data normalized to standard format
✅ JSON fields properly structured
✅ Invalid data rejected
✅ Timestamps in UTC

Pass Criteria: All inventory data normalized correctly

TC-P2-005: Core Data Processing - Correlation
Objective: Verify core correlates related resources
Test Steps:
bash# 1. Verify Pod → ServiceAccount correlation
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT p.name as pod_name, sa.name as sa_name
FROM pods p
JOIN service_accounts sa ON p.service_account_id = sa.id
WHERE p.name = 'test-nginx';
EOF

# Expected: test-nginx linked to default or test-sa

# 2. Verify ServiceAccount → Role correlation
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT 
  sa.name as sa_name, 
  r.name as role_name
FROM service_accounts sa
JOIN role_bindings rb ON sa.id = rb.subject_id
JOIN roles r ON rb.role_id = r.id
WHERE sa.name = 'test-sa';
EOF

# Expected: test-sa linked to test-editor

# 3. Check correlation logs
kubectl logs -n ksam-test -l app=ksam-core | grep "Correlated"

# Expected: "Correlated Pod test-nginx with ServiceAccount default"
Expected Results:

✅ Pod-SA relationships established
✅ SA-Role relationships established
✅ Foreign keys valid
✅ Orphaned resources detected

Pass Criteria: Resource graph complete and accurate

TC-P2-006: Risk Insights - CIS 5.1.3 Detection
Objective: Verify detection of ServiceAccount with cluster-admin binding
Test Steps:
bash# 1. Create high-risk scenario
kubectl create sa dangerous-sa -n test-workloads
kubectl create clusterrolebinding dangerous-binding \
  --clusterrole=cluster-admin \
  --serviceaccount=test-workloads:dangerous-sa

# 2. Wait for risk engine (30 seconds)
sleep 30

# 3. Query insights
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT 
  rule_id,
  title,
  severity,
  risk_score,
  resource_type,
  resource_id
FROM insights
WHERE rule_id = 'cis-5.1.3'
ORDER BY detected_at DESC
LIMIT 5;
EOF

# Expected: 1 insight for dangerous-sa with severity=critical

# 4. Verify via API
curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/insights?severity=critical&rule_id=cis-5.1.3" \
  | jq '.data[] | {title, severity, risk_score}'

# Expected:
# {
#   "title": "ServiceAccount with cluster-admin binding",
#   "severity": "critical",
#   "risk_score": 9.5
# }
Expected Results:

✅ Insight created within 30 seconds
✅ Rule ID = cis-5.1.3
✅ Severity = critical
✅ Risk score ≈ 9.5
✅ Resource correctly identified

Pass Criteria: High-risk RBAC detected and reported

TC-P2-007: Risk Insights - CIS 5.2.1 Detection
Objective: Verify detection of privileged containers
Test Steps:
bash# 1. Deploy privileged pod
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: privileged-pod
  namespace: test-workloads
spec:
  containers:
  - name: privileged
    image: nginx
    securityContext:
      privileged: true
EOF

# 2. Wait for risk engine
sleep 30

# 3. Query insights
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT title, severity, risk_score
FROM insights
WHERE rule_id = 'cis-5.2.1'
  AND resource_type = 'Pod'
ORDER BY detected_at DESC
LIMIT 1;
EOF

# Expected:
# title: "Privileged container detected"
# severity: "critical"
# risk_score: 9.0

# 4. Check remediation
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT remediation
FROM insights
WHERE rule_id = 'cis-5.2.1'
ORDER BY detected_at DESC
LIMIT 1;
EOF

# Expected: JSON with remediation steps
Expected Results:

✅ Privileged container detected
✅ Correct severity (critical)
✅ Risk score ≈ 9.0
✅ Remediation provided

Pass Criteria: Pod security violation detected

TC-P2-008: Risk Insights - Missing NetworkPolicy
Objective: Verify detection of namespaces without NetworkPolicy
Test Steps:
bash# 1. Create namespace without NetworkPolicy
kubectl create namespace no-netpol-ns

# 2. Deploy pod in that namespace
kubectl run test-pod --image=nginx -n no-netpol-ns

# 3. Wait for risk engine
sleep 30

# 4. Query insights
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT title, severity, cis_section
FROM insights
WHERE rule_id = 'cis-5.3.1'
  AND resource_type = 'Namespace'
ORDER BY detected_at DESC
LIMIT 1;
EOF

# Expected:
# title: "Namespace without NetworkPolicy"
# severity: "high"
# cis_section: "5.3.1"
Expected Results:

✅ Missing NetworkPolicy detected
✅ Namespace correctly identified
✅ Severity = high
✅ CIS section mapped

Pass Criteria: Network policy gaps identified

TC-P2-009: API Functionality - Authentication
Objective: Verify API authentication works
Test Steps:
bash# 1. Test login (should succeed)
RESPONSE=$(curl -s -X POST http://ksam-core:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin@example.com",
    "password": "admin123"
  }')

echo $RESPONSE | jq

# Expected:
# {
#   "access_token": "eyJhbG...",
#   "refresh_token": "eyJhbG...",
#   "expires_in": 1800,
#   "user": {
#     "id": "...",
#     "username": "admin@example.com",
#     "role": "admin"
#   }
# }

# 2. Extract token
TOKEN=$(echo $RESPONSE | jq -r '.access_token')

# 3. Test authenticated request (should succeed)
curl -H "Authorization: Bearer $TOKEN" \
  http://ksam-core:8080/api/v1/insights

# Expected: JSON array of insights

# 4. Test without token (should fail with 401)
curl -i http://ksam-core:8080/api/v1/insights

# Expected: HTTP/1.1 401 Unauthorized

# 5. Test with invalid token (should fail with 401)
curl -i -H "Authorization: Bearer invalid_token" \
  http://ksam-core:8080/api/v1/insights

# Expected: HTTP/1.1 401 Unauthorized
Expected Results:

✅ Login returns JWT token
✅ Token expires in 1800 seconds
✅ Authenticated requests succeed
✅ Unauthenticated requests return 401
✅ Invalid tokens rejected

Pass Criteria: Authentication working as expected

TC-P2-010: API Functionality - RBAC Authorization
Objective: Verify API enforces role-based permissions
Test Steps:
bash# 1. Create viewer user
curl -X POST -H "Authorization: Bearer $ADMIN_TOKEN" \
  http://ksam-core:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{
    "username": "viewer@example.com",
    "password": "viewer123",
    "role": "viewer"
  }'

# 2. Login as viewer
VIEWER_TOKEN=$(curl -s -X POST http://ksam-core:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "viewer@example.com",
    "password": "viewer123"
  }' | jq -r '.access_token')

# 3. Test read access (should succeed)
curl -H "Authorization: Bearer $VIEWER_TOKEN" \
  http://ksam-core:8080/api/v1/insights

# Expected: 200 OK with data

# 4. Test write access (should fail with 403)
curl -i -X PATCH -H "Authorization: Bearer $VIEWER_TOKEN" \
  http://ksam-core:8080/api/v1/insights/insight-123 \
  -H "Content-Type: application/json" \
  -d '{"status": "resolved"}'

# Expected: HTTP/1.1 403 Forbidden

# 5. Test admin-only endpoint (should fail with 403)
curl -i -X POST -H "Authorization: Bearer $VIEWER_TOKEN" \
  http://ksam-core:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{...}'

# Expected: HTTP/1.1 403 Forbidden
Expected Results:

✅ Viewer can read resources
✅ Viewer cannot modify resources
✅ Viewer cannot access admin endpoints
✅ 403 Forbidden returned for unauthorized actions

Pass Criteria: RBAC enforced correctly

TC-P2-011: API Functionality - Pagination
Objective: Verify API pagination works correctly
Test Steps:
bash# 1. Get first page
RESPONSE=$(curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/insights?page=1&per_page=5")

echo $RESPONSE | jq '{
  data_count: (.data | length),
  meta: .meta
}'

# Expected:
# {
#   "data_count": 5,
#   "meta": {
#     "total": 47,
#     "page": 1,
#     "per_page": 5,
#     "total_pages": 10
#   }
# }

# 2. Get second page
RESPONSE=$(curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/insights?page=2&per_page=5")

# Expected: Different 5 insights

# 3. Test invalid page (should return empty)
RESPONSE=$(curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/insights?page=999&per_page=5")

echo $RESPONSE | jq '.data | length'

# Expected: 0

# 4. Test per_page limits
RESPONSE=$(curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/insights?per_page=200")

# Expected: Error or capped at max (e.g., 100)
Expected Results:

✅ Pagination metadata correct
✅ Correct number of items per page
✅ Different items on different pages
✅ Invalid pages handled gracefully
✅ per_page capped at maximum

Pass Criteria: Pagination functional and accurate

TC-P2-012: API Functionality - Filtering
Objective: Verify API filtering works
Test Steps:
bash# 1. Filter by severity
curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/insights?severity=critical" \
  | jq '.data[] | .severity' | sort | uniq

# Expected: Only "critical"

# 2. Filter by status
curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/insights?status=active" \
  | jq '.data[] | .status' | sort | uniq

# Expected: Only "active"

# 3. Filter by multiple criteria
curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/insights?severity=critical&status=active&category=rbac" \
  | jq '.data[] | {severity, status, category}'

# Expected: All results match all criteria

# 4. Filter by namespace
curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/serviceaccounts?namespace=test-workloads" \
  | jq '.data[] | .namespace' | sort | uniq

# Expected: Only "test-workloads"
Expected Results:

✅ Single filters work
✅ Multiple filters combine correctly (AND logic)
✅ Case-insensitive filtering
✅ No invalid results returned

Pass Criteria: All filters produce correct results

Functional Test Cases
TC-F001: ServiceAccount Risk Scoring
Objective: Verify ServiceAccount risk scores are calculated correctly
Test Steps:
bash# Setup: Create SAs with different risk levels

# 1. Low risk SA (no permissions)
kubectl create sa low-risk-sa -n test-workloads

# 2. Medium risk SA (read-only)
kubectl create sa medium-risk-sa -n test-workloads
kubectl create rolebinding medium-rb \
  --role=view \
  --serviceaccount=test-workloads:medium-risk-sa \
  -n test-workloads

# 3. High risk SA (edit permissions)
kubectl create sa high-risk-sa -n test-workloads
kubectl create rolebinding high-rb \
  --role=edit \
  --serviceaccount=test-workloads:high-risk-sa \
  -n test-workloads

# 4. Critical risk SA (cluster-admin)
kubectl create sa critical-risk-sa -n test-workloads
kubectl create clusterrolebinding critical-rb \
  --clusterrole=cluster-admin \
  --serviceaccount=test-workloads:critical-risk-sa

# Wait for risk calculation
sleep 60

# 5. Query risk scores
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT name, risk_score, status
FROM service_accounts
WHERE namespace = 'test-workloads'
  AND name LIKE '%risk-sa'
ORDER BY risk_score DESC;
EOF
Expected Results:
ServiceAccountRisk Score RangeStatuscritical-risk-sa9.0 - 10.0activehigh-risk-sa7.0 - 8.9activemedium-risk-sa4.0 - 6.9activelow-risk-sa0.0 - 3.9active
Pass Criteria: Risk scores proportional to permissions

TC-F002: Orphaned ServiceAccount Detection
Objective: Verify unused ServiceAccounts are detected
Test Steps:
bash# 1. Create SA not used by any pod
kubectl create sa orphaned-sa -n test-workloads

# 2. Wait for orphan detection (60 seconds)
sleep 60

# 3. Query for orphaned SAs
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT name, status, last_used
FROM service_accounts
WHERE status = 'orphaned'
  AND namespace = 'test-workloads';
EOF

# Expected: orphaned-sa with status='orphaned', last_used=NULL

# 4. Verify insight created
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT title, severity
FROM insights
WHERE resource_type = 'ServiceAccount'
  AND title LIKE '%orphaned%'
ORDER BY detected_at DESC
LIMIT 1;
EOF

# Expected: Insight about orphaned SA
Expected Results:

✅ Orphaned SA detected
✅ Status = 'orphaned'
✅ Insight created
✅ Remediation suggests deletion

Pass Criteria: Orphan detection accurate

TC-F003: Pod Security Standards Validation
Objective: Verify all Pod Security Standard violations detected
Test Steps:
bash# Deploy pod with multiple violations
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: insecure-pod
  namespace: test-workloads
spec:
  hostPID: true
  hostIPC: true
  hostNetwork: true
  containers:
  - name: insecure
    image: nginx
    securityContext:
      privileged: true
      allowPrivilegeEscalation: true
      runAsUser: 0
      capabilities:
        add: ["SYS_ADMIN", "NET_ADMIN"]
EOF

# Wait for detection
sleep 30

# Query insights
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT rule_id, title, severity
FROM insights
WHERE resource_type = 'Pod'
  AND resource_id IN (
    SELECT id FROM pods WHERE name = 'insecure-pod'
  )
ORDER BY severity DESC, rule_id;
EOF
Expected Insights:
Rule IDTitleSeveritycis-5.2.1Privileged containercriticalcis-5.2.2hostPID enabledhighcis-5.2.3hostIPC enabledhighcis-5.2.4hostNetwork enabledhighcis-5.2.5allowPrivilegeEscalationhighcis-5.2.9Dangerous capabilitiescritical
Pass Criteria: All violations detected (6+ insights)

TC-F004: Secret Exposure Detection
Objective: Verify secrets in environment variables are detected
Test Steps:
bash# 1. Create pod with secret in env var
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: secret-env-pod
  namespace: test-workloads
spec:
  containers:
  - name: app
    image: nginx
    env:
    - name: API_KEY
      valueFrom:
        secretKeyRef:
          name: api-secret
          key: key
EOF

# 2. Wait for detection
sleep 30

# 3. Query insights
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT rule_id, title, severity
FROM insights
WHERE rule_id = 'cis-5.4.1'
  AND resource_type = 'Pod'
ORDER BY detected_at DESC
LIMIT 1;
EOF

# Expected:
# rule_id: cis-5.4.1
# title: "Secret exposed in environment variable"
# severity: "medium"
Expected Results:

✅ Secret exposure detected
✅ Insight created
✅ Remediation suggests volume mount

Pass Criteria: Secret misuse identified

TC-F005: RBAC Permission Analysis
Objective: Verify RBAC permissions are analyzed correctly
Test Steps:
bash# 1. Create SA with excessive permissions
kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: excessive-role
  namespace: test-workloads
rules:
- apiGroups: ["*"]
  resources: ["*"]
  verbs: ["*"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: excessive-binding
  namespace: test-workloads
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: excessive-role
subjects:
- kind: ServiceAccount
  name: test-sa
  namespace: test-workloads
EOF

# 2. Wait for analysis
sleep 30

# 3. Check insights
curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/insights?category=rbac&severity=critical" \
  | jq '.data[] | select(.title | contains("excessive"))'

# Expected: Insight about excessive permissions
Expected Results:

✅ Wildcard permissions detected
✅ Risk score elevated
✅ Insight explains risk
✅ Remediation suggests least privilege

Pass Criteria: RBAC analysis comprehensive

Integration Test Cases
TC-I001: End-to-End Pod Lifecycle
Objective: Test complete flow from pod creation to insight
Test Steps:
bash# 1. Deploy privileged pod
POD_CREATE_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
kubectl run privileged-test --image=nginx \
  --overrides='{"spec":{"containers":[{"name":"nginx","image":"nginx","securityContext":{"privileged":true}}]}}' \
  -n test-workloads

# 2. Verify agent detects (check logs)
sleep 10
kubectl logs -n ksam-test -l app=ksam-agent --since=1m | grep "privileged-test"

# Expected: "Collected Pod: privileged-test"

# 3. Verify core processes (check logs)
sleep 10
kubectl logs -n ksam-test -l app=ksam-core --since=1m | grep "privileged-test"

# Expected: 
# - "Normalized Pod: privileged-test"
# - "Correlated Pod: privileged-test"
# - "Evaluated rules for Pod: privileged-test"

# 4. Verify insight created
sleep 30
INSIGHT=$(curl -s -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/insights?severity=critical" \
  | jq '.data[] | select(.title | contains("Privileged"))')

echo $INSIGHT | jq '{title, severity, detected_at}'

# 5. Verify timing (should be < 60 seconds end-to-end)
INSIGHT_TIME=$(echo $INSIGHT | jq -r '.detected_at')
TIME_DIFF=$(( $(date -d "$INSIGHT_TIME" +%s) - $(date -d "$POD_CREATE_TIME" +%s) ))

echo "Time to detect: ${TIME_DIFF}s"

# Expected: < 60 seconds

# 6. Delete pod and verify cleanup
kubectl delete pod privileged-test -n test-workloads
sleep 30

kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT name, deleted_at
FROM pods
WHERE name = 'privileged-test';
EOF

# Expected: deleted_at populated
Expected Results:

✅ Pod collected within 30s
✅ Data normalized correctly
✅ Relationships correlated
✅ Risk evaluated
✅ Insight created within 60s
✅ Deletion tracked

Pass Criteria: Complete lifecycle < 60 seconds

TC-I002: Multi-Resource Correlation
Objective: Verify complex resource relationships are correlated
Test Steps:
bash# 1. Deploy complete stack
kubectl apply -f - <<EOF
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: app-sa
  namespace: test-workloads
---
apiVersion: v1
kind: Secret
metadata:
  name: app-secret
  namespace: test-workloads
type: Opaque
data:
  api-key: YXBpLWtleQ==
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: app-role
  namespace: test-workloads
rules:
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: app-binding
  namespace: test-workloads
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: app-role
subjects:
- kind: ServiceAccount
  name: app-sa
---
apiVersion: v1
kind: Pod
metadata:
  name: app-pod
  namespace: test-workloads
spec:
  serviceAccountName: app-sa
  containers:
  - name: app
    image: nginx
    volumeMounts:
    - name: secret-vol
      mountPath: /etc/secrets
  volumes:
  - name: secret-vol
    secret:
      secretName: app-secret
EOF

# 2. Wait for full correlation
sleep 60

# 3. Verify complete graph
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
-- Check Pod → SA
SELECT p.name as pod, sa.name as serviceaccount
FROM pods p
JOIN service_accounts sa ON p.service_account_id = sa.id
WHERE p.name = 'app-pod';

-- Check Pod → Secret
SELECT p.name as pod, s.name as secret
FROM pods p
JOIN pod_volumes pv ON p.id = pv.pod_id
JOIN secrets s ON pv.secret_id = s.id
WHERE p.name = 'app-pod';

-- Check SA → Role
SELECT sa.name as sa, r.name as role
FROM service_accounts sa
JOIN role_bindings rb ON sa.id = rb.subject_id
JOIN roles r ON rb.role_id = r.id
WHERE sa.name = 'app-sa';
EOF

# Expected: All relationships established

# 4. Test graph API
curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/graph/neighborhood/$(get_sa_id app-sa)?depth=2" \
  | jq '.neighbors[] | {type, name}'

# Expected: Pod, Role, Secret in results
Expected Results:

✅ All 5 resources collected
✅ 4 relationships established
✅ Graph queryable via API
✅ No orphaned resources

Pass Criteria: Complete resource graph constructed

TC-I003: Concurrent Agent Load
Objective: Test system under concurrent agent connections
Test Steps:
bash# 1. Scale test cluster to 10 nodes
minikube node add --count=9

# 2. Verify 10 agents running
kubectl get pods -n ksam-test -l app=ksam-agent -o wide

# Expected: 10 agent pods (one per node)

# 3. Deploy workloads on all nodes
for i in {1..100}; do
  kubectl run test-pod-$i --image=nginx -n test-workloads &
done
wait

# 4. Monitor core processing
kubectl logs -n ksam-test -l app=ksam-core -f &
LOGS_PID=$!

# 5. Wait for all pods collected
sleep 120

# 6. Verify collection
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT COUNT(*) as pod_count
FROM pods
WHERE namespace = 'test-workloads'
  AND name LIKE 'test-pod-%';
EOF

# Expected: 100 pods

# 7. Check core performance
kubectl top pod -n ksam-test -l app=ksam-core

# Expected: CPU < 1000m, Memory < 1Gi

# 8. Check for errors
kill $LOGS_PID
kubectl logs -n ksam-test -l app=ksam-core --tail=500 | grep ERROR

# Expected: No ERROR logs

# Cleanup
kubectl delete pods -n test-workloads -l run=test-pod-1 --all
minikube node delete node2 node3 ... node10
Expected Results:

✅ All 10 agents connect successfully
✅ 100 pods collected within 2 minutes
✅ Core handles load without errors
✅ Resource usage within limits
✅ No data loss

Pass Criteria: System stable under 10 agents, 100 pods

TC-I004: Database Performance
Objective: Verify database queries perform within SLA
Test Steps:
bash# 1. Seed database with large dataset
kubectl exec -it ksam-core-xxxxx -n ksam-test -- \
  /app/seed --large --pods=1000 --service-accounts=200

# 2. Test query performance
cat > test-queries.sql <<EOF
-- Query 1: Get high-risk SAs
\timing on
SELECT id, name, namespace, risk_score
FROM service_accounts
WHERE risk_score >= 7.0
ORDER BY risk_score DESC
LIMIT 20;

-- Query 2: Get critical insights
SELECT id, title, severity, detected_at
FROM insights
WHERE severity = 'critical'
  AND status = 'active'
ORDER BY detected_at DESC
LIMIT 50;

-- Query 3: Get pods with ServiceAccount
SELECT p.name, sa.name, p.namespace
FROM pods p
JOIN service_accounts sa ON p.service_account_id = sa.id
WHERE sa.risk_score > 5.0
LIMIT 100;

-- Query 4: Complex aggregation
SELECT 
  namespace,
  COUNT(DISTINCT id) as pod_count,
  AVG(risk_score) as avg_risk
FROM pods
GROUP BY namespace
ORDER BY avg_risk DESC;
EOF

kubectl exec -it postgres-0 -n ksam-test -- \
  psql -U ksam -d ksam -f /tmp/test-queries.sql

# Expected timing:
# Query 1: < 10ms
# Query 2: < 20ms
# Query 3: < 50ms
# Query 4: < 100ms
Expected Results:

✅ Simple queries < 10ms
✅ Moderate queries < 50ms
✅ Complex queries < 100ms
✅ Indexes utilized (check EXPLAIN)

Pass Criteria: All queries within SLA

Performance Test Cases
TC-P001: Agent Resource Usage
Objective: Verify agent stays within resource limits
Test Steps:
bash# 1. Deploy agent on test node
kubectl label node minikube-m02 test-node=true
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: ksam-agent-test
  namespace: ksam-test
spec:
  selector:
    matchLabels:
      app: ksam-agent-test
  template:
    metadata:
      labels:
        app: ksam-agent-test
    spec:
      nodeSelector:
        test-node: "true"
      # ... (same as production agent)
EOF

# 2. Monitor for 10 minutes
for i in {1..60}; do
  kubectl top pod -n ksam-test -l app=ksam-agent-test
  sleep 10
done > agent-metrics.txt

# 3. Analyze metrics
cat agent-metrics.txt | awk '{
  if (substr($3, 1, length($3)-2) > max_cpu) max_cpu = substr($3, 1, length($3)-2);
  if (substr($4, 1, length($4)-2) > max_mem) max_mem = substr($4, 1, length($4)-2);
}
END {
  print "Max CPU: " max_cpu "m";
  print "Max Memory: " max_mem "Mi";
}'
Expected Results:

Max CPU: < 200m (target: < 100m)
Max Memory: < 128Mi (target: < 64Mi)
No OOMKilled events
Stable over time

Pass Criteria:

✅ CPU < 200m (95th percentile)
✅ Memory < 128Mi (95th percentile)


TC-P002: Core API Latency
Objective: Measure API response times under load
Test Steps:
bash# 1. Install K6
brew install k6  # or download from k6.io

# 2. Create load test script
cat > api-load-test.js <<EOF
import http from 'k6/http';
import { check, sleep } from 'k6';

export let options = {
  stages: [
    { duration: '2m', target: 50 },  // Ramp up
    { duration: '5m', target: 50 },  // Stay at 50 VUs
    { duration: '2m', target: 100 }, // Ramp to 100 VUs
    { duration: '5m', target: 100 }, // Stay at 100 VUs
    { duration: '2m', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<100'], // 95% < 100ms
    http_req_failed: ['rate<0.01'],   // Error rate < 1%
  },
};

const BASE_URL = 'http://ksam-core:8080/api/v1';
const TOKEN = __ENV.API_TOKEN;

export default function () {
  const headers = { Authorization: \`Bearer \${TOKEN}\` };
  
  // Test GET /insights
  let res = http.get(\`\${BASE_URL}/insights\`, { headers });
  check(res, { 'status 200': (r) => r.status === 200 });
  
  // Test GET /serviceaccounts
  res = http.get(\`\${BASE_URL}/serviceaccounts\`, { headers });
  check(res, { 'status 200': (r) => r.status === 200 });
  
  sleep(1);
}
EOF

# 3. Run load test
k6 run api-load-test.js

# Expected output:
# http_req_duration..........: avg=45ms  min=10ms med=40ms max=150ms p(95)=85ms
# http_req_failed............: 0.05% ✓ 25 ✗ 48975
Expected Results:

✅ p95 latency < 100ms
✅ p99 latency < 200ms
✅ Error rate < 1%
✅ Throughput > 500 req/s

Pass Criteria: All thresholds met

TC-P003: Event Processing Throughput
Objective: Measure events processed per second
Test Steps:
bash# 1. Generate high event load
cat > event-generator.sh <<'EOF'
#!/bin/bash
for i in {1..1000}; do
  kubectl run event-test-$i --image=nginx -n test-workloads &
  if [ $((i % 100)) -eq 0 ]; then
    wait
  fi
done
wait
EOF

chmod +x event-generator.sh

# 2. Monitor core metrics
kubectl port-forward -n ksam-test svc/ksam-core 8080:8080 &
PF_PID=$!

# 3. Start monitoring
START_TIME=$(date +%s)
START_COUNT=$(curl -s http://localhost:8080/metrics | grep ksam_events_processed_total | awk '{print $2}')

# 4. Run generator
./event-generator.sh

# 5. Wait for processing
sleep 120

# 6. Check throughput
END_TIME=$(date +%s)
END_COUNT=$(curl -s http://localhost:8080/metrics | grep ksam_events_processed_total | awk '{print $2}')

DURATION=$((END_TIME - START_TIME))
EVENTS=$((END_COUNT - START_COUNT))
THROUGHPUT=$((EVENTS / DURATION))

echo "Events processed: $EVENTS"
echo "Duration: ${DURATION}s"
echo "Throughput: ${THROUGHPUT} events/s"

kill $PF_PID

# Cleanup
kubectl delete pods -n test-workloads -l run=event-test-1 --all
Expected Results:

✅ Throughput > 100 events/s
✅ No events dropped
✅ Processing latency < 1s (p95)

Pass Criteria: Throughput meets target

Security Test Cases
TC-S001: Authentication Bypass Attempt
Objective: Verify authentication cannot be bypassed
Test Steps:
bash# 1. Try accessing API without token
curl -i http://ksam-core:8080/api/v1/insights

# Expected: 401 Unauthorized

# 2. Try with empty token
curl -i -H "Authorization: Bearer " \
  http://ksam-core:8080/api/v1/insights

# Expected: 401 Unauthorized

# 3. Try with malformed token
curl -i -H "Authorization: Bearer not.a.valid.jwt" \
  http://ksam-core:8080/api/v1/insights

# Expected: 401 Unauthorized

# 4. Try with expired token
# (Generate token with past expiry)
EXPIRED_TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2MzAwMDAwMDB9.xxx"
curl -i -H "Authorization: Bearer $EXPIRED_TOKEN" \
  http://ksam-core:8080/api/v1/insights

# Expected: 401 Unauthorized

# 5. Try SQL injection in login
curl -X POST http://ksam-core:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin@example.com",
    "password": "password OR 1=1"
  }'

# Expected: 401 Unauthorized
Expected Results:

✅ All attempts return 401
✅ No sensitive info leaked
✅ SQL injection blocked
✅ Attempts logged

Pass Criteria: No authentication bypass possible

TC-S002: Authorization Escalation Attempt
Objective: Verify users cannot escalate privileges
Test Steps:
bash# 1. Login as viewer
VIEWER_TOKEN=$(get_viewer_token)

# 2. Try to create admin user
curl -i -X POST -H "Authorization: Bearer $VIEWER_TOKEN" \
  http://ksam-core:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{
    "username": "hacker@example.com",
    "password": "hacker123",
    "role": "admin"
  }'

# Expected: 403 Forbidden

# 3. Try to modify own role
USER_ID=$(get_viewer_user_id)
curl -i -X PATCH -H "Authorization: Bearer $VIEWER_TOKEN" \
  http://ksam-core:8080/api/v1/users/$USER_ID \
  -H "Content-Type: application/json" \
  -d '{"role": "admin"}'

# Expected: 403 Forbidden

# 4. Try to access admin endpoint
curl -i -H "Authorization: Bearer $VIEWER_TOKEN" \
  http://ksam-core:8080/api/v1/admin/settings

# Expected: 403 Forbidden
Expected Results:

✅ All escalation attempts blocked
✅ 403 Forbidden returned
✅ Attempts logged in audit log

Pass Criteria: No privilege escalation possible

TC-S003: Input Validation
Objective: Verify API validates and sanitizes input
Test Steps:
bash# 1. Test XSS in insight update
curl -X PATCH -H "Authorization: Bearer $TOKEN" \
  http://ksam-core:8080/api/v1/insights/insight-123 \
  -H "Content-Type: application/json" \
  -d '{
    "resolution_note": "<script>alert(\"XSS\")</script>"
  }'

# Check database
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -d ksam << EOF
SELECT resolution_note FROM insights WHERE id = 'insight-123';
EOF

# Expected: Script tags escaped or removed

# 2. Test SQL injection in filter
curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/insights?severity=critical';DROP TABLE insights;--"

# Expected: 400 Bad Request or safely handled

# 3. Test oversized input
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://ksam-core:8080/api/v1/comments \
  -H "Content-Type: application/json" \
  -d "{\"comment\": \"$(python3 -c 'print("A" * 100000)')\"}"

# Expected: 400 Bad Request (payload too large)

# 4. Test invalid JSON
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://ksam-core:8080/api/v1/insights \
  -H "Content-Type: application/json" \
  -d '{invalid json}'

# Expected: 400 Bad Request
Expected Results:

✅ XSS attempts sanitized
✅ SQL injection blocked
✅ Oversized inputs rejected
✅ Invalid JSON rejected

Pass Criteria: All malicious inputs safely handled

End-to-End Scenarios
E2E-001: Complete Security Incident Workflow
Scenario: Detect, investigate, and remediate a security incident
Steps:

Incident Trigger: Deploy high-risk workload

bashkubectl apply -f test/fixtures/high-risk-deployment.yaml

Detection: Wait for KSAM to detect (< 60s)

bash# Check dashboard for alert
# Or query API
curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/insights?severity=critical&status=active" \
  | jq '.data[0]'

Investigation: Use graph API to understand blast radius

bash# Get compromised resource ID
RESOURCE_ID=$(...)

# Query blast radius
curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/graph/blast-radius/$RESOURCE_ID?max_depth=5" \
  | jq

Simulation: Run attack simulation

bashcurl -X POST -H "Authorization: Bearer $TOKEN" \
  http://ksam-core:8080/api/v1/attack-simulation/simulate \
  -H "Content-Type: application/json" \
  -d "{
    \"compromised_id\": \"$RESOURCE_ID\",
    \"objective\": \"access-secrets\"
  }" | jq

Remediation: Apply policy

bash# Generate policy
curl -X POST -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/policies/generate/$RESOURCE_ID" \
  -H "Content-Type: application/json" \
  -d '{"type": "kubearmor"}' | jq

# Preview policy
POLICY_ID=$(...)
curl -X POST -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/policies/$POLICY_ID/preview" | jq

# Apply policy
curl -X POST -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/policies/$POLICY_ID/apply" \
  -H "Content-Type: application/json" \
  -d '{"dry_run": false}' | jq

Verification: Confirm remediation

bash# Check insight status
curl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-core:8080/api/v1/insights/$INSIGHT_ID" \
  | jq '.status'

# Expected: "resolved" or risk_score reduced
Expected Results:

✅ Incident detected within 60s
✅ Blast radius calculated
✅ Attack paths identified
✅ Policy generated and applied
✅ Insight resolved

Pass Criteria: Complete workflow < 5 minutes

E2E-002: Multi-Cluster Compliance Reporting
Scenario: Generate compliance report across multiple clusters
Steps:

Setup: Deploy KSAM on 2 clusters
Deploy Workloads: Mix of compliant and non-compliant resources
Wait for Analysis: 5 minutes
Generate Report:

bashcurl -H "Authorization: Bearer $TOKEN" \
  "http://ksam-hub:8080/api/v1/compliance/report?framework=cis-1.8&format=pdf" \
  -o compliance-report.pdf

Verify Report:


Contains data from both clusters
CIS sections 5.1-5.4 covered
Pass/fail status for each check
Remediation steps included

Expected Results:

✅ Multi-cluster data aggregated
✅ Report generated (PDF)
✅ Compliance scores accurate
✅ Actionable recommendations

Pass Criteria: Comprehensive compliance report

Regression Test Suite
Automated Regression Tests
bash#!/bin/bash
# regression-suite.sh

echo "=== KSAM Regression Test Suite ==="

# Array to track results
PASSED=0
FAILED=0

run_test() {
  TEST_NAME=$1
  TEST_SCRIPT=$2
  
  echo "Running: $TEST_NAME"
  if bash $TEST_SCRIPT; then
    echo "✅ PASSED: $TEST_NAME"
    ((PASSED++))
  else
    echo "❌ FAILED: $TEST_NAME"
    ((FAILED++))
  fi
  echo ""
}

# Phase 1 Tests
run_test "TC-P1-001: Database Schema" "tests/tc-p1-001.sh"
run_test "TC-P1-002: Component Build" "tests/tc-p1-002.sh"
run_test "TC-P1-003: Deployment" "tests/tc-p1-003.sh"
run_test "TC-P1-004: DB Connectivity" "tests/tc-p1-004.sh"

# Phase 2 Tests
run_test "TC-P2-001: Agent - Pods" "tests/tc-p2-001.sh"
run_test "TC-P2-002: Agent - SAs" "tests/tc-p2-002.sh"
run_test "TC-P2-003: Agent - RBAC" "tests/tc-p2-003.sh"
run_test "TC-P2-004: Normalization" "tests/tc-p2-004.sh"
run_test "TC-P2-005: Correlation" "tests/tc-p2-005.sh"
run_test "TC-P2-006: CIS 5.1.3" "tests/tc-p2-006.sh"
run_test "TC-P2-007: CIS 5.2.1" "tests/tc-p2-007.sh"
run_test "TC-P2-008: CIS 5.3.1" "tests/tc-p2-008.sh"
run_test "TC-P2-009: Authentication" "tests/tc-p2-009.sh"
run_test "TC-P2-010: Authorization" "tests/tc-p2-010.sh"
run_test "TC-P2-011: Pagination" "tests/tc-p2-011.sh"
run_test "TC-P2-012: Filtering" "tests/tc-p2-012.sh"

# Functional Tests
run_test "TC-F001: Risk Scoring" "tests/tc-f001.sh"
run_test "TC-F002: Orphan Detection" "tests/tc-f002.sh"
run_test "TC-F003: Pod Security" "tests/tc-f003.sh"

# Integration Tests
run_test "TC-I001: Pod Lifecycle" "tests/tc-i001.sh"
run_test "TC-I002: Correlation" "tests/tc-i002.sh"

# Security Tests
run_test "TC-S001: Auth Bypass" "tests/tc-s001.sh"
run_test "TC-S002: Authz Escalation" "tests/tc-s002.sh"
run_test "TC-S003: Input Validation" "tests/tc-s003.sh"

# Summary
echo "==================================="
echo "Test Results:"
echo "  PASSED: $PASSED"
echo "  FAILED: $FAILED"
echo "  TOTAL:  $((PASSED + FAILED))"
echo "==================================="

if [ $FAILED -eq 0 ]; then
  echo "✅ All tests passed!"
  exit 0
else
  echo "❌ $FAILED test(s) failed"
  exit 1
fi

Test Automation
CI/CD Integration (GitHub Actions)
yaml# .github/workflows/test.yml
name: KSAM Test Suite

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run Unit Tests
        run: |
          cd agent && go test ./... -v -cover
          cd ../core && go test ./... -v -cover
      
      - name: Upload Coverage
        uses: codecov/codecov-action@v3

  integration-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Kubernetes (kind)
        uses: helm/kind-action@v1
      
      - name: Deploy KSAM
        run: |
          kubectl create namespace ksam-test
          helm install ksam-test ./deploy/helm/ksam \
            -f ./test/fixtures/values-test.yaml \
            -n ksam-test
      
      - name: Run Integration Tests
        run: ./test/regression-suite.sh

  e2e-tests:
    runs-on: ubuntu-latest
    if: github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup Kubernetes
        uses: helm/kind-action@v1
      
      - name: Deploy KSAM
        run: |
          # Full deployment
          ./scripts/deploy-test-env.sh
      
      - name: Run E2E Tests
        run: ./test/e2e-suite.sh
      
      - name: Collect Logs
        if: failure()
        run: |
          kubectl logs -n ksam-test --all-containers=true > logs.txt
      
      - name: Upload Logs
        if: failure()
        uses: actions/upload-artifact@v3
        with:
          name: test-logs
          path: logs.txt

Test Data & Fixtures
Test Data Generator
bash# generate-test-data.sh

#!/bin/bash

NAMESPACE="test-workloads"

echo "Generating test data in namespace: $NAMESPACE"

# 1. Create namespace
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

# 2. ServiceAccounts (various risk levels)
for RISK in low medium high critical; do
  kubectl create sa ${RISK}-risk-sa -n $NAMESPACE
done

# 3. Pods (various security postures)
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: secure-pod
  namespace: $NAMESPACE
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 1000
    fsGroup: 2000
    seccompProfile:
      type: RuntimeDefault
  containers:
  - name: app
    image: nginx:latest
    securityContext:
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities:
        drop: ["ALL"]
    resources:
      requests:
        memory: "64Mi"
        cpu: "250m"
      limits:
        memory: "128Mi"
        cpu: "500m"
---
apiVersion: v1
kind: Pod
metadata:
  name: insecure-pod
  namespace: $NAMESPACE
spec:
  hostPID: true
  hostNetwork: true
  containers:
  - name: app
    image: nginx:latest
    securityContext:
      privileged: true
      runAsUser: 0
EOF

# 4. RBAC (various permission levels)
kubectl create role viewer --verb=get,list --resource=pods -n $NAMESPACE
kubectl create role editor --verb=get,list,create,update,patch --resource=pods,configmaps -n $NAMESPACE
kubectl create clusterrole dangerous --verb="*" --resource="*"

# 5. Secrets
kubectl create secret generic api-key --from-literal=key=secret123 -n $NAMESPACE
kubectl create secret generic db-creds --from-literal=user=admin --from-literal=password=password123 -n $NAMESPACE

echo "Test data generated successfully"

Bug Reporting
Bug Report Template
markdown## Bug Report

**Bug ID**: BUG-XXX
**Severity**: Critical / High / Medium / Low
**Status**: Open / In Progress / Resolved / Closed

### Environment
- KSAM Version: v1.0.0
- Kubernetes Version: 1.28.0
- OS: Ubuntu 22.04
- Cluster Size: 3 nodes

### Description
[Clear description of the bug]

### Steps to Reproduce
1. Deploy KSAM with default config
2. Create privileged pod
3. Wait 60 seconds
4. Check insights API

### Expected Behavior
Insight should be created with rule_id=cis-5.2.1

### Actual Behavior
No insight created, error in core logs:
ERROR: failed to evaluate rule cis-5.2.1: nil pointer dereference

### Logs
[Attach relevant logs]

### Screenshots
[If applicable]

### Additional Context
Reproducible 100% of the time on fresh deployment

Appendix
A. Test Coverage Matrix
ComponentUnitIntegrationE2ESecurityPerformanceAgent✅ 85%✅ 80%✅✅✅Core✅ 82%✅ 75%✅✅✅Risk Engine✅ 90%✅ 85%✅✅-API✅ 88%✅ 90%✅✅✅Dashboard✅ 75%✅ 70%✅✅-
B. Test Environment Specs
yamlminikube:
  version: v1.32.0
  cpus: 4
  memory: 8192
  kubernetes: v1.28.0
  driver: docker

ksam-test:
  namespace: ksam-test
  agent:
    replicas: 3 (one per node)
  core:
    replicas: 2
  postgres:
    storage: 20Gi
  nats:
    replicas: 3
C. Useful Testing Commands
bash# Quick smoke test
kubectl exec -it ksam-core-xxxxx -n ksam-test -- /app/healthcheck

# Check agent connectivity
kubectl logs -n ksam-test -l app=ksam-agent | grep "Connected to core"

# Trigger immediate collection
kubectl annotate pod test-pod -n test-workloads ksam.io/force-collect=true

# Generate test traffic
kubectl run curl --image=curlimages/curl -it --rm -- sh

# Reset test database
kubectl exec -it postgres-0 -n ksam-test -- psql -U ksam -c "TRUNCATE TABLE insights CASCADE;"

End of Testing Strategy
Document Status: Ready for Execution
Next Review: After MVP-1 Complete