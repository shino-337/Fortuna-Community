# Thứ tự thực thi Agent/Core và mất SBOM khi Core chưa sẵn sàng

## 1. Thứ tự khởi động Agent (main.go)

```
1. K8s client, cluster discovery
2. Syncer.Start(ctx)          → HTTP sync tới Core (pods, RBAC...) – chạy nền, không chờ Core
3. PodDetail reporter, Events collector – chạy nền
4. gRPC Connect(ctx)          → VÒNG LẶP CHỜ: retry mỗi 15s cho đến khi kết nối Core thành công
5. RegisterAgent, PingCore
6. SBOM processor + WorkQueue → queue.Start() → 2 workers bắt đầu đọc từ channel (chưa có pod)
7. PodWatcher.Start(ctx)      → CHẠY TRONG GOROUTINE (informer start, cache sync, “startup check” queue pods từ cache)
8. ListCurrentPods(ctx)       → GỌI TỪ MAIN: list API trực tiếp (Running pods trên node), NẾU queue != nil:
                                  → đẩy TẤT CẢ pod vào queue, return (nil, nil)
9. Main: for _, pod := range existingPods → existingPods = nil → không gọi ProcessPod; pod chỉ được xử lý bởi workers từ queue
10. Heartbeat goroutine, ...
```

**Kết luận thứ tự:** Agent **chờ kết nối Core xong** (bước 4) rồi mới tạo SBOM queue và watcher, sau đó mới gọi `ListCurrentPods` đẩy pod vào queue. Vì vậy **lúc đẩy pod vào queue, Agent đã connected**. Vấn đề mất SBOM không phải do “Core chưa sẵn sàng lúc agent khởi động” (vì agent đã block tới khi connect), mà do **Core sẵn sàng rồi sau đó bị restart** (rollout/restart) trong khi worker đang xử lý hoặc sắp gửi.

---

## 2. Khi nào SBOM được gửi?

- **Lần đầu (initial):** `ListCurrentPods` đẩy toàn bộ Running pods trên node vào queue → workers lần lượt `ProcessPod` (extract + SendSBOMFinding).
- **Sau đó:** Informer AddFunc/UpdateFunc (pod mới hoặc chuyển Running) → đẩy pod vào queue → worker xử lý.

Mỗi pod chỉ được watcher đẩy vào queue **một lần** (đã đánh dấu `processedPods[podUID]`). Nếu worker gọi `SendSBOMFinding` **thất bại** (vd: Core vừa restart → "client not connected"), Agent **không** tự retry và **không** xóa đánh dấu đã xử lý → pod đó **không bao giờ được đẩy lại** → SBOM mất.

---

## 3. Kịch bản mất SBOM (sau rebuild/deploy)

1. Deploy: Core và Agent (DaemonSet) đều được rollout.
2. Agent (trên từng node) chạy trước hoặc cùng lúc Core, **chờ Connect()** tới khi Core Ready.
3. Core Ready → Agent connect → ListCurrentPods → đẩy nhiều pod (dashboard, core, agent, flannel, …) vào queue.
4. **Rollout restart Core** (trong script deploy) → Core pod bị tạo lại → gRPC connection từ Agent tới Core **đứt**.
5. Workers vẫn đang xử lý (extract xong, gọi SendSBOMFinding) → lỗi **"client not connected"** → ProcessPod trả lỗi, worker chỉ log và bỏ qua, **không đẩy lại pod vào queue**.
6. Pod đã được watcher đánh dấu processed → informer không đẩy lại → **SBOM của các pod đó vĩnh viễn không gửi được**.

Tương tự nếu **Core chưa kịp listen gRPC** (vd startup chậm) trong lúc worker gửi → lỗi connection → cùng hậu quả.

---

## 4. Cần thay đổi gì?

1. **Retry khi gửi SBOM thất bại (lỗi thoáng qua):**  
   Khi `SendSBOMFinding` trả lỗi “client not connected”, “connection refused”, “Unavailable”, “deadline exceeded”, v.v. → coi là **transient** → **đẩy lại pod vào SBOM queue** (có giới hạn số lần, vd 3 lần, và delay 30s) để worker thử lại sau khi Core lên lại / reconnect.

2. **Không coi pod là “đã xử lý xong” khi chỉ gửi lỗi:**  
   Hiện tại watcher đánh dấu processed ngay khi **đẩy vào queue**, không phải khi **gửi thành công**. Nếu thêm retry trong queue (đẩy lại pod vào cùng queue), không cần sửa watcher: pod được worker “xử lý lại” từ queue, không phụ thuộc informer.

3. **Tùy chọn: đảm bảo Core Ready trước khi rollout Agent:**  
   Trong pipeline deploy, có thể “rollout Core trước, đợi Ready, rồi mới rollout Agent” để giảm cửa sổ Core restart đúng lúc Agent đang gửi. Cách này vẫn nên kết hợp với retry ở Agent.

---

## 5. Đã triển khai: Retry khi gửi SBOM thất bại

Trong **agent/internal/sbom/queue.go**:

- **isTransientSendError(err):** coi lỗi là thoáng qua nếu chứa "client not connected", "connection refused", "unavailable", "deadline exceeded", "no such host", v.v.
- **Khi ProcessPod trả lỗi transient:** tăng `retryCount[key]`, nếu ≤ **maxSendRetries (3)** thì sau **sendRetryDelay (30s)** đẩy lại pod vào queue (goroutine), **không** xóa khỏi active để pod được xử lý lại khi Core lên lại.
- **Sau 3 lần thử vẫn lỗi:** xóa retryCount và active, ghi log "Gave up pod ... after 3 send retries".
- **Khi gửi thành công:** xóa retryCount và active.

Nhờ đó, nếu Core restart đúng lúc worker gửi SBOM, agent sẽ tự retry tối đa 3 lần (mỗi lần cách 30s) thay vì mất SBOM vĩnh viễn.

---

## 6. Tóm tắt

| Vấn đề | Nguyên nhân | Hướng xử lý |
|--------|-------------|-------------|
| Pod dashboard, core, agent không có SBOM | Worker gửi SBOM đúng lúc Core restart / chưa listen → "client not connected" → không retry, pod đã marked processed → mất SBOM | **Đã thêm:** retry tối đa 3 lần, mỗi lần delay 30s, khi lỗi transient (queue.go) |
| Thứ tự thực thi | Agent đã chờ Connect() trước khi đẩy pod vào queue; mất SBOM do Core **sau đó** không sẵn sàng (restart/network) | Không cần đổi thứ tự startup; retry khi gửi thất bại đã bù trừ. |

File tham chiếu: `agent/cmd/main.go`, `agent/internal/sbom/queue.go`, `agent/internal/sbom/processor.go`, `agent/internal/watcher/pod_watcher_local.go`.
