# E2E Complete – Monitor Agent/Core & API kết quả thực tế

**Thời gian:** 2026-02-22T14:06:19+00:00
**Namespace:** fortuna

---
## 1. Test scripts đã chạy

| Script | Mô tả |
|--------|--------|
| run-e2e-full.sh | E2E full (cluster, API, DB, dashboard) |
| test-priority1-apis.sh | Promotion rules, runtime-signals API |
| test-runtime-signals-e2e.sh | POST runtime-events, DB, GET runtime-signals |
| e2e-dashboard-data.sh | Dashboard chart data (Threat Velocity, PCE Trend) |

---
## 2. Kết quả API thực tế (clusters, dashboard/stats, agents/status, insights/summary)

```json
[0;32m[OK][0m JWT obtained

[0;34m[E2E][0m Step 1: Creating privileged pod e2e-dashboard-pod-1771769061...
pod/e2e-dashboard-pod-1771769061 created
pod/e2e-dashboard-pod-1771769061 condition met
[0;32m[OK][0m Pod UID: 7c4681f9-7498-43e8-9688-e092b3b7ded4

[0;34m[E2E][0m Step 2: Waiting for pod in Core /pods (agent sync, up to 90s)...
[1;33m[WARN][0m Pod not seen in Core /pods after 90s; PCE may not have run yet

[0;34m[E2E][0m Step 3: Waiting for PCE evaluation (15s)...

[0;34m[E2E][0m Step 4: POST runtime-events (PROC_ROOT_PIVOT, FS_ESCAPE_ATTEMPT)...
[0;32m[OK][0m Runtime events processed: 2

[0;34m[E2E][0m Step 5: Triggering insights evaluate (historical) for CVE insights...
[0;32m[OK][0m Insights evaluate triggered (async)

[0;34m[E2E][0m Step 6: Verifying dashboard APIs (threat-velocity, pod-capabilities/trends)...
[0;32m[OK][0m Threat Velocity: 7 points, total risks in window: 0
[0;32m[OK][0m PCE Trend: 7 points, total capabilities in window: 64

[0;34m[E2E][0m Step 7: Cleaning up test pod...
pod "e2e-dashboard-pod-1771769061" deleted
[0;32m[OK][0m Test pod delete requested

==========================================
[0;32m[OK][0m E2E dashboard data run finished.
==========================================
Dashboard charts use: threat-velocity (insights), pod-capabilities/trends (PCE), runtime-signals (Risk Center).
Refresh dashboard (admin/admin123) to see data.


=== GET /api/v1/clusters ===
{
    "clusters": [
        {
            "id": "sha256-c93f5c6cb0e57f8f",
            "name": "cluster-c93f5c6c",
            "source": "auto",
            "k8sVersion": "v1.29.15",
            "distribution": "kubeadm",
            "region": "",
            "endpoint": "",
            "status": "active",
            "lastSync": "2026-02-22T14:02:40.768254Z",
            "createdAt": "2026-02-22T13:47:35.516287Z",
            "updatedAt": "2026-02-22T14:02:40.768254Z"
        }
    ]
}

=== GET /api/v1/dashboard/stats ===
{
    "totalClusters": 1,
    "activeAgents": 2,
    "runningPods": 20,
    "totalRisks": 0,
    "criticalRisks": 0,
    "resolved24h": 0,
    "affectedPodCount": 0
}

=== GET /api/v1/agents/status ===
{
    "agents": [
        {
            "agentId": "k8s-master-agent",
            "clusterId": "sha256-c93f5c6cb0e57f8f",
            "clusterName": "cluster-c93f5c6c",
            "lastHeartbeat": "2026-02-22T13:47:37.756995Z",
            "nodeName": "k8s-master",
            "status": "slow",
            "version": "dev"
        },
        {
            "agentId": "k8s-worker01-agent",
            "clusterId": "sha256-c93f5c6cb0e57f8f",
            "clusterName": "cluster-c93f5c6c",
            "lastHeartbeat": "2026-02-22T13:47:35.496568Z",
            "nodeName": "k8s-worker01",
            "status": "slow",
            "version": "dev"
        }
    ],
    "disconnected": 0,
    "healthy": 0,
    "slow": 2,
    "total": 2
}

=== GET /api/v1/insights/summary ===
{
    "total": 0,
    "critical": 0,
    "high": 0,
    "medium": 0,
    "low": 0,
    "byType": {}
}
```

---
## 3. Agent/Core logs (trước khi chạy E2E)

