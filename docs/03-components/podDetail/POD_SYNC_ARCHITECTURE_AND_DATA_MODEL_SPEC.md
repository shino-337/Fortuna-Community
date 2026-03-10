# Pod Sync Architecture & Data Model Optimization Spec

## 1. Purpose

This document defines:

- Assessment of current Pod sync architecture
- Identified risks and blind spots
- Proposed production-grade sync architecture
- Optimized data model for scalability
- Risk engine decoupling strategy
- Multi-cluster hardening recommendations

Target scale:
- 10k+ pods per cluster
- Multi-cluster support
- Security-grade visibility
- Near real-time update

---

# 2. Current Architecture Assessment

## 2.1 Current Flow

Kubernetes → Node Agent → Core API → AgentService → DB → Read API → Dashboard

Sync interval:
- Polling-based (default 30s)

Data scope:
- Pods
- ServiceAccounts
- Roles
- RoleBindings
- Deployments
- ReplicaSets

Risk evaluation:
- Triggered during full sync
- Evaluated per pod

---

# 3. Identified Issues

## 3.1 Polling-Based Sync

Problem:
- List all Pods every 30s
- High API load in large clusters
- Inefficient resource usage

Impact:
- CPU spike
- Network overhead
- Latency at scale

Recommendation:
- Replace polling with Kubernetes Informer (watch-based).
- Use full reconciliation every 15–30 minutes only.

---

## 3.2 Delta Sync Is Not True Delta

Current behavior:
- Delta only processes pods if data.pods exists.

Issues:
- Deleted pods may not be tracked properly.
- RBAC changes may impact pod risk but not trigger re-evaluation.
- No resourceVersion tracking.

Required improvements:
- Track resourceVersion per resource.
- Include tombstone events.
- Include changed object hash comparison.

---

## 3.3 Risk Evaluation Tight Coupling

Current:
- Risk and PCE evaluated during sync flow.
- Evaluate per pod.

Risks:
- N+1 query problem.
- High compute during full sync.
- Sync latency increases with cluster size.

Required:
- Decouple risk engine.
- Introduce async evaluation queue.
- Evaluate only if spec_hash changed.

---

## 3.4 Container-Level Data Loss

Current:
- restartCount stored at pod level.

Issues:
- Container-specific crash invisible.
- Cannot detect container exploit patterns.

Required:
- Store container-level runtime state.
- Separate container table.

---

## 3.5 Soft Delete Logic Risk

Full sync:
- Soft delete pods missing from payload.

Risk:
- If agent scope is node-level, deletion may be incorrect.
- Multi-agent environment risk of accidental deletion.

Requirement:
- Define sync scope clearly:
    - Cluster-level sync
    - Node-level sync
- Cleanup must respect scope boundary.

---

## 3.6 Over-Fetching in Read API

Current:
- Full ORM model returned in detail API.

Problems:
- Tight coupling DB schema ↔ API contract.
- Payload bloat.
- Hard to version.

Required:
- Introduce DTO layer.
- Separate List DTO and Detail DTO.
- Lazy-load heavy sections (SBOM, Events).

---

# 4. Production-Grade Sync Architecture

## 4.1 Event-Driven Model

Replace periodic polling with:

Agent:
- Use SharedInformer for:
    - Pods
    - ServiceAccounts
    - Roles
    - RoleBindings
    - Deployments
    - ReplicaSets

On Add/Update/Delete:
- Push delta event immediately.

Periodic:
- Full reconciliation every 15–30 minutes.

---

## 4.2 Sync Protocol

Payload must include:

- cluster_id (derived from mTLS certificate)
- agent_id
- resource_type
- event_type (ADD | UPDATE | DELETE)
- resource_uid
- resource_version
- spec_hash
- payload

Compression:
- Gzip required.

Integrity:
- HMAC or signature validation.

---

## 4.3 Hash-Based Change Detection

For each Pod store:

- spec_hash
- security_hash
- network_hash
- config_hash

Only trigger:
- Risk evaluation
- PCE evaluation
- Exposure recalculation

If relevant hash changed.

---

## 4.4 Asynchronous Risk Engine

Sync Flow:
1. Upsert pod
2. Compare spec_hash
3. If changed → enqueue evaluation job

