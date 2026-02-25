# E2E Complete – Monitor Agent/Core & API kết quả thực tế

**Thời gian:** 2026-02-22T14:12:57+00:00
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

[0;34m[E2E][0m Step 1: Creating privileged pod e2e-dashboard-pod-1771769500...
pod/e2e-dashboard-pod-1771769500 created
pod/e2e-dashboard-pod-1771769500 condition met
[0;32m[OK][0m Pod UID: d4129268-29ed-4578-9195-8b092fe727c5

[0;34m[E2E][0m Step 2: Waiting for pod in Core /pods (agent sync, up to 180s)...
[0;32m[OK][0m Pod found in Core after 50s

[0;34m[E2E][0m Step 3: Waiting for PCE evaluation (15s)...

[0;34m[E2E][0m Step 4: POST runtime-events (PROC_ROOT_PIVOT, FS_ESCAPE_ATTEMPT)...
[0;32m[OK][0m Runtime events processed: 2

[0;34m[E2E][0m Step 5: Triggering insights evaluate (historical) for CVE insights...
[0;32m[OK][0m Insights evaluate triggered (async)

[0;34m[E2E][0m Step 6: Verifying dashboard APIs (threat-velocity, pod-capabilities/trends)...
[0;32m[OK][0m Threat Velocity: 7 points, total risks in window: 0
[0;32m[OK][0m PCE Trend: 7 points, total capabilities in window: 69

[0;34m[E2E][0m Step 7: Cleaning up test pod...
pod "e2e-dashboard-pod-1771769500" deleted
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
            "lastSync": "2026-02-22T14:12:40.773343Z",
            "createdAt": "2026-02-22T13:47:35.516287Z",
            "updatedAt": "2026-02-22T14:12:40.773343Z"
        }
    ]
}

=== GET /api/v1/dashboard/stats ===
{
    "totalClusters": 1,
    "activeAgents": 2,
    "runningPods": 21,
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
[GIN] 2026/02/22 - 14:09:21 | 200 |    3.115999ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:09:26 | 200 |    2.508192ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:09:31 | 200 |      80.301µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:09:31 | 200 |    2.582742ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:09:36 | 200 |    2.184457ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:09:41 | 200 |      155.04µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:09:41 | 200 |    3.114065ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:09:46 | 200 |    3.155803ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:09:51 | 200 |      67.126µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:09:51 | 200 |    3.747311ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:09:56 | 200 |    2.903372ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:10:01 | 200 |      82.234µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:10:01 | 200 |    3.129865ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:10:06 | 200 |    2.757408ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:10:11 | 200 |     119.513µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:10:11 | 200 |    2.152116ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:10:16 | 200 |    2.760966ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:10:21 | 200 |      97.883µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:10:21 | 200 |    2.849601ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:10:26 | 200 |    2.521578ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:10:31 | 200 |       80.24µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:10:31 | 200 |    3.081084ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:10:36 | 200 |    2.960196ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:10:41 | 200 |      88.586µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:10:41 | 200 |    3.156165ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:10:46 | 200 |     2.56084ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:10:51 | 200 |      91.791µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:10:51 | 200 |    2.577372ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:10:56 | 200 |    2.193213ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:11:01 | 200 |      176.37µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:11:01 | 200 |     2.86498ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:11:06 | 200 |    2.643847ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:11:11 | 200 |      83.336µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:11:11 | 200 |    2.748572ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:11:16 | 200 |    2.796281ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:11:21 | 200 |     349.274µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:11:21 | 200 |    3.698768ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:11:26 | 200 |    3.015622ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/22 - 14:11:31 | 200 |      64.962µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/22 - 14:11:31 | 200 |    2.689201ms |      10.244.0.1 | GET      "/ready"

=== Agent (last 50 lines, sync/heartbeat) ===
[SBOMExtractor] 2026/02/22 13:48:11 ⚠️  Containerd fetch failed: containerd export error: content digest sha256:4397c18a33dce22f6f6cd4cb37836285a4f81d74c033054f6cdcb41c5dc4181a: not found
2026/02/22 13:52:42.269375 [Syncer] ✅ Full sync completed
2026/02/22 13:52:52.804564 ✅ Heartbeat OK (interval=15s)
2026/02/22 13:57:42.319883 [Syncer] ✅ Full sync completed
2026/02/22 13:57:52.800939 ✅ Heartbeat OK (interval=15s)
2026/02/22 14:02:42.268701 [Syncer] ✅ Full sync completed
2026/02/22 14:02:52.802594 ✅ Heartbeat OK (interval=15s)
2026/02/22 14:07:42.291568 [Syncer] ✅ Full sync completed
2026/02/22 14:07:52.802194 ✅ Heartbeat OK (interval=15s)
2026/02/22 13:52:39.423664 [Syncer] ✅ Full sync completed
2026/02/22 13:52:50.529450 ✅ Heartbeat OK (interval=15s)
2026/02/22 13:57:39.321707 [Syncer] ✅ Full sync completed
2026/02/22 13:57:50.531905 ✅ Heartbeat OK (interval=15s)
2026/02/22 14:02:39.383766 [Syncer] ✅ Full sync completed
2026/02/22 14:02:50.528337 ✅ Heartbeat OK (interval=15s)
[LocalPodWatcher] 2026/02/22 14:04:29    → Queued pod fortuna/e2e-dashboard-pod-1771769061 for async processing (transitioned to Running)
[SBOMExtractor] 2026/02/22 14:04:29 ⚠️  Containerd fetch failed: containerd export error: content digest sha256:fe2385f276937dcf780967a5385767fd34b34580c8ed8d303a0cd1485a692635: not found
2026/02/22 14:07:39.265174 [Syncer] ✅ Full sync completed
2026/02/22 14:07:50.528380 ✅ Heartbeat OK (interval=15s)
```

---
## 4. Agent/Core logs (sau khi chạy E2E)

```
=== Core (last 50 lines) ===
ERROR: <input>:1:90: Syntax error: extraneous input ']' expecting {'[', '{', '(', '.', '-', '!', '?', 'true', 'false', 'null', NUM_FLOAT, NUM_INT, NUM_UINT, STRING, BYTES, IDENTIFIER}
 | rules[].resources contains 'secrets' || rules[].resources contains 'configmaps' || rules[].resources contains 'pods'
 | .........................................................................................^
ERROR: <input>:1:102: Syntax error: mismatched input 'contains' expecting ']'
 | rules[].resources contains 'secrets' || rules[].resources contains 'configmaps' || rules[].resources contains 'pods'
 | .....................................................................................................^
2026/02/22 14:12:55.443973 [YAMLEngine] WARNING: Failed to compile CEL for rule overprivileged-binding, condition 0: CEL compilation error: ERROR: <input>:1:1: undeclared reference to 'roleRef' (in container '')
 | roleRef.kind == 'Role' || roleRef.kind == 'ClusterRole'
 | ^
ERROR: <input>:1:27: undeclared reference to 'roleRef' (in container '')
 | roleRef.kind == 'Role' || roleRef.kind == 'ClusterRole'
 | ..........................^
2026/02/22 14:12:55.444031 [YAMLEngine] WARNING: Failed to compile CEL for rule cluster-admin-pod, condition 0: CEL compilation error: ERROR: <input>:1:1: undeclared reference to 'resource' (in container '')
 | resource.kind == "Pod" &&
 | ^
ERROR: <input>:2:1: undeclared reference to 'resource' (in container '')
 | resource.service_account_name != "" &&
 | ^
ERROR: <input>:3:1: undeclared reference to 'resource' (in container '')
 | resource.service_account_name != "default"
 | ^
2026/02/22 14:12:55.444269 [YAMLEngine] Compiled 1 CEL expressions
2026/02/22 14:12:55.444275 [RiskEngine] ✅ Using YAML engine with 7 rules (YAML + hardcoded fallback)
2026/02/22 14:12:55.444277 [InsightStatusUpdater] Starting status update for resolved risks...

2026/02/22 14:12:55 [32mgithub.com/fortuna/core/pkg/worker/insight_status_updater.go:41
[0m[33m[0.943ms] [34;1m[rows:0][0m SELECT * FROM "insights" WHERE (status = 'active' OR status IS NULL) AND "insights"."deleted_at" IS NULL
2026/02/22 14:12:55.445342 [InsightStatusUpdater] Found 0 active insights to check
2026/02/22 14:12:55.445440 [InsightStatusUpdater] Status update completed in 1.160781ms: 0 insights auto-resolved
[GIN] 2026/02/22 - 14:12:55 | 200 |   42.657622ms |       127.0.0.1 | POST     "/api/v1/insights/evaluate/historical"

2026/02/22 14:12:55 [32mgithub.com/fortuna/core/internal/middleware/auth.go:45
[0m[33m[6.536ms] [34;1m[rows:1][0m SELECT * FROM "users" WHERE "users"."id" = 1 AND "users"."deleted_at" IS NULL ORDER BY "users"."id" LIMIT 1

2026/02/22 14:12:55 [32mgithub.com/fortuna/core/internal/api/dashboard_handlers.go:224
[0m[33m[1.086ms] [34;1m[rows:0][0m SELECT date_trunc('day', detected_at) as date, LOWER(severity) as severity, COUNT(*) as count FROM "insights" WHERE (insight_type = 'vulnerability' AND detected_at >= '2026-02-16 00:00:00' AND deleted_at IS NULL) AND "insights"."deleted_at" IS NULL GROUP BY date_trunc('day', detected_at), LOWER(severity) ORDER BY date_trunc('day', detected_at)
[GIN] 2026/02/22 - 14:12:55 | 200 |    7.868619ms |       127.0.0.1 | GET      "/api/v1/dashboard/metrics/threat-velocity?days=7"

2026/02/22 14:12:55 [32mgithub.com/fortuna/core/internal/middleware/auth.go:45
[0m[33m[1.237ms] [34;1m[rows:1][0m SELECT * FROM "users" WHERE "users"."id" = 1 AND "users"."deleted_at" IS NULL ORDER BY "users"."id" LIMIT 1

2026/02/22 14:12:55 [32mgithub.com/fortuna/core/internal/api/pod_capability_handlers.go:355
[0m[33m[1.284ms] [34;1m[rows:1][0m SELECT 
				TO_CHAR(pc.created_at::date, 'YYYY-MM-DD') AS date,
				SUM(CASE WHEN LOWER(pc.severity) = 'critical' THEN 1 ELSE 0 END) AS critical,
				SUM(CASE WHEN LOWER(pc.severity) = 'high' THEN 1 ELSE 0 END) AS high,
				SUM(CASE WHEN LOWER(pc.severity) = 'medium' THEN 1 ELSE 0 END) AS medium,
				SUM(CASE WHEN LOWER(pc.severity) = 'low' THEN 1 ELSE 0 END) AS low
			 FROM pod_capabilities AS pc WHERE pc.created_at >= NOW() - (7 * INTERVAL '1 day') GROUP BY pc.created_at::date ORDER BY pc.created_at::date
[GIN] 2026/02/22 - 14:12:55 | 200 |    2.853768ms |       127.0.0.1 | GET      "/api/v1/pod-capabilities/trends?days=7"

=== Agent (last 60 lines, sync/heartbeat) ===
[SBOMExtractor] 2026/02/22 13:48:11 ⚠️  Containerd fetch failed: containerd export error: content digest sha256:4397c18a33dce22f6f6cd4cb37836285a4f81d74c033054f6cdcb41c5dc4181a: not found
2026/02/22 13:52:42.269375 [Syncer] ✅ Full sync completed
2026/02/22 13:52:52.804564 ✅ Heartbeat OK (interval=15s)
2026/02/22 13:57:42.319883 [Syncer] ✅ Full sync completed
2026/02/22 13:57:52.800939 ✅ Heartbeat OK (interval=15s)
2026/02/22 14:02:42.268701 [Syncer] ✅ Full sync completed
2026/02/22 14:02:52.802594 ✅ Heartbeat OK (interval=15s)
2026/02/22 14:07:42.291568 [Syncer] ✅ Full sync completed
2026/02/22 14:07:52.802194 ✅ Heartbeat OK (interval=15s)
2026/02/22 14:12:42.413503 [Syncer] ✅ Full sync completed
2026/02/22 14:12:52.800795 ✅ Heartbeat OK (interval=15s)
2026/02/22 13:52:39.423664 [Syncer] ✅ Full sync completed
2026/02/22 13:52:50.529450 ✅ Heartbeat OK (interval=15s)
2026/02/22 13:57:39.321707 [Syncer] ✅ Full sync completed
2026/02/22 13:57:50.531905 ✅ Heartbeat OK (interval=15s)
2026/02/22 14:02:39.383766 [Syncer] ✅ Full sync completed
2026/02/22 14:02:50.528337 ✅ Heartbeat OK (interval=15s)
[LocalPodWatcher] 2026/02/22 14:04:29    → Queued pod fortuna/e2e-dashboard-pod-1771769061 for async processing (transitioned to Running)
[SBOMExtractor] 2026/02/22 14:04:29 ⚠️  Containerd fetch failed: containerd export error: content digest sha256:fe2385f276937dcf780967a5385767fd34b34580c8ed8d303a0cd1485a692635: not found
2026/02/22 14:07:39.265174 [Syncer] ✅ Full sync completed
2026/02/22 14:07:50.528380 ✅ Heartbeat OK (interval=15s)
[LocalPodWatcher] 2026/02/22 14:11:48    → Queued pod fortuna/e2e-dashboard-pod-1771769500 for async processing (transitioned to Running)
[SBOMExtractor] 2026/02/22 14:11:48 ⚠️  Containerd fetch failed: containerd export error: content digest sha256:fe2385f276937dcf780967a5385767fd34b34580c8ed8d303a0cd1485a692635: not found
2026/02/22 14:12:39.552066 [Syncer] ✅ Full sync completed
2026/02/22 14:12:50.528966 ✅ Heartbeat OK (interval=15s)
```

---
## 5. File báo cáo chi tiết

- E2E full: `docs/test-results/E2E-FULL-*.md` (mới nhất)
- API raw: `/home/k8s/KSAM/docs/test-results/e2e-api-results-20260222-141134.txt`
- Log before: `/home/k8s/KSAM/docs/test-results/e2e-logs-before-20260222-141134.txt`
- Log after: `/home/k8s/KSAM/docs/test-results/e2e-logs-after-20260222-141134.txt`

