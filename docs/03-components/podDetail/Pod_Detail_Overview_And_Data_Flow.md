# Pod Detail – Tổng quan tính năng và luồng dữ liệu

**Mục đích:** Một tài liệu tham chiếu nhanh cho toàn bộ tính năng Pod Detail: thu thập, lưu trữ, API, và load lên Dashboard.

---

## 1. Tính năng Pod Detail (4 nhóm dữ liệu)

| Nhóm | Mô tả | Nguồn thu thập | Bảng DB |
|------|--------|----------------|---------|
| **Runtime metrics** | CPU/memory (millicore, bytes), restart count, state (Running/Waiting/Terminated) theo container | Agent: `pod.Status.ContainerStatuses` | `pod_runtime_metrics` |
| **Processes** | PID, PPID, user, %CPU, %MEM, command, binary_path theo container | Agent: **exec** `ps -eo ...` hoặc **host** `/proc` + cgroup (khi `POD_DETAIL_RUNTIME_SOURCE=host`) | `pod_processes` |
| **Network connections** | source/dest IP:port, protocol, state (LISTEN, ESTABLISHED, ...) theo container | Agent: **exec** `ss`/`netstat` hoặc **host** `/proc/<pid>/net/tcp,udp` | `pod_network_connections` |
| **K8s Events** | Events (Warning/Normal) có `involved_uid` = pod UID | Agent: SharedInformer `core/v1.Event`, batch POST | `k8s_events` |

- **Runtime Source:** Khi `POD_DETAIL_RUNTIME_SOURCE=host` (hoặc `auto` và /host/proc khả dụng), process và network lấy từ **host** (/proc, cgroup), không exec vào container. Cột `runtime_source` ('host' | 'exec') lưu trong DB; Dashboard hiển thị badge "Runtime: Host Inspection" | "Container Exec".

---

## 2. Luồng từ Agent → Core → DB → Dashboard

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ AGENT (DaemonSet, mỗi node)                                                 │
│  - listPodsOnNode() → pods trên node                                        │
│  - Với mỗi pod: sendRuntimeMetrics (từ pod.Status)                          │
│  - Process: CollectProcessesFromPod (exec) HOẶC CollectProcessesFromHost    │
│  - Network: CollectNetworkFromPod (exec) HOẶC CollectNetworkFromHost         │
│  - Events: EventsCollector (informer) → batch POST                          │
│  POST /api/v1/agent/pod-runtime-metrics | pod-processes | pod-network-...    │
└─────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ CORE (API)                                                                   │
│  - IngestPodRuntimeMetricsPayload / IngestPodProcessesPayload / ...          │
│  - Validate podUid (reject "" và "0")                                        │
│  - Set pod_uid, cluster_id, namespace, observed_at; optional runtimeSource  │
│  - Encrypt command/binary_path nếu POD_DETAIL_ENCRYPTION_KEY set             │
│  - dbIngestWithRetry → CreateInBatches                                      │
│  - BroadcastPodDetailUpdate(podUid, "metrics"|"processes"|"network"|"events")│
└─────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ DB (PostgreSQL)                                                              │
│  - pod_runtime_metrics   (last_observed_at, limit 100 khi GET)               │
│  - pod_processes         (observed_at, retention 30 ngày, decrypt khi GET) │
│  - pod_network_connections (observed_at)                                    │
│  - k8s_events            (event_uid unique per cluster)                      │
│  Khi pod bị xóa (sync): Core xóa rows theo pod_uid trong 4 bảng trên.        │
└─────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ DASHBOARD (PodDetail page)                                                   │
│  - getPod(id) hoặc getPodByUid(uid) → Pod header                             │
│  - getPodRuntimeMetrics(idOrUid) → tab Runtime metrics                        │
│  - getPodProcesses(idOrUid) → tab Processes (+ badge Runtime Source)        │
│  - getPodNetworkConnections(idOrUid) → tab Network (+ badge Runtime Source)   │
│  - getPodEvents(idOrUid) → tab Events                                        │
│  - WebSocket getPodDetailWsUrl(uid): subscribe → on message refetch 4 API    │
│  idOrUid = pod.id ?? pod.uid (số id hoặc chuỗi uid đều được).                │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 3. Lưu trữ và giới hạn