Worker:
- Fetch pod snapshot
- Compute risk
- Update risk summary

Never block sync transaction.

---

# 5. Exposure Resolution Layer

Introduce Exposure Resolver:

When Pod sync occurs:
- Resolve Service mapping
- Resolve Ingress mapping
- Resolve LoadBalancer exposure
- Resolve NetworkPolicy isolation

Store results in pod_exposures table.

Fields:
- pod_id
- exposed (bool)
- exposure_type (ClusterIP | NodePort | LB | Ingress)
- public (bool)
- ingress_host
- port
- protocol

---

# 6. Optimized Data Model

## 6.1 pods

Columns:
- id (PK)
- cluster_id (indexed)
- uid (indexed)
- name
- namespace (indexed)
- node_name (indexed)
- phase
- qos_class
- pod_ip
- owner_kind
- owner_name
- replica_set_name
- service_account
- spec_hash
- security_hash
- network_hash
- config_hash
- risk_score (indexed)
- risk_critical
- risk_high
- risk_medium
- risk_low
- created_at
- updated_at
- deleted_at (soft delete)

Composite Index:
- (cluster_id, uid)
- (cluster_id, namespace)
- (cluster_id, node_name)

---

## 6.2 containers

Columns:
- id (PK)
- pod_id (FK indexed)
- name
- image
- image_digest
- privileged
- allow_privilege_escalation
- read_only_root_fs
- restart_count
- ready
- state
- created_at
- updated_at

Index:
- (pod_id)

---

## 6.3 container_capabilities

- container_id
- capability_added
- capability_dropped

---

## 6.4 pod_volumes

- pod_id
- volume_type
- volume_name
- mount_path
- host_path_flag (bool)
- secret_flag (bool)

---

## 6.5 pod_exposures

- pod_id
- exposure_type
- public_flag
- ingress_host
- port
- protocol

---

## 6.6 pod_instances (runtime lifecycle)

- pod_id
- node_name
- start_time
- end_time
- restart_count
- active_flag

---

## 6.7 risk_events

- pod_id
- rule_id
- severity
- status
- first_seen
- last_seen

---

# 7. Multi-Cluster Security Hardening

## 7.1 Cluster Identity

- cluster_id must be derived from certificate.
- Agent must authenticate via mTLS.
- Reject cluster_id from payload body.

## 7.2 Agent Isolation

- Each agent scoped to:
    - Full cluster OR
    - Node-specific
- Deletion logic must respect scope.

---

# 8. Performance Targets

- Sync latency < 1s per event.
- Risk evaluation async.
- List API 95p < 500ms.
- Detail API < 2s including heavy tabs.

---

# 9. Required Immediate Improvements

Priority 1:
- Introduce spec_hash comparison.
- Decouple risk engine.
- Add container-level table.

Priority 2:
- Implement informer-based sync.
- Add exposure resolution layer.

Priority 3:
- Introduce DTO layer.
- Optimize indexes.

---

# 10. Non-Goals

This spec does not cover:
- Full SBOM storage strategy.
- Long-term audit storage.
- SIEM export pipeline.

Separate spec required for those.

---

# 11. Conclusion

Current system is suitable for:
- Inventory view
- Basic CVE reporting

To become production-grade security platform:
- Must adopt event-driven sync
- Must decouple risk engine
- Must normalize container data
- Must implement exposure resolution
- Must harden multi-cluster identity

---

# 12. Gap vs Current Implementation

This section maps the spec to the codebase and lists what is already done vs pending.

