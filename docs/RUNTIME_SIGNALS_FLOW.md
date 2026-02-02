# Runtime Signals – Luồng xử lý và kiểm tra

*Cập nhật: 2026-02-01*

---

## 1. Tổng quan luồng

```
[Agent]  RUNTIME_EVENTS_ENABLED=true
         RUNTIME_EVENTS_PATH=/var/log/fortuna/runtime-events.log
         RUNTIME_EVENTS_POLL=5s
              │
              │ Đọc file (JSON lines), poll mỗi 5s
              ▼
         POST /api/v1/runtime-events  (HTTP → Core)
              │
[Core]   PostRuntimeEvents handler
              │
              ├─► runtime_events (raw)  INSERT
              │
              ├─► rep.ProcessRuntimeEvent()
              │      ├─► RuntimeEvent CREATE
              │      ├─► SignalAdapter.AdaptAndPersist() → runtime_signals INSERT/UPDATE (dedup pod_uid+signal_type+day)
              │      ├─► classifySignal → promotion
              │      └─► CSC.PromoteCapability (pod_capabilities)
              │
              ▼
[API]    GET /api/v1/runtime-signals           → từ bảng runtime_signals
         GET /api/v1/runtime-signals/pods/:uid  → từ bảng runtime_signals
```

---

## 2. Thành phần chi tiết

### 2.1 Agent

| Thành phần | File | Mô tả |
|------------|------|--------|
| Reader | `agent/internal/runtime/events_reader.go` | Đọc file JSON lines tại `RUNTIME_EVENTS_PATH`, mỗi `RUNTIME_EVENTS_POLL` (mặc định 5s), gửi `POST /api/v1/runtime-events` tới Core. |
| Config | `agent/internal/config/config.go` | `RuntimeEventsEnabled` (mặc định false), `RuntimeEventsPath`, `RuntimeEventsPoll`. |
| Main | `agent/cmd/main.go` | Chỉ start Reader khi `cfg.RuntimeEventsEnabled == true`. |

**Định dạng event (JSON line trong file):**

```json
{
  "event_type": "escape_attempt",
  "mitre_technique": "T1611.001",
  "signal": "PROC_ROOT_PIVOT",
  "severity": "high",
  "pod": { "name": "my-pod", "namespace": "default", "uid": "abc-123" },
  "syscall": "openat",
  "target": "/proc/1/root",
  "timestamp": 1706700000
}
```

**Deploy:** DaemonSet fortuna-agent không set `RUNTIME_EVENTS_ENABLED`/`RUNTIME_EVENTS_PATH` mặc định → Reader tắt. Bật bằng env:

- `RUNTIME_EVENTS_ENABLED=true`
- `RUNTIME_EVENTS_PATH=/var/log/fortuna/runtime-events.log` (và mount volume nếu cần)

### 2.2 Core

| Thành phần | File | Mô tả |
|------------|------|--------|
| Ingest | `core/internal/api/runtime_event_handlers.go` | `POST /api/v1/runtime-events` nhận array payload, map sang `RuntimeEventInput`, gọi `rep.ProcessRuntimeEvent`. |
| REP | `core/pkg/rep/processor.go` | Tạo `RuntimeEvent` (raw) → `SignalAdapter.AdaptAndPersist` (ghi `runtime_signals`) → classifySignal → CSC.PromoteCapability. |
| Adapter | `core/pkg/rep/signal_adapter.go` | `AdaptEvent`: map syscall/target → signal type (PROC_ROOT_PIVOT, FS_ESCAPE_ATTEMPT, NAMESPACE_ESCAPE, CAPABILITY_MISUSE). Dedup: cùng pod_uid + signal_type + ngày thì update confidence nếu cao hơn. |
| API read | `core/internal/api/runtime_signals_handlers.go` | `GET /runtime-signals` (filter podUid, signalType, category, date, limit/offset), `GET /runtime-signals/pods/:podUid`. |

**Bảng DB:**

- `runtime_events`: raw event (pod_uid, namespace, syscall, target_path, capability, created_at).
- `runtime_signals`: semantic signal (pod_uid, signal_type, category, confidence, evidence JSONB, created_at).

### 2.3 Dashboard

- Risk Center → tab Reference → **Runtime Signals**: component `RuntimeSignalsTable` gọi `api.getRuntimeSignals(params)`.

---

## 3. Test cases thực tế

| # | Test case | Cách kiểm tra |
|---|-----------|----------------|
| 1 | Core nhận event và ghi raw | POST /api/v1/runtime-events với payload hợp lệ (pod_uid, syscall, target) → 200; query `SELECT COUNT(*) FROM runtime_events` tăng. |
| 2 | Core tạo signal từ event | Sau POST, query `SELECT * FROM runtime_signals ORDER BY created_at DESC LIMIT 5` có bản ghi mới; signal_type PROC_ROOT_PIVOT / FS_ESCAPE_ATTEMPT / … theo syscall/target. |
| 3 | API list signals | GET /api/v1/runtime-signals?limit=10 (có Authorization) → 200, body có `signals`, `total`. |
| 4 | API signals theo pod | Lấy pod_uid từ bảng runtime_signals; GET /api/v1/runtime-signals/pods/{pod_uid} → 200, `signals` khớp pod. |
| 5 | Agent gửi event (E2E) | Bật RUNTIME_EVENTS_ENABLED, mount file hoặc exec echo JSON vào path, đợi poll → Core log “[RuntimeEvent] Ingesting”; runtime_events/runtime_signals có bản ghi mới. |
| 6 | Dedup cùng ngày | POST 2 event cùng pod_uid + syscall/target (cùng signal_type) trong cùng ngày → runtime_signals chỉ 1 row (hoặc update confidence). |

---

## 4. Clean / Rebuild / Redeploy và xác nhận luồng

1. **Clean + Rebuild + Redeploy:**  
   - `scripts/full-clean-rebuild-redeploy.sh` [--skip-clean] [--skip-rebuild] [--skip-deploy] [--db]  
   - Hoặc `scripts/clean-rebuild-redeploy-and-test.sh` (gồm clean/rebuild/deploy + test, trong đó có **test-runtime-signals-e2e**).
2. **Sau khi Core/Agent chạy:**
   - **Test E2E Runtime Signals:** `./scripts/test-runtime-signals-e2e.sh`  
     (POST /runtime-events → kiểm tra DB runtime_events/runtime_signals → GET /runtime-signals và /runtime-signals/pods/:uid)
   - **Monitor luồng:** `./scripts/monitor-runtime-signals.sh` (snapshot DB + API + log Agent/Core); `./scripts/monitor-runtime-signals.sh --follow` để tail log.
3. **Bật Runtime Events trên Agent (tùy chọn):** Sửa DaemonSet thêm env `RUNTIME_EVENTS_ENABLED=true` và volume cho `RUNTIME_EVENTS_PATH` nếu cần.

---

## 5. Log cần xem khi debug

| Nơi | Log mẫu |
|-----|--------|
| Agent | `[RuntimeEvents] ...` (read/send), `✅ Runtime events reader enabled (path=... poll=...)` |
| Core | `[RuntimeEvent] Ingesting event: pod_uid=... syscall=... target=...`, `[RuntimeEvent] ✅ Processed: ... signal=...`, `[RuntimeEvent] ❌ Failed ...`, `[REP] Failed to adapt event to signal` |
