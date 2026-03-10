# Pod Detail – Six Services Design (Agent, Core, Database)

Version: 1.0  
Scope: Logic & flow cho Pod Service, Runtime Service, Process Service, Security Service, Network Service, Event Service.

---

## 1. Tổng quan

| Component         | Responsibility                      | Hiện trạng | Bổ sung |
|------------------|--------------------------------------|------------|--------|
| **Pod Service**  | Sync static pod spec                 | ✅ Có (syncer → pods) | Mở rộng spec (probes, limits) nếu cần |
| **Runtime Service** | Sync CPU/memory/runtime state     | ⚠️ Một phần (runtime_events/signals) | Bảng `pod_runtime_metrics`, agent thu thập metrics |
| **Process Service** | Aggregate runtime process info   | ❌ Chưa có | Bảng `pod_processes`, agent gửi process list |
| **Security Service** | Risk scoring + detection logic   | ✅ Có (PCE, insights, pod_risk_profiles) | Chuẩn hóa API pod detail → security summary |
| **Network Service** | Track pod network connections    | ❌ Chưa có | Bảng `pod_network_connections`, agent thu thập |
| **Event Service** | Persist K8s events                  | ❌ Chưa có (chỉ runtime_events probe) | Bảng `k8s_events`, agent watch Events |

---

## 2. Pod Service (Sync static pod spec)

### 2.1 Responsibility
- Đồng bộ spec tĩnh của pod từ cluster vào Core: containers, volumes, security context, phase, podIP, startTime, restartCount, owner*, qosClass, specHash.

### 2.2 Hiện trạng
- **Agent:** `agent/internal/syncer/syncer.go` – full sync định kỳ (SYNC_INTERVAL), payload gửi pod list + spec (PodPayload với SpecHash, PodIP, StartTime, RestartCount, OwnerKind, OwnerName, ReplicaSetName, QoSClass).
- **Core:** `core/internal/service/agent_service.go` – `ProcessSyncedPods`: upsert `pods`, trigger PCE khi spec_hash thay đổi, restore soft-delete.
- **DB:** `pods` (migration 068, 069, 070: pod_ip, start_time, restart_count, owner_*, qos_class, spec_hash, last_evaluated_hash).

### 2.3 Flow
```
[Cluster] --> Agent (list pods per namespace)
                --> HTTP POST /api/v1/agent/sync
                     --> Core: ProcessSyncedPods
                          --> pods upsert, pod_instances EnsureActive
                          --> (spec_hash changed) --> PCE async
```

### 2.4 Bổ sung (tùy chọn)
- Mở rộng PodPayload/Core để lưu thêm: resource limits/requests (containers), liveness/readiness probe summary (nếu cần cho UI Configuration tab). Có thể lưu trong `pods.containers` JSONB hiện tại.

---

## 3. Runtime Service (CPU/memory/runtime state)

### 3.1 Responsibility
- Đồng bộ trạng thái runtime: CPU usage, memory usage, container state (Running/Waiting/Terminated), restart count per container.

### 3.2 Hiện trạng
- `runtime_events`: raw probe events (syscall, capability).
- `runtime_signals`: semantic signals từ events.
- Chưa có bảng lưu CPU/memory hoặc container runtime state.

### 3.3 Thiết kế bổ sung

**Database**
- Bảng `pod_runtime_metrics`:
  - `id`, `pod_uid`, `cluster_id`, `namespace`, `container_name`, `cpu_usage_millicore`, `memory_usage_bytes`, `memory_limit_bytes`, `restart_count`, `state` (Running/Waiting/Terminated), `last_observed_at`, `created_at`, `updated_at`.
- Unique (pod_uid, container_name) hoặc time-series (partition by pod_uid, container_name, bucket 1min/5min).

**Agent**
- Thu thập từ K8s metrics (cAdvisor / metrics-server): container CPU, memory.
- Hoặc đọc từ node: `/sys/fs/cgroup/...` hoặc metrics API.
- Payload: `POST /api/v1/pod-runtime-metrics` với mảng theo pod_uid + container.

**Core**
- Handler `PostPodRuntimeMetrics` upsert `pod_runtime_metrics` (theo pod_uid + container_name).
- API đọc: `GET /api/v1/pods/:id/runtime-metrics` hoặc `GET /api/v1/pods/by-uid/:uid/runtime-metrics` trả về list container metrics.

### 3.4 Flow
```
[Node/cAdvisor or metrics-server]
    --> Agent (periodic scrape container stats)
         --> POST /api/v1/pod-runtime-metrics
              --> Core: upsert pod_runtime_metrics
Dashboard Pod Detail (Runtime tab)
    --> GET /api/v1/pods/:id/runtime-metrics
```

---

## 4. Process Service (Runtime process info)