| Spec section | Current state | Location / notes |
|--------------|---------------|------------------|
| **2.1 Current flow** | Implemented | Agent: `agent/internal/syncer/syncer.go` (SyncOnce, buildPayload). Core: `core/internal/api/agent_handlers.go` (SyncDataFromAgent), `core/internal/service/agent_service.go` (SyncData, processSyncedPods). |
| **3.1 Polling** | Still polling (List every SYNC_INTERVAL) | Agent config: `SYNC_INTERVAL` (default 30s). Informer not yet used. |
| **3.2 Delta sync** | Delta only processes pods when `data.pods` present; no resourceVersion/tombstone | processSyncedPods called with isFullSync=false on delta; no version tracking. |
| **3.3 Risk coupling** | PCE runs synchronously after each pod upsert | evaluatePodCapabilities called in processSyncedPods after Create/Update/Restore. **Adjusted:** PCE runs asynchronously and only when spec_hash changed (see §13). |
| **3.4 Container-level** | restartCount at pod level only; no containers table | pods.restart_count (total). **Adjusted:** spec_hash added; containers table deferred to later phase. |
| **3.5 Soft delete scope** | Full sync soft-deletes pods not in payload | cleanupStalePods; agent scope is cluster-wide (list all pods in namespace or all namespaces). |
| **3.6 Over-fetch** | Full Pod model in API | GetPod/GetPodByUID return full GORM model; no DTO yet. |
| **4.x Event-driven** | Not implemented | Still polling. |
| **4.2 Sync protocol** | Plain JSON POST; no compression/HMAC | agent/sync accepts JSON. |
| **4.3 Hash-based detection** | **Implemented** | spec_hash on pods; agent sends it; core stores and only triggers PCE when spec_hash changed (§13). |
| **4.4 Async risk** | **Implemented** | evaluatePodCapabilities invoked in a goroutine so sync does not block. |
| **5 Exposure resolver** | Not implemented | No pod_exposures table. |
| **6.1 pods** | Partially aligned | pods table has id, cluster_id, uid, name, namespace, node_name, phase, qos_class, pod_ip, owner_*, replica_set_name, service_account, created_at, updated_at, deleted_at; **spec_hash** added. risk_score/risk_* and hashes (security_hash, network_hash, config_hash) not added. |
| **6.2 containers** | Not implemented | No separate containers table; pods.containers JSONB. Planned for later phase. |
| **6.5 pod_exposures** | Not implemented | — |
| **6.6 pod_instances** | Exists (different shape) | `pod_instances` table: pod_uid, workload_id, namespace, name, generation, started_at, terminated_at, status. Used for lifecycle. |
| **7.x Multi-cluster** | cluster_id from payload | Rejecting cluster_id from body in favor of mTLS-derived identity is not implemented. |

---

# 13. Implemented Adjustments (Phase 1)

The following changes are implemented in code and tests.

## 13.1 spec_hash (Hash-based change detection)

- **Purpose:** Trigger PCE (and risk) only when pod spec actually changed, reducing N+1 and redundant evaluation.
- **Agent:** Computes a deterministic SHA256 hash of spec-relevant fields. **Canonicalization** (review fix): containers sorted by name, volumes by name, env (per container) by name, tolerations by key+value+effect before JSON marshal — so reordering does not change hash (avoids false-positive PCE triggers). Sent as `specHash` in PodPayload.
- **Core:** Column `pods.spec_hash` (migration 069). Stored on create/update. Trigger: `existing.SpecHash == "" || existing.SpecHash != pod.SpecHash` so old agent (empty) always triggers; new agent triggers only when hash changed.
- **Files:** Agent: `agent/internal/syncer/syncer.go` (SpecHash, computePodSpecHash with sort). Core: `core/pkg/models/models.go` (SpecHash), `core/internal/service/agent_service.go` (parse, store, compare), migration `069_add_pod_spec_hash.go`.

## 13.2 Decoupled and conditional PCE + race protection

- **Conditional:** PCE only when pod new, or `existing.SpecHash == ""` (backward compat), or `existing.SpecHash != pod.SpecHash`.
- **Async:** `evaluatePodCapabilities(clusterID, uid, specHash)` run in goroutine; sync does not block.
- **Race protection (review fix):** `EvaluateAndUpsertPod(ctx, db, pod, expectedSpecHash)`. If `expectedSpecHash` non-empty: (1) at start skip when `pod.SpecHash != expectedSpecHash`; (2) before writing re-read pod and discard result if `spec_hash` changed; (3) after success set `last_evaluated_hash = expectedSpecHash` only where `spec_hash = expectedSpecHash` (atomic). So late goroutine from sync #1 does not overwrite risk after sync #2 updated spec.
- **Restore:** Restore always enqueues PCE (risk may be stale); same race protection via `specHash` param.

