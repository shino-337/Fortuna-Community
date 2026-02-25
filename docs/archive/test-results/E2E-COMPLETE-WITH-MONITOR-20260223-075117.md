# E2E Complete – Monitor Agent/Core & API kết quả thực tế

**Thời gian:** 2026-02-23T07:52:35+00:00
**Namespace:** fortuna

---
## 1. Test scripts đã chạy

| Script | Mô tả |
|--------|--------|
| run-e2e-full.sh | E2E full (cluster, API, DB, dashboard) |
| test-priority1-apis.sh | Promotion rules, runtime-signals API |
| test-runtime-signals-e2e.sh | POST runtime-events, DB, GET runtime-signals |
| e2e-dashboard-data.sh | Dashboard chart data (Threat Velocity, PCE Trend) |
| e2e-risk-center-verify.sh | Risk Center: seed insight, /risks, /insights/summary, /runtime-signals |

---
## 2. Kết quả API thực tế (clusters, dashboard/stats, agents/status, insights/summary, Risk Center)

```json
Refresh dashboard (admin/admin123) to see data.



========== E2E Risk Center – seed insight & verify APIs ==========
[0;34m[E2E Risk Center][0m Core pod: fortuna-core-76dd8f498d-9czq4 | PG pod: postgres-5484f7745d-gsr4f

[1;33m[WARN][0m No cluster or pod in DB (cluster_id=manual-sync-cluster, pod_uid=). Skip seeding insight; verify APIs only.

[0;34m[E2E Risk Center][0m GET /api/v1/risks (clusterId=manual-sync-cluster)...
[1;33m[WARN][0m GET /risks: total=0 (Risk Center tab Risks will show empty if 0)

[0;34m[E2E Risk Center][0m GET /api/v1/insights/summary (clusterId=manual-sync-cluster)...
[1;33m[WARN][0m GET /insights/summary: total=0 (Risk Center severity bar will show zeros if all 0)

[0;34m[E2E Risk Center][0m GET /api/v1/runtime-signals (Risk Center Reference tab)...
[0;32m[OK][0m GET /runtime-signals: total=4, count=4

========== Risk Center E2E summary ==========
[1;33m[WARN][0m Risk Center risks/summary still 0. Ensure insights exist for pods in the selected cluster (e.g. run this script after cluster sync; or seed insight with valid pod uid).


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
            "lastSync": "2026-02-23T07:52:16.758826Z",
            "createdAt": "2026-02-23T04:57:37.876102Z",
            "updatedAt": "2026-02-23T07:52:16.758826Z"
        },
        {
            "id": "manual-sync-cluster",
            "name": "manual-sync-cluster",
            "source": "auto",
            "region": "",
            "endpoint": "",
            "status": "active",
            "lastSync": "2026-02-23T05:02:11.888386Z",
            "createdAt": "2026-02-23T04:56:51.458475Z",
            "updatedAt": "2026-02-23T05:02:11.888386Z"
        }
    ]
}

=== GET /api/v1/dashboard/stats ===
{
    "totalClusters": 2,
    "activeAgents": 2,
    "runningPods": 7,
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
            "lastHeartbeat": "2026-02-23T07:52:16.753127Z",
            "nodeName": "k8s-master",
            "status": "healthy",
            "version": "dev"
        },
        {
            "agentId": "k8s-worker01-agent",
            "clusterId": "sha256-c93f5c6cb0e57f8f",
            "clusterName": "cluster-c93f5c6c",
            "lastHeartbeat": "2026-02-23T07:52:14.047904Z",
            "nodeName": "k8s-worker01",
            "status": "healthy",
            "version": "dev"
        }
    ],
    "disconnected": 0,
    "healthy": 2,
    "slow": 0,
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

2026/02/23 07:49:19 [32mgithub.com/fortuna/core/internal/api/insights_handlers.go:297
[0m[33m[1.491ms] [34;1m[rows:0][0m SELECT insight_type, COUNT(*) as count FROM "insights" WHERE (deleted_at IS NULL AND (status = 'active' OR status IS NULL)) AND "insights"."deleted_at" IS NULL GROUP BY "insight_type"
[GIN] 2026/02/23 - 07:49:19 | 200 |    6.117297ms |       127.0.0.1 | GET      "/api/v1/insights/summary"
[GIN] 2026/02/23 - 07:49:20 | 200 |      48.611µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/23 - 07:49:20 | 200 |     2.88031ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:49:25 | 200 |    2.638445ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:49:30 | 200 |      77.575µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/23 - 07:49:30 | 200 |    4.272651ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:49:35 | 200 |    2.042676ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:49:40 | 200 |      62.647µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/23 - 07:49:40 | 200 |    2.637795ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:49:45 | 200 |    2.444294ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:49:50 | 200 |     213.808µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/23 - 07:49:50 | 200 |    2.944445ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:49:55 | 200 |    2.836645ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:50:00 | 200 |      72.495µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/23 - 07:50:00 | 200 |    2.145447ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:50:05 | 200 |     3.13959ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:50:10 | 200 |     129.782µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/23 - 07:50:10 | 200 |    2.744111ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:50:15 | 200 |     2.36676ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:50:20 | 200 |      59.901µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/23 - 07:50:20 | 200 |    2.937202ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:50:25 | 200 |    2.765081ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:50:30 | 200 |     239.486µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/23 - 07:50:30 | 200 |    2.962148ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:50:35 | 200 |    2.519815ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:50:40 | 200 |      44.533µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/23 - 07:50:40 | 200 |    3.412437ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:50:45 | 200 |    2.358726ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:50:50 | 200 |      60.353µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/23 - 07:50:50 | 200 |    2.460915ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:50:55 | 200 |    2.518783ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:51:00 | 200 |      65.392µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/23 - 07:51:00 | 200 |    2.814324ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:51:05 | 200 |    2.243571ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:51:10 | 200 |      78.106µs |      10.244.0.1 | GET      "/healthz"
[GIN] 2026/02/23 - 07:51:10 | 200 |    3.122217ms |      10.244.0.1 | GET      "/ready"
[GIN] 2026/02/23 - 07:51:15 | 200 |    2.392498ms |      10.244.0.1 | GET      "/ready"

=== Agent (last 50 lines, sync/heartbeat) ===
[SBOMExtractor] 2026/02/23 07:47:50 ⚠️  Containerd fetch failed: containerd export error: content digest sha256:4397c18a33dce22f6f6cd4cb37836285a4f81d74c033054f6cdcb41c5dc4181a: not found
[SBOMExtractor] 2026/02/23 07:47:34 ⚠️  Containerd fetch failed: containerd export error: content digest sha256:7e55d1d0d2c095cecd55022878b8e64d88176157c6ed49a89e2ced129d26465e: not found
```

