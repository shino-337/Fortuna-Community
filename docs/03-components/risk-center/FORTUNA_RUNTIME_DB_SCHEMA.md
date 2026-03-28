# FORTUNA_RUNTIME_DB_SCHEMA.md

## Status
Draft v0.1

## Purpose
Mô tả schema Postgres (logical) cho các bảng cụ thể để triển khai runtime-aware risk engine theo target model.

Lưu ý:
- Fortuna hiện đang có `runtime_events`, `runtime_signals`, `pod_capabilities`, `insights`, `risk_scores`.
- Tài liệu này chỉ ra phần cần bổ sung/chỉnh để đạt được target layered architecture:
  runtime_events → runtime_behavior_facts → runtime_signals → runtime_incidents → capabilities → asset_security_state → insights/risk_scores.

---

## 0) Existing Baseline (hiện trạng)
### `runtime_events`
Model hiện có:
- `pod_uid` (not null)
- `namespace`, `node_name`
- `runtime`, `event_type`, `signal`, `mitre_technique`, `severity`
- `syscall` (not null), `target_path`
- `capability`
- `created_at`

### `runtime_signals`
Model hiện có:
- `pod_uid`, `signal_type`, `category`
- `confidence`, `evidence` (jsonb string), `count`
- `created_at`

### `pod_capabilities`
Model hiện có:
- `pod_uid`, `namespace`
- `capability_id`, `capability_group`, `severity`
- `state` detected/confirmed/exploited/chained
- `confidence`, `first_seen_at`, `last_seen_at`, `evidence` jsonb string

### `insights`
- Resource context (ResourceUID)
- InsightType/Severity/Title/Description/Recommendation…
- Evidence/ViolatedRules jsonb

### `risk_scores`
- ResourceType/ResourceUID/Namespace/ClusterID
- TotalScore/BaseScore/Exploitability/BusinessImpact/TimeDecay…
- Factors jsonb (v2)

---

## 1) Table: `runtime_events`
Mục tiêu: immutable raw runtime evidence, replayable, source-traceable.

### Columns (logical)
- `event_id` UUID (PK)
- `observed_at` timestamptz (when occurred on sensor, RFC3339 UTC)
- `ingested_at` timestamptz (server-side)
- `source_kind` varchar (falco|ebpf|tetragon|tracee|custom)
- `sensor_id` varchar
- `node_name` varchar
- `cluster_id` varchar
- `namespace` varchar
- `pod_uid` varchar NOT NULL (primary join key whenever resolvable)
- `pod_name` varchar
- `container_id` varchar (optional)
- `container_name` varchar (optional)
- `workload_kind` varchar (optional)
- `workload_name` varchar (optional)
- `resolution_state` varchar (resolved|partial|unresolved)
- `raw_rule` varchar (falco rule name)
- `raw_category` varchar (falco native grouping)
- `raw_severity` varchar
- `event_domain_hint` varchar (process|file|network|security…)
- `syscall` varchar
- `target` varchar
- `target_path` varchar
- `capability_hint` varchar (optional; for backward compat)
- `mitre_technique` varchar
- `severity` varchar
- `payload_hash` varchar (sha256 of normalized payload json)
- `payload_json` jsonb (optional / optional truncation policy)
- `created_at` timestamptz

### Indexes
- `(cluster_id, observed_at DESC)`
- `(pod_uid, observed_at DESC)`
- `(source_kind, observed_at DESC)`
- `(payload_hash)`
- optional: `(syscall, target_path)`

### Backward compatibility
- Map current fields:
  - current `runtime` -> `source_kind`
  - current `event_type/signal/mitre/severity/syscall/target_path/capability` -> canonical columns
- Tiếp tục hỗ trợ POST payload cũ bằng adapter.

---

## 2) Table: `runtime_behavior_facts`
Mục tiêu: bridge deterministic và reusable giữa raw runtime evidence và detector logic.

### Columns
- `fact_id` UUID (PK)
- `event_id` UUID NOT NULL (FK → runtime_events)
- `observed_at` timestamptz (copy from event)
- `pod_uid` varchar NOT NULL
- `container_id` varchar (optional)
- `fact_type` varchar NOT NULL (PROCESS_EXEC, FILE_READ, NETWORK_CONNECT…)
- `domain` varchar NOT NULL (execution|filesystem|network|credentials|evasion|recon|persistence…)
- `attributes` jsonb NOT NULL (exe/cmdline/path/dst_ip/dst_port…)
- `source_ref` jsonb (source_kind + raw_rule + raw identifiers)
- `created_at` timestamptz

### Indexes
- `(pod_uid, observed_at DESC)`
- `(fact_type, observed_at DESC)`
- GIN on `attributes` nếu cần query theo attributes keys.

---

## 3) Table: `runtime_signals`
Mục tiêu: semantic security layer, evidence-backed, taxonomy formalized.