```
=== Core (last 40 lines) ===
[0m[33m[2.954ms] [34;1m[rows:1][0m INSERT INTO "pod_capabilities" ("pod_uid","namespace","capability_id","capability_group","severity","state","confidence","first_seen_at","last_seen_at","evidence","mitre","created_at","updated_at") VALUES ('926489f3-dbba-46aa-97be-3c482d4fb0ea','fortuna','ESC_HOSTPATH_NODE','ESC','CRITICAL','detected',0.8,'2026-02-22 14:02:42.571','2026-02-22 14:02:42.571','{"hostPath":true}','{}','2026-02-22 14:02:42.571','2026-02-22 14:02:42.571') ON CONFLICT ("pod_uid","capability_id") DO UPDATE SET "capability_group"="excluded"."capability_group","severity"="excluded"."severity","evidence"="excluded"."evidence","updated_at"="excluded"."updated_at" RETURNING "id"

2026/02/22 14:02:42 [32mgithub.com/fortuna/core/pkg/capability/state_controller.go:183
[0m[33m[0.838ms] [34;1m[rows:1][0m SELECT * FROM "capability_metadata" WHERE capability_id = 'ESC_RUNTIME_PROBE' ORDER BY "capability_metadata"."capability_id" LIMIT 1

2026/02/22 14:02:42 [32mgithub.com/fortuna/core/pkg/capability/state_controller.go:215
[0m[33m[2.858ms] [34;1m[rows:1][0m INSERT INTO "pod_capabilities" ("pod_uid","namespace","capability_id","capability_group","severity","state","confidence","first_seen_at","last_seen_at","evidence","mitre","created_at","updated_at") VALUES ('926489f3-dbba-46aa-97be-3c482d4fb0ea','fortuna','ESC_RUNTIME_PROBE','ESC','HIGH','detected',0.5,'2026-02-22 14:02:42.575','2026-02-22 14:02:42.575','{"hostIPC":false,"hostPID":false,"hostPathMatches":[{"hostPath":"/run/containerd/containerd.sock","mountPath":"/run/containerd/containerd.sock","readOnly":true,"volume":"containerd-socket"}]}','{}','2026-02-22 14:02:42.575','2026-02-22 14:02:42.575') ON CONFLICT ("pod_uid","capability_id") DO UPDATE SET "capability_group"="excluded"."capability_group","severity"="excluded"."severity","evidence"="excluded"."evidence","updated_at"="excluded"."updated_at" RETURNING "id"

2026/02/22 14:02:42 [32mgithub.com/fortuna/core/pkg/capability/state_controller.go:183
[0m[33m[0.865ms] [34;1m[rows:1][0m SELECT * FROM "capability_metadata" WHERE capability_id = 'ID_TOKEN_POD' ORDER BY "capability_metadata"."capability_id" LIMIT 1

2026/02/22 14:02:42 [32mgithub.com/fortuna/core/pkg/capability/state_controller.go:215
[0m[33m[2.605ms] [34;1m[rows:1][0m INSERT INTO "pod_capabilities" ("pod_uid","namespace","capability_id","capability_group","severity","state","confidence","first_seen_at","last_seen_at","evidence","mitre","created_at","updated_at") VALUES ('926489f3-dbba-46aa-97be-3c482d4fb0ea','fortuna','ID_TOKEN_POD','ID','MEDIUM','detected',0.5,'2026-02-22 14:02:42.579','2026-02-22 14:02:42.579','{"automountServiceAccountToken":true}','{}','2026-02-22 14:02:42.579','2026-02-22 14:02:42.579') ON CONFLICT ("pod_uid","capability_id") DO UPDATE SET "capability_group"="excluded"."capability_group","severity"="excluded"."severity","evidence"="excluded"."evidence","updated_at"="excluded"."updated_at" RETURNING "id"
[GIN] 2026/02/22 - 14:02:46 | 200 |    2.932821ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:02:51 | 200 |      86.331µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:02:51 | 200 |    2.486965ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:02:56 | 200 |    2.529376ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:03:01 | 200 |      112.08µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:03:01 | 200 |    3.483001ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:03:06 | 200 |    3.026185ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:03:11 | 200 |     241.512µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:03:11 | 200 |    2.677802ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:03:16 | 200 |    2.345091ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:03:21 | 200 |      98.505µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:03:21 | 200 |     2.53259ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:03:26 | 200 |    2.568648ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:03:31 | 200 |      85.419µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:03:31 | 200 |     3.50949ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:03:36 | 200 |    2.487647ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:03:41 | 200 |      85.651µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:03:41 | 200 |    2.720363ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:03:46 | 200 |    3.640224ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:03:51 | 200 |      58.089µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:03:51 | 200 |    3.294997ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:03:56 | 200 |    2.914846ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:04:01 | 200 |     278.301µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:04:01 | 200 |    2.625694ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:04:06 | 200 |    2.194688ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:04:11 | 200 |      96.952µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:04:11 | 200 |     2.57542ms |      10.244.0.1 | GET      "/ready"

=== Agent (last 50 lines, sync/heartbeat) ===
[SBOMExtractor] 2026/02/22 13:48:11 ⚠️  Containerd fetch failed: containerd export error: content digest sha256:4397c18a33dce22f6f6cd4cb37836285a4f81d74c033054f6cdcb41c5dc4181a: not found
2026/02/22 13:52:42.269375 [Syncer] ✅ Full sync completed
2026/02/22 13:52:52.804564 ✅ Heartbeat OK (interval=15s)
2026/02/22 13:57:42.319883 [Syncer] ✅ Full sync completed
2026/02/22 13:57:52.800939 ✅ Heartbeat OK (interval=15s)
2026/02/22 14:02:42.268701 [Syncer] ✅ Full sync completed
2026/02/22 14:02:52.802594 ✅ Heartbeat OK (interval=15s)
[SBOMExtractor] 2026/02/22 13:48:11 ⚠️  Containerd fetch failed: containerd export error: content digest sha256:7e55d1d0d2c095cecd55022878b8e64d88176157c6ed49a89e2ced129d26465e: not found
2026/02/22 13:52:39.423664 [Syncer] ✅ Full sync completed
2026/02/22 13:52:50.529450 ✅ Heartbeat OK (interval=15s)
2026/02/22 13:57:39.321707 [Syncer] ✅ Full sync completed
2026/02/22 13:57:50.531905 ✅ Heartbeat OK (interval=15s)
2026/02/22 14:02:39.383766 [Syncer] ✅ Full sync completed
2026/02/22 14:02:50.528337 ✅ Heartbeat OK (interval=15s)
```

