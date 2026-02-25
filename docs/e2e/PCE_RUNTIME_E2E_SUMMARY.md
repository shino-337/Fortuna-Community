# E2E: Privileged Pod + PCE/Runtime Signals + Risk Center

*Thực hiện: deploy pod privileged, exec lệnh vi phạm, mô phỏng runtime events, theo dõi agent/core và Risk Center.*

---

## 1. Pod đã deploy

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: cve-2025-31133-pod
spec:
  containers:
  - name: test
    image: alpine:latest
    command: ["sh", "-c", "sleep 3600"]
    securityContext:
      privileged: true
```

- **Namespace:** default  
- **Node:** k8s-worker01  
- **Pod UID:** `c43da190-f581-4204-ba13-75a5831a18f8`  
- **File:** `deploy/e2e/cve-2025-31133-pod.yaml`  
- Apply: `kubectl apply -f deploy/e2e/cve-2025-31133-pod.yaml`

---

## 2. Lệnh exec “vi phạm PCE” đã chạy trong pod

| Lệnh | Mục đích |
|------|----------|
| `id; whoami` | Root trong container (privileged). |
| `cat /proc/1/root/etc/passwd` | Đọc host `/etc/passwd` qua `/proc/1/root` (container escape). |
| `ls -la /proc/1/root/etc/shadow` | Truy cập host `/etc/shadow`. |
| `mount \| head -5; ls /sys/kernel` | Thông tin mount và kernel (privileged). |

Các hành vi này tương ứng với **PROC_ROOT_PIVOT** (truy cập `/proc/1/root`) và **FS_ESCAPE_ATTEMPT** (mount/sys) trong PCE/runtime.

---

## 3. Runtime events gửi tới Core (mô phỏng)

Runtime signals thực tế đến từ **agent** khi bật `RUNTIME_EVENTS_ENABLED=true` và có nguồn event (vd: file log từ Falco/tracee). Ở đây đã **mô phỏng** bằng cách POST trực tiếp tới Core:

- **Endpoint:** `POST /api/v1/runtime-events` (cần JWT).
- **Payload ví dụ (PROC_ROOT_PIVOT):**  
  `syscall=openat`, `target_path=/proc/1/root`, `pod_uid=c43da190-f581-4204-ba13-75a5831a18f8`.
- **Payload ví dụ (FS_ESCAPE_ATTEMPT):**  
  `syscall=mount`, `target_path=/proc`, cùng `pod_uid`.

Kết quả sau khi POST:

- **runtime_events:** bản ghi raw được ghi vào DB.
- **runtime_signals:** Core (REP) map event → signal (PROC_ROOT_PIVOT, FS_ESCAPE_ATTEMPT, UNKNOWN), ghi vào bảng `runtime_signals`.
- **Pod UID** `c43da190-f581-4204-ba13-75a5831a18f8` có **3 signals:**  
  1x PROC_ROOT_PIVOT (ESCAPE), 1x FS_ESCAPE_ATTEMPT (ESCAPE), 1x UNKNOWN.

---

## 4. Agent và Core

- **Agent (fortuna-agent trên k8s-worker01):**  
  Heartbeat OK; Sync có lúc trả 500. Agent **không** gửi runtime events nếu chưa bật `RUNTIME_EVENTS_ENABLED` và chưa có file event.
- **Core:**  
  - Nhận `POST /api/v1/runtime-events` → `ProcessRuntimeEvent` → ghi `runtime_events`, tạo `runtime_signals` (SignalAdapter), có thể promote capability (CSC).  
  - API: `GET /api/v1/runtime-signals`, `GET /api/v1/runtime-signals/pods/:podUid` trả đúng signals của pod test.

Luồng chi tiết: `docs/03-components/RUNTIME_SIGNALS_FLOW.md`.

---

## 5. Risk Center (Dashboard) – Runtime trên UI

- **Trang:** **Risk Center** (Insights) → tab **Reference**.
- **Khối “Runtime Signals”:**  
  Component `RuntimeSignalsTable` gọi `api.getRuntimeSignals()` → `GET /api/v1/runtime-signals?limit=20`.
- **Nội dung hiển thị:**  
  Bảng runtime signals: Pod UID, Signal Type (vd: PROC_ROOT_PIVOT, FS_ESCAPE_ATTEMPT), Category (ESCAPE), Confidence, Evidence, Created At.
- **Pod test:**  
  Pod `c43da190-f581-4204-ba13-75a5831a18f8` (cve-2025-31133-pod) sẽ có 3 dòng tương ứng 3 signals trên.

**Cách xem nhanh:**

1. Port-forward Core (nếu chưa):  
   `kubectl port-forward -n fortuna svc/fortuna-core 8080:8080`
2. Port-forward Dashboard:  
   `kubectl port-forward -n fortuna svc/fortuna-dashboard 8081:80`
3. Mở Dashboard → login → **Risk Center** → tab **Reference** → xem bảng **Runtime Signals**.

---

## 6. API kiểm tra nhanh (có JWT)

```bash
# Login
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}' | jq -r .token)

# List runtime signals
curl -s -H "Authorization: Bearer $TOKEN" "http://localhost:8080/api/v1/runtime-signals?limit=10" | jq .

# Signals cho pod test
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/runtime-signals/pods/c43da190-f581-4204-ba13-75a5831a18f8" | jq .
```

---

## 7. Tóm tắt

| Bước | Trạng thái |
|------|------------|
| Deploy pod `cve-2025-31133-pod` (privileged) | ✅ Pod Running (default, k8s-worker01). |
| Exec vào pod, chạy lệnh “vi phạm” (host fs, mount, sys) | ✅ Đã chạy. |
| Gửi runtime events tới Core (POST /runtime-events) | ✅ 3 events → 3 runtime_signals (PROC_ROOT_PIVOT, FS_ESCAPE_ATTEMPT, UNKNOWN). |
| Core xử lý (REP, SignalAdapter, DB) | ✅ runtime_events + runtime_signals có bản ghi. |
| Risk Center → Reference → Runtime Signals | ✅ Bảng gọi GET /runtime-signals, hiển thị runtime signals (trong đó có pod test). |

**Lưu ý:** Trong môi trường thật, runtime events thường do **agent** đọc từ file/sensor (vd: Falco) và gửi lên Core khi bật `RUNTIME_EVENTS_ENABLED` và cấu hình `RUNTIME_EVENTS_PATH` tương ứng.