| Bảng | Lưu history? | Retention / limit | Ghi chú |
|------|--------------|--------------------|---------|
| pod_runtime_metrics | Có | GET limit 100 mới nhất | Không TTL xóa |
| pod_processes | Có (nhiều snapshot) | 30 ngày (POD_PROCESS_RETENTION_DAYS), job xóa; field truncate 1024/512/128 | GET limit 1000, snapshot mới nhất |
| pod_network_connections | Có | GET limit 500 | Xóa theo pod khi pod sync mất |
| k8s_events | Có | Unique (cluster_id, event_uid) OnConflict DoNothing | — |

- **Encryption:** `command`, `binary_path` encrypt at-rest khi `POD_DETAIL_ENCRYPTION_KEY` set; GET decrypt trước khi trả về.
- **Chi tiết:** `docs/02-architecture/pod-detail-storage-and-cleanup.md`.

---

## 4. API Endpoints (Core)

**Quy ước:** Lấy pod theo primary key (từ list): `GET /api/v1/pods/:id`. Mọi dữ liệu theo pod (metrics, processes, events, SBOM, risks, …) dùng **pod UID**: `GET /api/v1/pods/:podUid/...`.

| Method | Path | Mô tả |
|--------|------|--------|
| GET | `/api/v1/pods/:id` | Pod theo primary key (số; dùng khi navigate từ list) |
| GET | `/api/v1/pods/:podUid` | Pod theo UID (K8s uid) |
| GET | `/api/v1/pods/:podUid/runtime-metrics` | Metrics |
| GET | `/api/v1/pods/:podUid/processes` | Process list (có runtimeSource) |
| GET | `/api/v1/pods/:podUid/network-connections` | Network |
| GET | `/api/v1/pods/:podUid/events` | K8s events |
| GET | `/api/v1/pods/:podUid/spec` | Pod spec YAML |
| GET | `/api/v1/pods/:podUid/capabilities` | Pod capabilities |
| POST | `/api/v1/agent/pod-runtime-metrics` | Ingest metrics (Agent gửi podUid) |
| POST | `/api/v1/agent/pod-processes` | Ingest processes |
| POST | `/api/v1/agent/pod-network-connections` | Ingest network |
| POST | `/api/v1/agent/pod-events` | Ingest events |
| GET | `/api/v1/ws/pod/:uid` | WebSocket live updates |

---

## 5. Dashboard load dữ liệu

- **Khi mở Pod Detail:** `fetchPod()` (getPod(id) hoặc getPodByUid(uid) tùy URL) → set pod. Sau khi có pod, mọi API pod-scoped dùng **pod.uid**: preload metrics, processes, network, events qua GET /pods/:podUid/...
- **Khi đổi tab:** `fetchTabData(activeTab)` load SBOM, risks, processes, network, events, spec (tất cả gọi với pod.uid).
- **WebSocket:** Khi có message (type metrics|processes|network|events), refetch đúng API tương ứng để cập nhật UI realtime sau ingest.
- **Empty state:** Nếu `items.length === 0`, hiển thị message "No process data" / "No runtime metrics" / ... (không mock).

---

## 6. Clean / Rebuild / Redeploy (không xóa CVE)

- **Clean host (giảm tải, tránh đầy):** Chỉ xóa image Fortuna, prune containerd, temp junk. **Không** chạy SQL lên DB.
  - Script: `scripts/clean/clean-host-images-and-junk.sh` (standalone) hoặc Phase 1 của `full-clean-database-rebuild-deploy.sh` **không** truyền `--db`.
- **Rebuild:** `build-and-load-containerd.sh` (core, agent, dashboard).
- **Redeploy:** `deploy-fortuna-robust.sh` + rollout restart.
- **Lệnh đầy đủ (không đụng DB, giữ CVE):**
  ```bash
  ./scripts/pipeline/full-clean-database-rebuild-deploy.sh
  ```
  **Không** dùng `--db` hay `--db-reset` → CVE và mọi dữ liệu DB giữ nguyên.
