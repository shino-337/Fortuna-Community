# Pod Detail Component

## Overview

The Pod Detail component provides a security-grade inspection console for Kubernetes pods. It supports security investigation, runtime troubleshooting, supply chain verification, compliance validation, and incident root cause analysis.

Pod Detail collects and presents four data groups per pod:

| Group | Description | Source | DB Table |
|-------|-------------|--------|----------|
| **Runtime Metrics** | CPU/memory (millicore, bytes), restart count, container state | Agent: `pod.Status.ContainerStatuses` | `pod_runtime_metrics` |
| **Processes** | PID, PPID, user, %CPU, %MEM, command, binary_path per container | Agent: exec `ps` or host `/proc` + cgroup | `pod_processes` |
| **Network Connections** | src/dst IP:port, protocol, state per container | Agent: exec `ss`/`netstat` or host `/proc/<pid>/net/tcp,udp` | `pod_network_connections` |
| **K8s Events** | Warning/Normal events with `involved_uid` = pod UID | Agent: SharedInformer `core/v1.Event` | `k8s_events` |

**Runtime Source:** When `POD_DETAIL_RUNTIME_SOURCE=host` (or `auto` with `/host/proc` available), process and network data come from host `/proc` and cgroup rather than exec into containers. Column `runtime_source` (`'host'` | `'exec'`) is stored in DB. Dashboard shows badge "Runtime: Host Inspection" or "Container Exec".

## Architecture

### Services Design (6 services)

| Service | Responsibility | Status |
|---------|---------------|--------|
| **Pod Service** | Sync static pod spec (containers, volumes, security context, phase, podIP, startTime, restartCount, owner, qosClass, specHash) | ✅ Implemented |
| **Runtime Service** | CPU/memory metrics, container runtime state | ✅ Implemented |
| **Process Service** | Aggregate runtime process info per pod/container | ✅ Implemented |
| **Security Service** | Risk scoring, PCE (Pod Capability Engine), insights | ✅ Implemented |
| **Network Service** | Track pod network connections | ✅ Implemented |
| **Event Service** | Persist K8s events (Warning/Normal) related to pods | ✅ Implemented |

**Key code locations:**

- **Agent syncer:** `agent/internal/syncer/syncer.go` — full sync, `buildPayload()`, `SyncOnce()`
- **Agent reporter:** `agent/internal/poddetail/reporter.go` — collect metrics/processes, POST to Core
- **Agent process collector:** `agent/internal/poddetail/process_collector.go` — exec `ps -eo pid,ppid,user,%cpu,%mem,comm`
- **Agent network collector:** `agent/internal/poddetail/network_collector.go` — exec `ss -tunap` or `netstat -tunap`
- **Agent events collector:** `agent/internal/poddetail/events_collector.go` — SharedInformer + batch POST
- **Agent host inspection:** `agent/internal/poddetail/host_map.go`, `cgroup.go`, `proc_scan.go`, `host_process.go`, `host_network.go`, `proc_net.go`
- **Core handlers:** `core/internal/api/pod_detail_services_handlers.go` — GET/POST handlers
- **Core agent service:** `core/internal/service/agent_service.go` — `ProcessSyncedPods`, upsert, PCE trigger
- **Core routes:** `core/internal/api/routes.go` — route registration
- **Core models:** `core/pkg/models/pod_runtime_metrics.go`, `pod_process.go`, `pod_network_connection.go`, `k8s_event.go`
- **Core PCE:** `core/pkg/capability/evaluator.go` — `EvaluateAndUpsertPod(ctx, db, pod, expectedSpecHash)`
- **Core encryption:** `core/internal/api/pod_detail_encrypt.go` — AES-256-GCM for command/binary_path
- **Core WebSocket:** `core/internal/api/pod_detail_ws_hub.go` — live push on ingest
- **Core retention:** `core/internal/scheduler/pod_process_retention_job.go`
- **Dashboard page:** `dashboard/pages/PodDetail.tsx`
- **Dashboard API:** `dashboard/lib/api.ts` — `getPodRuntimeMetrics`, `getPodProcesses`, `getPodNetworkConnections`, `getPodEvents`
- **Migrations:** `068` (pod detail columns), `069` (spec_hash), `070` (last_evaluated_hash), `071` (pod detail services tables), `072`–`073` (refinements), `074` (runtime_source)