### 4.1 Responsibility
- Tổng hợp thông tin process trong pod: PID, PPID, user, CPU%, MEM%, command, container.

### 4.2 Hiện trạng
- Chưa có bảng hay API process.

### 4.3 Thiết kế bổ sung

**Database**
- Bảng `pod_processes`:
  - `id`, `pod_uid`, `cluster_id`, `namespace`, `container_name`, `pid`, `ppid`, `user`, `cpu_percent`, `memory_percent`, `command`, `binary_path`, `started_at`, `observed_at`, `created_at`.
- Index: `pod_uid`, `observed_at` (để lấy snapshot mới nhất).

**Agent**
- Trên node, với mỗi pod local: exec vào container (hoặc đọc từ host PID namespace nếu hostPID) để lấy process list (e.g. `ps` hoặc đọc `/proc`).
- Gửi định kỳ: `POST /api/v1/pod-processes` với body `{ "podUid": "...", "processes": [ ... ] }`.

**Core**
- Handler `PostPodProcesses`: replace snapshot cho pod_uid (xóa cũ theo pod_uid, insert batch mới) hoặc append với observed_at.
- API đọc: `GET /api/v1/pods/:id/processes` hoặc `GET /api/v1/pods/by-uid/:uid/processes` (filter theo observed_at gần nhất hoặc time range).

### 4.4 Flow
```
[Node] Agent (exec/read process list per pod)
    --> POST /api/v1/pod-processes
         --> Core: replace/append pod_processes
Dashboard (Runtime Process tab)
    --> GET /api/v1/pods/:id/processes
```

---

## 5. Security Service (Risk scoring + detection logic)

### 5.1 Responsibility
- Risk scoring, detection logic, PCE (Pod Capability Engine), insights.

### 5.2 Hiện trạng
- **Core:** PCE (`core/pkg/capability/evaluator.go`), insights (`core/pkg/riskengine`, insights_handlers), `pod_risk_profiles`, `pod_capabilities`.
- **API:** `/api/v1/risks/pods/:podUid/report`, `/api/v1/pods/:id/capabilities`, insights by pod.

### 5.3 Flow (giữ nguyên, có thể chuẩn hóa)
- PCE chạy khi spec_hash thay đổi hoặc restore; ghi `pod_capabilities`, `pod_risk_profiles`, insights.
- Pod Detail gọi:
  - `GET /api/v1/pods/:id` (đã có riskCount).
  - `GET /api/v1/risks/pods/:uid/report` (Related Risks).
  - `GET /api/v1/pods/:uid/capabilities` (capabilities list).

### 5.4 Bổ sung (tùy chọn)
- Một endpoint tổng hợp: `GET /api/v1/pods/:id/security-summary` trả về risk score, capability count, insight count, last_evaluated_hash để UI không gọi nhiều endpoint.

---

## 6. Network Service (Pod network connections)

### 6.1 Responsibility
- Theo dõi kết nối mạng của pod: source, destination, port, protocol, state.

### 6.2 Hiện trạng
- Chưa có bảng hay API.

### 6.3 Thiết kế bổ sung

**Database**
- Bảng `pod_network_connections`:
  - `id`, `pod_uid`, `cluster_id`, `namespace`, `container_name`, `source_ip`, `source_port`, `dest_ip`, `dest_port`, `protocol`, `state`, `bytes_sent`, `bytes_recv`, `observed_at`, `created_at`.
- Index: `pod_uid`, `observed_at`.

**Agent**
- Đọc từ node: `ss`/`netstat` hoặc `/proc/net/tcp`, `/proc/net/udp` map PID → container (cần mapping từ network namespace hoặc cgroup).
- Payload: `POST /api/v1/pod-network-connections` với `podUid` + danh sách connection.

**Core**
- Handler `PostPodNetworkConnections`: replace snapshot theo pod_uid (hoặc append theo observed_at).
- API: `GET /api/v1/pods/:id/network-connections` hoặc `GET /api/v1/pods/by-uid/:uid/network-connections`.

### 6.4 Flow
```
[Node] Agent (read connections per pod/container)
    --> POST /api/v1/pod-network-connections
         --> Core: upsert pod_network_connections
Dashboard (Network tab)
    --> GET /api/v1/pods/:id/network-connections
```

---

## 7. Event Service (Persist K8s events)

### 7.1 Responsibility
- Lưu Kubernetes Events (Warning, Normal) liên quan pod/deployment/node để hiển thị trong tab Events.

### 7.2 Hiện trạng
- `runtime_events`: chỉ event từ runtime probe (syscall), không phải K8s Events.

### 7.3 Thiết kế bổ sung