### Columns
- `signal_id` UUID (PK)
- `pod_uid` varchar NOT NULL
- `signal_type` varchar NOT NULL
- `domain` varchar NOT NULL
- `severity_hint` varchar (low/medium/high/critical)
- `confidence` float (0..1 hoặc normalized)
- `first_seen_at` timestamptz
- `last_seen_at` timestamptz
- `count` int
- `evidence_refs` jsonb NOT NULL
  - ví dụ: `{ "event_ids": [..], "fact_ids": [..] }`
- `metadata` jsonb (mitre tactic/techniques snapshot, etc.)
- `created_at` timestamptz
- `updated_at` timestamptz

### Indexes
- `(pod_uid, signal_type, last_seen_at DESC)`
- `(domain, last_seen_at DESC)`
- `(signal_type)`

### Migration từ hiện trạng
- runtime_signals hiện tại dùng `created_at` + dedupe theo ngày.
- P0/P1 nên chuyển sang “rolling last_seen_at” (có thể triển khai song song: cột mới + logic update dần).

---

## 4) Table: `runtime_incidents`
Mục tiêu: stateful correlated higher-order runtime conclusions.

### Columns
- `incident_id` UUID (PK)
- `pod_uid` varchar NOT NULL
- `incident_type` varchar NOT NULL
- `severity_hint` varchar
- `confidence` float
- `first_seen_at` timestamptz
- `last_seen_at` timestamptz
- `window` interval or varchar (5m/1h…)
- `evidence_refs` jsonb NOT NULL
  - ví dụ: `{ "signal_ids": [...], "fact_ids": [...], "event_ids": [...] }`
- `metadata` jsonb
- `created_at` timestamptz
- `updated_at` timestamptz

### Indexes
- `(pod_uid, incident_type, last_seen_at DESC)`
- `(incident_type, last_seen_at DESC)`
- optional: severity/confidence indexes nếu UI filter theo.

---

## 5) Table: `capabilities`
Mục tiêu: capacity model 3 lớp: declared / observed / effective.

### Columns (logical)
- `capability_id` UUID (PK)
- `pod_uid` varchar NOT NULL
- `asset_ref` jsonb (pod_uid/container_id optional)
- `capability_type` varchar NOT NULL
  - ví dụ: CAN_REACH_K8S_API, OBSERVED_K8S_API_ACCESS, EFFECTIVE_K8S_CONTROL_PLANE_ACCESS
- `capability_class` varchar NOT NULL (declared|observed|effective)
- `state` varchar (active|suppressed|expired)
- `confidence` float
- `severity_hint` varchar
- `first_seen_at` timestamptz
- `last_seen_at` timestamptz
- `expires_at` timestamptz NULL
- `evidence_refs` jsonb (signal_ids/incidents/event_ids…)
- `derived_from` jsonb (declared vs observed dependencies)
- `metadata` jsonb
- `created_at`, `updated_at`

### Migration từ `pod_capabilities`
- map `pod_capabilities.CapabilityID` → capability_type
- map `state` detected/confirmed/... → confidence/state heuristics cho observed/effective
- dùng `capability_group`/`severity` hiện có làm proxy severity_hint và domain/category.

---

## 6) Table: `asset_security_state`
Mục tiêu: unified risk context snapshot cho từng asset (pod/workload).

### Columns
- `asset_id` UUID or varchar (unique key, ví dụ `pod:<pod_uid>`)
- `asset_type` varchar (pod, node, serviceaccount…)
- `cluster_id` varchar
- `identity` jsonb (namespace, pod_uid, workload, service_account…)
- `exposure` jsonb (internet_exposed, service_type, public_ingress…)
- `privilege` jsonb (run_as_root, host_network/host_pid/host_ipc, host_path_mount, linux_capabilities…)
- `software` jsonb (critical_vulns, fix_available_vulns…)
- `runtime` jsonb
  - recent_signals: [signal_type…]
  - recent_incidents: [incident_type…]
  - runtime_confidence
  - last_runtime_activity_at
- `capabilities` jsonb
  - declared/observed/effective lists
- `blast_radius` jsonb
- `freshness` jsonb
- `updated_at` timestamptz

### Indexes
- `(asset_type, updated_at DESC)`
- optional: GIN index trên runtime.capabilities lists nếu cần query coverage.

---

## 7) Table: `insights`
Hiện trạng đã có `insights` (Evidence/ViolatedRules dạng jsonb string).

Target:
- structured explainability:
  - `evidence_refs`, `signal_refs`, `incident_refs`, `capability_refs`
- insight lifecycle: open/ack/suppressed/resolved/reopened (docs)

### Columns add-on (logical)
- `evidence_refs` jsonb
- `signal_refs` jsonb
- `incident_refs` jsonb
- `capability_refs` jsonb
- `score_factors` jsonb

---

## 8) Table: `risk_scores`
Hiện đã có fields đủ cho V2.

Target:
- factorized + explainable score:
  - exposure/exploitability/privilege/confidence/decay as dimensions
- add freshness-aware updates

### Add-on (logical)
- `score_factors` jsonb (normalized dims)
- `confidence` dimension (để tách khỏi severity)
- `last_updated_at`/`computed_at` consistency