## 13.3 last_evaluated_hash (review fix)

- **Column:** `pods.last_evaluated_hash` (migration 070). Set after PCE completes successfully for that spec. Enables: detect unevaluated pods, retry logic, debug. Risk in DB reflects `last_evaluated_hash`.

## 13.4 Deployment and migrations

- **Order:** 068 → 069 → 070. Core startup runs migrations in order; no extra deployment step.
- **Flow:** Deploy Core first (068, 069, 070). Then deploy Agent. Existing pods get `spec_hash` on next full sync.
- **Backward compatibility:** Logic `existing.SpecHash == "" || existing.SpecHash != pod.SpecHash` ensures agent without specHash still triggers PCE (no bypass). NULL/empty compare is explicit.
- **Unit tests:** Agent: `TestComputePodSpecHash` (stable, different spec, 64-char hex, **order_insensitivity_containers_and_volumes**). Core: `TestProcessSyncedPods_SpecHashStoredAndConditionalPCE`, **TestProcessSyncedPods_NullToNonNullSpecHash** (agent upgrade path), **TestEvaluateAndUpsertPod_SkipsWhenSpecHashMismatch** (race skip).

---

# 14. Phased Implementation Roadmap

| Phase | Item | Status |
|-------|------|--------|
| 1 | spec_hash + canonical hash (order-insensitive) | Done (§13) |
| 1 | PCE async + race protection (expectedSpecHash, last_evaluated_hash) | Done (§13) |
| 1 | Backward compat (existing.SpecHash == "" \|\| != incoming) | Done (§13) |
| 1 | Restore always PCE | Done (§13) |
| 2 | Containers table (normalized) | Planned |
| 2 | Informer-based sync (watch); rate-limit PCE queue | Planned |
| 3 | DTO layer for read API | Planned |
| 3 | Exposure resolution layer | Planned |
| 3 | Multi-cluster identity (mTLS cluster_id) | Planned |

---

# 15. Deployment and Migration Checklist

- **Core:** Deploy before or with Agent. On startup, Core runs all migrations in order (068, 069, 070). No manual SQL required.
- **Agent:** After Core has 069/070 applied, deploy Agent; sync payload will include `specHash`. Existing pods receive `spec_hash` on next full sync.
- **Consistency:** Pod upsert key remains `(cluster_id, uid)`. `spec_hash` optional; trigger uses `existing.SpecHash == "" || existing.SpecHash != pod.SpecHash` so old agent does not bypass PCE.
- **Tests:** `go test ./internal/service/... ./pkg/capability/... ./migrations/...` (core); `go test ./internal/syncer/...` (agent).

---

# 16. Review Fixes (Post–Phase 1)

Addresses from design review:

| Item | Fix |
|------|-----|
| **Hash design** | Canonical form: containers by name, volumes by name, env by name, tolerations deterministic order. Unit test: `order_insensitivity_containers_and_volumes`. |
| **Async PCE race** | Pass `spec_hash` into `evaluatePodCapabilities`; `EvaluateAndUpsertPod(..., expectedSpecHash)`: skip when mismatch at start, re-check before write, set `last_evaluated_hash` only when `spec_hash` still matches. Test: `TestEvaluateAndUpsertPod_SkipsWhenSpecHashMismatch`. |
| **Backward compat** | Trigger = `existing.SpecHash == "" \|\| existing.SpecHash != pod.SpecHash`. Agent old (no hash) still triggers PCE. Test: `TestProcessSyncedPods_NullToNonNullSpecHash`. |
| **Restore + PCE** | Restore always triggers PCE (risk may be stale); same async + specHash for race protection. |
| **last_evaluated_hash** | Migration 070; set after PCE success; risk reflects this spec; helps retry and debug. |
| **Unit tests** | Order insensitivity, race skip, NULL→non-NULL transition. |
| **Scale / 10k pods** | Full sync after migration can enqueue many PCE jobs. **Recommendation:** rate-limit worker queue, batch evaluation, configurable worker pool (Phase 2). |
| **Polling bottleneck** | spec_hash reduces compute but list API every 30s remains. Informer (watch) deferred to Phase 2; polling acknowledged as long-term bottleneck until then. |