### Data Model

**pods table:**
`id`, `cluster_id`, `uid`, `name`, `namespace`, `node_name`, `phase`, `qos_class`, `pod_ip`, `owner_kind`, `owner_name`, `replica_set_name`, `service_account`, `spec_hash`, `last_evaluated_hash`, `host_network`, `host_pid`, `host_ipc`, `containers` (JSONB), `volumes`, `tolerations`, `affinity`, `pod_security_context`, `container_security_contexts`, `start_time`, `restart_count`, `created_at`, `updated_at`, `deleted_at`

Composite indexes: `(cluster_id, uid)`, `(cluster_id, namespace)`, `(cluster_id, node_name)`

**pod_runtime_metrics:** `id`, `pod_uid`, `cluster_id`, `namespace`, `container_name`, `cpu_usage_millicore`, `memory_usage_bytes`, `memory_limit_bytes`, `restart_count`, `state`, `last_observed_at`

**pod_processes:** `id`, `pod_uid`, `cluster_id`, `namespace`, `container_name`, `pid`, `ppid`, `user_name`, `cpu_percent`, `memory_percent`, `command`, `binary_path`, `runtime_source`, `observed_at`
- Index: `pod_uid`, `observed_at`
- Fields `command` and `binary_path` encrypted at-rest (AES-256-GCM) when `POD_DETAIL_ENCRYPTION_KEY` is set

**pod_network_connections:** `id`, `pod_uid`, `cluster_id`, `namespace`, `container_name`, `source_ip`, `source_port`, `dest_ip`, `dest_port`, `protocol`, `state`, `bytes_sent`, `bytes_recv`, `runtime_source`, `observed_at`

**k8s_events:** `id`, `cluster_id`, `namespace`, `event_uid`, `involved_kind`, `involved_uid`, `involved_name`, `reason`, `message`, `event_type`, `count`, `first_timestamp`, `last_timestamp`

**Proposed (not yet implemented):** `containers` (normalized), `pod_exposures`, `risk_events`

## Processing Flow

### Pod Sync Flow (Agent → Core)

```
Kubernetes API
    │ List pods (SYNC_INTERVAL, default 30s)
    ▼
Agent (syncer.go)
    │ buildPayload() → PodPayload with specHash (SHA256, canonical: sorted containers/volumes/env/tolerations)
    │ SyncOnce() → POST /api/v1/agent/sync
    ▼
Core (agent_handlers.go → agent_service.go)
    │ SyncDataFromAgent → normalize clusterId
    │ processSyncedPods: upsert by (cluster_id, uid)
    │   - If spec_hash changed (or empty → non-empty): trigger PCE async
    │   - PCE race protection: EvaluateAndUpsertPod(ctx, db, pod, expectedSpecHash)
    │     Skip if spec_hash mismatch at start; re-check before write;
    │     set last_evaluated_hash atomically only when spec_hash still matches
    │   - Full sync: soft-delete pods not in payload
    │   - EnsureActiveInstance (pod_instances)
    ▼
Database (PostgreSQL)
    │ pods, pod_instances, pod_capabilities, pod_risk_profiles
    ▼
Dashboard
    │ GET /api/v1/pods/:id or /pods/:podUid
```

### Runtime Metrics Collection

```
Agent (reporter.go, interval POD_DETAIL_REPORT_INTERVAL default 2m)
    │ listPodsOnNode (K8s API, field selector spec.nodeName=)
    │ For each pod (skip uid="" or "0"):
    │   sendRuntimeMetrics from pod.Status.ContainerStatuses
    │   POST /api/v1/agent/pod-runtime-metrics
    ▼
Core
    │ IngestPodRuntimeMetricsPayload → validate podUid → CreateInBatches
    │ BroadcastPodDetailUpdate(podUid, "metrics")
    ▼
DB: pod_runtime_metrics
```