**Database**
- Bảng `k8s_events`:
  - `id`, `cluster_id`, `namespace`, `name`, `uid`, `involved_kind` (Pod/Deployment/Node/...), `involved_uid`, `involved_name`, `reason`, `message`, `type` (Warning/Normal), `count`, `first_timestamp`, `last_timestamp`, `created_at`.
- Index: `cluster_id`, `involved_kind`, `involved_uid`, `last_timestamp`.

**Agent**
- Watch K8s API: `watch core/v1/events` (filter theo namespace hoặc fieldSelector involvedObject.uid=pod-uid).
- Gửi batch: `POST /api/v1/k8s-events` với danh sách events (hoặc từng event).

**Core**
- Handler `PostK8sEvents`: upsert theo (cluster_id, uid) của event (K8s event có UID) hoặc insert mới.
- API: `GET /api/v1/pods/:id/events` hoặc `GET /api/v1/events?involvedUid=:podUid` (events có involvedObject.uid = pod UID).

### 7.4 Flow
```
[Cluster] K8s API (watch events)
    --> Agent (watch Events, filter by namespace or involvedObject)
         --> POST /api/v1/k8s-events
              --> Core: upsert k8s_events
Dashboard (Events tab)
    --> GET /api/v1/pods/:id/events
```

---

## 8. Thứ tự triển khai gợi ý

1. **Database:** Migration cho `pod_runtime_metrics`, `pod_processes`, `pod_network_connections`, `k8s_events` (có thể 1 migration tổng hoặc tách file).
2. **Core:** Model structs + handlers POST (ingest) + GET (cho Pod Detail).
3. **Agent:** Collector/watcher cho từng loại dữ liệu (runtime metrics, process list, network, k8s events); gửi HTTP POST tới Core.
4. **Dashboard:** Tab Runtime / Process / Network / Events gọi API tương ứng.

---

## 9. Tóm tắt API mới (Core)

| Method | Path | Mô tả |
|--------|------|--------|
| POST | /api/v1/pod-runtime-metrics | Agent gửi CPU/memory/container state |
| GET  | /api/v1/pods/:id/runtime-metrics | Lấy metrics cho pod (detail) |
| POST | /api/v1/pod-processes | Agent gửi process snapshot |
| GET  | /api/v1/pods/:id/processes | Lấy process list cho pod |
| POST | /api/v1/pod-network-connections | Agent gửi connection snapshot |
| GET  | /api/v1/pods/:id/network-connections | Lấy connections cho pod |
| POST | /api/v1/k8s-events | Agent gửi K8s events |
| GET  | /api/v1/pods/:id/events | Lấy K8s events liên quan pod |

(Có thể dùng `/api/v1/pods/by-uid/:uid/...` thay cho `:id` cho các GET nếu ưu tiên tra cứu theo UID.)

---

## 10. Agent flow (triển khai gợi ý)

| Service   | Agent component gợi ý | Nguồn dữ liệu | Gửi tới Core |
|-----------|------------------------|----------------|----------------|
| Runtime   | Metrics collector      | cAdvisor / metrics-server / cgroup | POST /api/v1/pod-runtime-metrics |
| Process   | Process scraper         | exec vào container (`ps`) hoặc host /proc khi hostPID | POST /api/v1/pod-processes |
| Network   | Connection scraper      | `ss`/`netstat` hoặc /proc/net, map PID→container | POST /api/v1/pod-network-connections |
| Event     | K8s Events watcher      | watch core/v1/events (involvedObject.uid = pod) | POST /api/v1/k8s-events |

- **Pod Service:** Đã có – `agent/internal/syncer/syncer.go` (full sync pods).
- **Security Service:** Core PCE + insights; agent chỉ cần gửi spec (đã có) và runtime probe events (đã có `runtime/events_reader.go` → POST /runtime-events).

Cấu trúc agent có thể thêm:
- `agent/internal/metrics/` – thu thập container CPU/memory (định kỳ), gọi POST /pod-runtime-metrics.
- `agent/internal/process/` – list process theo pod (định kỳ hoặc on-demand), gọi POST /pod-processes.
- `agent/internal/network/` – list connections theo pod, gọi POST /pod-network-connections.
- `agent/internal/events/` – watch K8s Events API, gọi POST /k8s-events.

---

## 11. Tài liệu tham chiếu

- Pod sync & spec: `docs/03-components/podDetail/POD_SYNC_ARCHITECTURE_AND_DATA_MODEL_SPEC.md`
- Pod Detail UI/UX: `docs/03-components/podDetail/podDetail_page_UIUX.md`
- Runtime events (probe): `core/internal/api/runtime_event_handlers.go`, `core/pkg/rep/processor.go`
- Migration 071: `core/migrations/071_add_pod_detail_services_tables.go`
- Handlers: `core/internal/api/pod_detail_services_handlers.go`