---
## 4. Agent/Core logs (sau khi chạy E2E)

```
=== Core (last 50 lines) ===
ERROR: <input>:1:19: Syntax error: mismatched input 'contains' expecting ']'
 | rules[].resources contains 'secrets' || rules[].resources contains 'configmaps' || rules[].resources contains 'pods'
 | ..................^
ERROR: <input>:1:47: Syntax error: extraneous input ']' expecting {'[', '{', '(', '.', '-', '!', '?', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
 | rules[].resources contains 'secrets' || rules[].resources contains 'configmaps' || rules[].resources contains 'pods'
 | ..............................................^
ERROR: <input>:1:59: Syntax error: mismatched input 'contains' expecting ']'
 | rules[].resources contains 'secrets' || rules[].resources contains 'configmaps' || rules[].resources contains 'pods'
 | ..........................................................^
ERROR: <input>:1:90: Syntax error: extraneous input ']' expecting {'[', '{', '(', '.', '-', '!', '?', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
 | rules[].resources contains 'secrets' || rules[].resources contains 'configmaps' || rules[].resources contains 'pods'
 | .........................................................................................^
ERROR: <input>:1:102: Syntax error: mismatched input 'contains' expecting ']'
 | rules[].resources contains 'secrets' || rules[].resources contains 'configmaps' || rules[].resources contains 'pods'
 | .....................................................................................................^
2026/02/22 14:06:18.167093 [YAMLEngine] WARNING: Failed to compile CEL for rule overprivileged-binding, condition 0: CEL compilation error: ERROR: <input>:1:1: undeclared reference to 'roleRef' (in container '')
 | roleRef.kind == 'Role' || roleRef.kind == 'ClusterRole'
 | ^
ERROR: <input>:1:27: undeclared reference to 'roleRef' (in container '')
 | roleRef.kind == 'Role' || roleRef.kind == 'ClusterRole'
 | ..........................^
2026/02/22 14:06:18.167096 [YAMLEngine] Compiled 1 CEL expressions
2026/02/22 14:06:18.167098 [RiskEngine] ✅ Using YAML engine with 7 rules (YAML + hardcoded fallback)
2026/02/22 14:06:18.167100 [InsightStatusUpdater] Starting status update for resolved risks...

2026/02/22 14:06:18 [32mgithub.com/fortuna/core/pkg/worker/insight_status_updater.go:41
[0m[33m[2.133ms] [34;1m[rows:0][0m SELECT * FROM "insights" WHERE (status = 'active' OR status IS NULL) AND "insights"."deleted_at" IS NULL
2026/02/22 14:06:18.169308 [InsightStatusUpdater] Found 0 active insights to check
2026/02/22 14:06:18.169314 [InsightStatusUpdater] Status update completed in 2.21269ms: 0 insights auto-resolved
[GIN] 2026/02/22 - 14:06:18 | 200 |   56.260419ms |       127.0.0.1 | POST     "/api/v1/insights/evaluate/historical"

2026/02/22 14:06:18 [32mgithub.com/fortuna/core/internal/middleware/auth.go:45
[0m[33m[1.111ms] [34;1m[rows:1][0m SELECT * FROM "users" WHERE "users"."id" = 1 AND "users"."deleted_at" IS NULL ORDER BY "users"."id" LIMIT 1

2026/02/22 14:06:18 [32mgithub.com/fortuna/core/internal/api/dashboard_handlers.go:224
[0m[33m[2.186ms] [34;1m[rows:0][0m SELECT date_trunc('day', detected_at) as date, LOWER(severity) as severity, COUNT(*) as count FROM "insights" WHERE (insight_type = 'vulnerability' AND detected_at >= '2026-02-16 00:00:00' AND deleted_at IS NULL) AND "insights"."deleted_at" IS NULL GROUP BY date_trunc('day', detected_at), LOWER(severity) ORDER BY date_trunc('day', detected_at)
[GIN] 2026/02/22 - 14:06:18 | 200 |    3.730331ms |       127.0.0.1 | GET      "/api/v1/dashboard/metrics/threat-velocity?days=7"

2026/02/22 14:06:18 [32mgithub.com/fortuna/core/internal/middleware/auth.go:45
[0m[33m[1.809ms] [34;1m[rows:1][0m SELECT * FROM "users" WHERE "users"."id" = 1 AND "users"."deleted_at" IS NULL ORDER BY "users"."id" LIMIT 1

2026/02/22 14:06:18 [32mgithub.com/fortuna/core/internal/api/pod_capability_handlers.go:355
[0m[33m[5.788ms] [34;1m[rows:1][0m SELECT 
				TO_CHAR(pc.created_at::date, 'YYYY-MM-DD') AS date,
				SUM(CASE WHEN LOWER(pc.severity) = 'critical' THEN 1 ELSE 0 END) AS critical,
				SUM(CASE WHEN LOWER(pc.severity) = 'high' THEN 1 ELSE 0 END) AS high,
				SUM(CASE WHEN LOWER(pc.severity) = 'medium' THEN 1 ELSE 0 END) AS medium,
				SUM(CASE WHEN LOWER(pc.severity) = 'low' THEN 1 ELSE 0 END) AS low
			 FROM pod_capabilities AS pc WHERE pc.created_at >= NOW() - (7 * INTERVAL '1 day') GROUP BY pc.created_at::date ORDER BY pc.created_at::date
[GIN] 2026/02/22 - 14:06:18 | 200 |    8.229562ms |       127.0.0.1 | GET      "/api/v1/pod-capabilities/trends?days=7"

=== Agent (last 60 lines, sync/heartbeat) ===
[SBOMExtractor] 2026/02/22 13:48:11 ⚠️  Containerd fetch failed: containerd export error: content digest sha256:4397c18a33dce22f6f6cd4cb37836285a4f81d74c033054f6cdcb41c5dc4181a: not found
2026/02/22 13:52:42.269375 [Syncer] ✅ Full sync completed
2026/02/22 13:52:52.804564 ✅ Heartbeat OK (interval=15s)
2026/02/22 13:57:42.319883 [Syncer] ✅ Full sync completed
2026/02/22 13:57:52.800939 ✅ Heartbeat OK (interval=15s)
2026/02/22 14:02:42.268701 [Syncer] ✅ Full sync completed
2026/02/22 14:02:52.802594 ✅ Heartbeat OK (interval=15s)
2026/02/22 13:52:39.423664 [Syncer] ✅ Full sync completed
2026/02/22 13:52:50.529450 ✅ Heartbeat OK (interval=15s)
2026/02/22 13:57:39.321707 [Syncer] ✅ Full sync completed
2026/02/22 13:57:50.531905 ✅ Heartbeat OK (interval=15s)
2026/02/22 14:02:39.383766 [Syncer] ✅ Full sync completed
2026/02/22 14:02:50.528337 ✅ Heartbeat OK (interval=15s)
[LocalPodWatcher] 2026/02/22 14:04:29    → Queued pod fortuna/e2e-dashboard-pod-1771769061 for async processing (transitioned to Running)
[SBOMExtractor] 2026/02/22 14:04:29 ⚠️  Containerd fetch failed: containerd export error: content digest sha256:fe2385f276937dcf780967a5385767fd34b34580c8ed8d303a0cd1485a692635: not found
```

---
## 5. File báo cáo chi tiết

- E2E full: `docs/test-results/E2E-FULL-*.md` (mới nhất)
- API raw: `/home/k8s/KSAM/docs/test-results/e2e-api-results-20260222-140416.txt`
- Log before: `/home/k8s/KSAM/docs/test-results/e2e-logs-before-20260222-140416.txt`
- Log after: `/home/k8s/KSAM/docs/test-results/e2e-logs-after-20260222-140416.txt`