---
## 4. Agent/Core logs (sau khi chạy E2E)

```
=== Core (last 50 lines) ===
2026/02/23 07:52:33 [32mgithub.com/fortuna/core/internal/api/dashboard_handlers.go:335
[0m[33m[2.220ms] [34;1m[rows:0][0m SELECT * FROM "insights" WHERE (resource_type = 'Pod' AND resource_uid IN (SELECT uid FROM pods WHERE cluster_id = 'manual-sync-cluster' AND deleted_at IS NULL)) AND status = 'active' AND insight_type = 'vulnerability' AND "insights"."deleted_at" IS NULL ORDER BY detected_at DESC LIMIT 5
[GIN] 2026/02/23 - 07:52:33 | 200 |    9.382119ms |       127.0.0.1 | GET      "/api/v1/risks?page=1&pageSize=5&clusterId=manual-sync-cluster"

2026/02/23 07:52:33 [32mgithub.com/fortuna/core/internal/middleware/auth.go:45
[0m[33m[1.443ms] [34;1m[rows:1][0m SELECT * FROM "users" WHERE "users"."id" = 1 AND "users"."deleted_at" IS NULL ORDER BY "users"."id" LIMIT 1

2026/02/23 07:52:33 [31;1mgithub.com/fortuna/core/internal/api/cluster_id.go:33 [35;1mrecord not found
[0m[33m[1.227ms] [34;1m[rows:0][0m SELECT * FROM "clusters" WHERE (name = 'manual-sync-cluster' AND id LIKE 'sha256-%') AND "clusters"."deleted_at" IS NULL ORDER BY "clusters"."id" LIMIT 1

2026/02/23 07:52:33 [32mgithub.com/fortuna/core/internal/api/insights_handlers.go:178
[0m[33m[1.591ms] [34;1m[rows:1][0m SELECT COUNT(*) FROM pods WHERE cluster_id = 'manual-sync-cluster' AND deleted_at IS NULL

2026/02/23 07:52:33 [32mgithub.com/fortuna/core/internal/api/insights_handlers.go:179
[0m[33m[2.041ms] [34;1m[rows:1][0m SELECT count(*) FROM "insights" WHERE (deleted_at IS NULL AND (status = 'active' OR status IS NULL)) AND "insights"."deleted_at" IS NULL
2026/02/23 07:52:33.980021 [InsightsSummary] clusterId="manual-sync-cluster" normalized; pods_in_cluster=0, insights_global=0

2026/02/23 07:52:33 [32mgithub.com/fortuna/core/internal/api/insights_handlers.go:189
[0m[33m[2.309ms] [34;1m[rows:1][0m 
				SELECT COUNT(*) FROM insights i
				INNER JOIN pods p ON p.uid = i.resource_uid AND p.cluster_id = 'manual-sync-cluster' AND p.deleted_at IS NULL
				WHERE i.deleted_at IS NULL AND (i.status = 'active' OR i.status IS NULL)
2026/02/23 07:52:33.982424 [InsightsSummary] clusterId="manual-sync-cluster" join result total=0

2026/02/23 07:52:33 [32mgithub.com/fortuna/core/internal/api/insights_handlers.go:215
[0m[33m[4.303ms] [34;1m[rows:0][0m 
				SELECT LOWER(i.severity) as severity, COUNT(*) as count 
				FROM insights i
				INNER JOIN pods p ON p.uid = i.resource_uid AND p.cluster_id = 'manual-sync-cluster' AND p.deleted_at IS NULL
				WHERE i.deleted_at IS NULL AND (i.status = 'active' OR i.status IS NULL)
				GROUP BY LOWER(i.severity)

2026/02/23 07:52:33 [32mgithub.com/fortuna/core/internal/api/insights_handlers.go:243
[0m[33m[2.101ms] [34;1m[rows:0][0m 
				SELECT i.insight_type, COUNT(*) as count 
				FROM insights i
				INNER JOIN pods p ON p.uid = i.resource_uid AND p.cluster_id = 'manual-sync-cluster' AND p.deleted_at IS NULL
				WHERE i.deleted_at IS NULL AND (i.status = 'active' OR i.status IS NULL)
				GROUP BY i.insight_type
[GIN] 2026/02/23 - 07:52:33 | 200 |   15.643122ms |       127.0.0.1 | GET      "/api/v1/insights/summary?clusterId=manual-sync-cluster"

2026/02/23 07:52:34 [32mgithub.com/fortuna/core/internal/middleware/auth.go:45
[0m[33m[1.359ms] [34;1m[rows:1][0m SELECT * FROM "users" WHERE "users"."id" = 1 AND "users"."deleted_at" IS NULL ORDER BY "users"."id" LIMIT 1

2026/02/23 07:52:34 [32mgithub.com/fortuna/core/internal/api/runtime_signals_handlers.go:73
[0m[33m[1.781ms] [34;1m[rows:1][0m SELECT count(*) FROM "runtime_signals"

2026/02/23 07:52:34 [32mgithub.com/fortuna/core/internal/api/runtime_signals_handlers.go:82
[0m[33m[1.631ms] [34;1m[rows:4][0m SELECT * FROM "runtime_signals" ORDER BY created_at DESC LIMIT 10
[GIN] 2026/02/23 - 07:52:34 | 200 |    5.155164ms |       127.0.0.1 | GET      "/api/v1/runtime-signals?limit=10"

=== Agent (last 60 lines, sync/heartbeat) ===
[SBOMExtractor] 2026/02/23 07:47:49 ⚠️  Containerd fetch failed: containerd export error: content digest sha256:53bec469a42df1e193889862235aec83f7dcc0a063cb861343926d3301e140be: not found
[SBOMExtractor] 2026/02/23 07:47:50 ⚠️  Containerd fetch failed: containerd export error: content digest sha256:4397c18a33dce22f6f6cd4cb37836285a4f81d74c033054f6cdcb41c5dc4181a: not found
2026/02/23 07:52:18.036096 [Syncer] ✅ Full sync completed
2026/02/23 07:52:29.349802 ✅ Heartbeat OK (interval=15s)
[LocalPodWatcher] 2026/02/23 07:51:45    → Queued pod fortuna/e2e-dashboard-pod-1771833102 for async processing (transitioned to Running)
[SBOMExtractor] 2026/02/23 07:51:45 ⚠️  Containerd fetch failed: containerd export error: content digest sha256:fe2385f276937dcf780967a5385767fd34b34580c8ed8d303a0cd1485a692635: not found
2026/02/23 07:52:15.417125 [Syncer] ✅ Full sync completed
2026/02/23 07:52:26.765169 ✅ Heartbeat OK (interval=15s)
```

---
## 5. File báo cáo chi tiết

- E2E full: `docs/test-results/E2E-FULL-*.md` (mới nhất)
- API raw: `/home/k8s/KSAM/docs/test-results/e2e-api-results-20260223-075117.txt`
- Log before: `/home/k8s/KSAM/docs/test-results/e2e-logs-before-20260223-075117.txt`
- Log after: `/home/k8s/KSAM/docs/test-results/e2e-logs-after-20260223-075117.txt`