### Process Data Collection

**Exec mode (default):**
```
Agent → exec into each container: ps -eo pid,ppid,user,%cpu,%mem,comm
       → parse output → POST /api/v1/agent/pod-processes
```

**Host mode** (`POD_DETAIL_RUNTIME_SOURCE=host`):
```
Agent → scan /host/proc (hostPID: true, volume mount /proc → /host/proc)
      → read /proc/<pid>/cgroup → parse containerID (cgroup v1 and v2)
      → buildContainerIDToPodMap from pod.Status.ContainerStatuses
      → map PID → container → pod
      → read /proc/<pid>/comm, /proc/<pid>/stat, /proc/<pid>/status (CapEff)
      → CollectProcessesFromHost → group by podUid → POST
```

**Process diff detection:** Snapshot current vs previous → create `runtime_events` with `capability=PROCESS_SNAPSHOT_DIFF`

### Host Inspection

Host inspection replaces exec-based collection for distroless containers and environments where exec is blocked.

**DaemonSet requirements:** `hostPID: true`, volume hostPath `/proc` → `/host/proc` (readOnly)

**Cgroup mapping:**
- Cgroup v2: `0::/kubepods.slice/kubepods-pod<uid>.slice/cri-containerd-<containerID>.scope`
- Cgroup v1: Multiple lines, find `containerd`/`cri-containerd` path segment
- ContainerID normalization: suffix match (12 chars) between K8s `ContainerID` and cgroup-extracted ID

**Implementation files:**
- `agent/internal/poddetail/host_map.go` — containerID → (podUID, namespace, containerName) map
- `agent/internal/poddetail/cgroup.go` — cgroup v1/v2 parsing
- `agent/internal/poddetail/proc_scan.go` — list PIDs from /proc
- `agent/internal/poddetail/host_process.go` — process collection from /proc
- `agent/internal/poddetail/host_network.go` — network from /proc/<pid>/net/tcp,udp
- `agent/internal/poddetail/proc_net.go` — parse /proc/net kernel hex format

**Environment variables:**
- `POD_DETAIL_RUNTIME_SOURCE`: `host` | `exec` | `auto` (auto tries host first if `/host/proc` available)
- `POD_DETAIL_PROC_ROOT`: default `/host/proc`

## Technical Details

### Database Schema

Migrations in order: `068` → `069` → `070` → `071` → `072` → `073` → `074`

- **068:** Add `pod_ip`, `start_time`, `restart_count`, `owner_kind`, `owner_name`, `replica_set_name`, `qos_class` to `pods`
- **069:** Add `spec_hash` to `pods`
- **070:** Add `last_evaluated_hash` to `pods`
- **071:** Create `pod_runtime_metrics`, `pod_processes`, `pod_network_connections`, `k8s_events`
- **074:** Add `runtime_source` column

Deploy Core first (runs migrations on startup), then Agent.

### API Endpoints

**Ingest (Agent → Core), base path `/api/v1/agent`:**

| Method | Path | Body | Notes |
|--------|------|------|-------|
| POST | `/api/v1/agent/sync` | SyncPayload with pods[] | Main pod sync |
| POST | `/api/v1/agent/pod-runtime-metrics` | `{ podUid, clusterId, namespace, metrics[] }` | Reject if podUid empty or "0" |
| POST | `/api/v1/agent/pod-processes` | `{ podUid, clusterId, namespace, processes[] }` | Reject if podUid empty or "0" |
| POST | `/api/v1/agent/pod-network-connections` | `{ podUid, clusterId, namespace, connections[] }` | Reject if podUid empty or "0" |
| POST | `/api/v1/agent/pod-events` | `{ clusterId, events[] }` | K8s events batch |
| POST | `/api/v1/runtime/events` | Runtime events | No JWT required |

**Read (Dashboard → Core), base path `/api/v1`:**

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/pods?cluster=&namespace=&node=&page=&pageSize=` | List pods with riskCount |
| GET | `/api/v1/pods/:id` | Pod by primary key |
| GET | `/api/v1/pods/:podUid` | Pod by UID |
| GET | `/api/v1/pods/:podUid/runtime-metrics` | Container CPU/memory/state |
| GET | `/api/v1/pods/:podUid/processes` | Process list (decrypted) |
| GET | `/api/v1/pods/:podUid/network-connections` | Network connections |
| GET | `/api/v1/pods/:podUid/events` | K8s events for pod |
| GET | `/api/v1/pods/:podUid/spec` | Pod spec YAML |
| GET | `/api/v1/pods/:podUid/capabilities` | Pod capabilities |
| GET | `/api/v1/inventory/pods/:uid/sbom` | SBOM (includes sbomSource, confidence) |
| GET | `/api/v1/risks/pods/:uid/report` | Risk report |
| GET | `/api/v1/risk/pods/:uid/runtime/events` | Security runtime events |
| GET | `/api/v1/ws/pod/:uid` | WebSocket live updates |

**Response shape (GET):**
```json
{
  "podUid": "<uid>",
  "items": [ ... ]
}
```
Dashboard always uses the `items` array. Empty results return `items: []`.

### Storage and Cleanup

| Table | Retention | GET Limit | Notes |
|-------|-----------|-----------|-------|
| `pod_runtime_metrics` | No TTL | 100 newest | — |
| `pod_processes` | 30 days (`POD_PROCESS_RETENTION_DAYS`), job deletes old | 1000, latest snapshot | Fields truncated: command 1024, binary_path 512, user 128 |
| `pod_network_connections` | Deleted when pod sync removes pod | 500 | `bytes_sent`/`bytes_recv` = `tx_queue`/`rx_queue` from `/proc/net/*` snapshot |
| `k8s_events` | Unique `(cluster_id, event_uid)` OnConflict DoNothing | — | — |

**Encryption at-rest:** `command`, `binary_path` in `pod_processes` encrypted AES-256-GCM when `POD_DETAIL_ENCRYPTION_KEY` is set (env or K8s Secret `fortuna-secrets` key `pod-detail-encryption-key`). GET decrypts before returning. File: `pod_detail_encrypt.go`.

**On pod deletion** (soft-delete during sync): Core deletes rows by `pod_uid` in all 4 detail tables.

### Code Paths

**Agent:**
- `agent/internal/syncer/syncer.go` — `buildPayload()`, `SyncOnce()`, `computePodSpecHash()` (canonical: containers sorted by name, volumes by name, env by name, tolerations by key+value+effect)
- `agent/internal/poddetail/reporter.go` — `reportOnce()`, `listPodsOnNode()`, `postWithRetry()` (3 retries, exponential backoff, retry on 429/5xx)
- `agent/internal/poddetail/process_collector.go` — `CollectProcessesFromPod()` (exec `ps`)
- `agent/internal/poddetail/network_collector.go` — `CollectNetworkFromPod()` (exec `ss`/`netstat`)
- `agent/internal/poddetail/events_collector.go` — `EventsCollector` (SharedInformer, batch 30s)
- `agent/internal/poddetail/host_*.go`, `cgroup.go`, `proc_scan.go`, `proc_net.go` — host inspection
- `agent/cmd/main.go` — starts reporter and events collector

**Core:**
- `core/internal/api/agent_handlers.go` — `SyncDataFromAgent`
- `core/internal/service/agent_service.go` — `SyncData`, `processSyncedPods`, `evaluatePodCapabilities` (async goroutine with specHash)
- `core/internal/api/pod_detail_services_handlers.go` — GET/POST handlers for 4 data types
- `core/internal/api/pod_detail_encrypt.go` — `InitPodDetailEncryptionKey(key)`
- `core/internal/api/pod_detail_ws_hub.go` — WebSocket hub, `BroadcastPodDetailUpdate(podUid, type)`
- `core/internal/api/pod_detail_ingest_retry.go` — `dbIngestWithRetry` (3 retries, backoff 1s/2s/4s)
- `core/internal/api/routes.go` — route registration
- `core/internal/config/config.go` — `PodDetailEncryptionKey`
- `core/pkg/capability/evaluator.go` — `EvaluateAndUpsertPod(ctx, db, pod, expectedSpecHash)`
- `core/internal/scheduler/pod_process_retention_job.go` — retention cleanup
- `core/migrations/068_*` through `074_*` — schema migrations

**Dashboard:**
- `dashboard/pages/PodDetail.tsx` — tabs: Overview, SBOM, Related Risks, Processes, Network, Events
- `dashboard/lib/api.ts` — API client functions
- `dashboard/types.ts` — TypeScript types for metrics, process, network, event

**Deploy:**
- `deploy/fortuna-agent-daemonset.yaml` — hostPID, /host/proc volume, memory limit 6Gi, env vars
- `deploy/fortuna-core-deployment.yaml` — startupProbe
- `scripts/deploy/deploy-fortuna-robust.sh`

**Verify scripts:**
- `scripts/verify/verify-pod-detail-api-and-db.sh`
- `scripts/verify/verify-pod-detail-empty-response.sh`
- `scripts/verify/verify-admission-risk-gate.sh`
- `scripts/e2e/run-pod-detail-test-suite.sh`

**Environment variables (Agent):**
- `CORE_HTTP_ENDPOINT` — Core API URL
- `SYNC_INTERVAL` — default 30s
- `POD_DETAIL_REPORT_INTERVAL` — default 2m
- `POD_DETAIL_RUNTIME_SOURCE` — `host` | `exec` | `auto`
- `POD_DETAIL_PROC_ROOT` — default `/host/proc`
- `SBOM_WORKERS` — default 1
- `FALCO_EVENTS_ENABLED`, `FALCO_EVENTS_PATH`, `FALCO_EVENTS_POLL`
- `EBPF_ENABLED` — default false
- `LOG_LEVEL` — debug suppresses exec tool-not-found errors

**Environment variables (Core):**
- `POD_DETAIL_ENCRYPTION_KEY`
- `POD_PROCESS_RETENTION_DAYS` — default 30
- `POD_DETAIL_NET_SPIKE_COOLDOWN_MINUTES`
- `ADMISSION_RISK_GATE_ENABLED`, `ADMISSION_RISK_SENSITIVE_NAMESPACES`, `ADMISSION_RISK_BLOCK_THRESHOLD`

## UI/UX

### Pod Detail Page specification

**Layout:**
```
┌─────────────────────────────────────────┐
│ Breadcrumb + Cluster + Namespace        │
├─────────────────────────────────────────┤
│ Sticky Pod Header (name, status, risk)  │
├─────────────────────────────────────────┤
│ Horizontal Tab Navigation               │
├─────────────────────────────────────────┤
│ Tab Content Area                        │
└─────────────────────────────────────────┘
```

**Header fields:** Status badge, Pod Name (copyable), Namespace (tag), Node (link), Owner (link), Restart Count (badge), Risk Score (colored), Age. Action buttons: Exec, Logs, Describe, Export YAML, Delete.

**Tabs:** Overview | Configuration | Runtime | Runtime Process Monitoring | Network | Security (SBOM + Risks) | Events | Logs | Related Resources

**Visual hierarchy:** Pod Status > Restart Count > CPU/Memory > Suspicious Process > Security Risk > Metadata

**Color system:** Running=Green, Warning=Orange, Critical=Red, Info=Blue, Neutral=Gray. Dark theme default.

**Process table columns:** PID, PPID, User, CPU%, MEM%, Start Time, Command, Container. Highlight: CPU>70% orange, CPU>90% red, root user red. Click row → side drawer with: full command, binary path, parent chain, network connections, security insights.

**Network table columns:** Source, Destination, Port, Protocol, Bytes, State. Filter internal/external, highlight unknown external IPs.

**Events columns:** Time, Type, Reason, Message. Filter: Warning/Error/Normal.

**Data loading:** Static config fetched on load; runtime metrics poll 10s; process monitoring via WebSocket; network poll 15s. Dashboard uses `pod.id ?? pod.uid` as identifier. Empty results show appropriate empty state messages.

**Performance:** Load time <2s at p95; data fetched in parallel; lazy-load heavy sections (Events, SBOM); process table uses row virtualization.

## Testing

**Test suite:** `scripts/e2e/run-pod-detail-test-suite.sh`

**Test groups:**
1. **Spec Hash Correctness** — Same spec→same hash; order change doesn't change hash; status change doesn't trigger PCE; security context change triggers PCE
2. **Async PCE Correctness** — Sync returns before PCE completes; `last_evaluated_hash` updated correctly
3. **Race Condition Protection** — Stale worker result discarded; high-frequency updates handled
4. **Backward Compatibility** — Old agent (no specHash) still triggers PCE; agent upgrade path
5. **Soft Delete & Restore** — Delete sets `deleted_at`; restore triggers PCE
6. **Scale Test** — 500 pods initial sync stable; agent restart no PCE flood
7. **Status-Only Change** — Container restart doesn't trigger PCE
8. **Database Consistency** — Composite index usage verified
9. **Failure Scenarios** — Worker crash mid-evaluation; Core restart during evaluation
10. **Security Hardening** — Tampered clusterId rejected

**Unit tests:**
- Agent: `TestComputePodSpecHash` (stable, different spec, 64-char hex, order insensitivity)
- Core: `TestProcessSyncedPods_SpecHashStoredAndConditionalPCE`, `TestProcessSyncedPods_NullToNonNullSpecHash`, `TestEvaluateAndUpsertPod_SkipsWhenSpecHashMismatch`
- Host inspection: `host_map_test`, `cgroup_test`, `proc_scan_test`, `host_process_test`, `proc_net_test`

**Run tests:**
- Core: `go test ./internal/service/... ./pkg/capability/... ./migrations/...`
- Agent: `go test ./internal/syncer/... ./internal/poddetail/...`

**Dashboard test cases (Network):**
- TC-NET-1: Preloads runtime-metrics, processes, network-connections on pod load
- TC-NET-2: Network tab displays Direction, Remote address, Local port, Protocol, Status, Timestamp
- TC-NET-3: E2E `scripts/e2e/test-pod-detail-ping-flow.sh` — asserts GET response shape
- TC-NET-4: E2E `scripts/e2e/test-pod-detail-lodash-network.sh` — tests LISTEN on port 3000

**Acceptance criteria:** No stale risk overwrite; no PCE flood on restart; hash stable under order changes; worker queue bounded; risk matches latest spec_hash; sync latency unaffected by PCE.

## Related ADRs

- **Pod Sync Architecture:** `docs/03-components/podDetail/` (this document) — event-driven sync proposed; informer-based sync planned for Phase 2
- **Runtime Monitoring Architecture:** `docs/02-architecture/Runtime_Monitoring_Architecture.md` — host inspection vs exec design
- **Storage and Cleanup:** `docs/02-architecture/pod-detail-storage-and-cleanup.md` — retention, encryption, limits
- **Architecture Findings:** `docs/02-architecture/Architecture_Finding_Remediation_Plan.md` — 11 findings analysis
- **Risk Center Gaps:** `docs/03-components/risk-center/GAPS.md` — full GAP + roadmap
- **SBOM Distroless:** `docs/03-components/sbom/DISTROLESS_SBOM_SPEC.md` — heuristic SBOM for distroless images
