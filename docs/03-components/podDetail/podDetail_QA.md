Tôi là Senior Software Engineer và System Architect với hơn 15 năm kinh nghiệm thiết kế hệ thống phân tán, nền tảng cloud-native và kiến trúc bảo mật doanh nghiệp. Tôi sẽ thực hiện kiểm tra và đánh giá chi tiết cho luồng xử lý thông tin pod trong Agent và Core của dự án Fortuna, bao gồm các yếu tố như status, SBOM, runtime metrics, process runtime, network connections, và risk evaluation.
Theo quy trình làm việc nghiêm ngặt của tôi, tôi sẽ KHÔNG bắt đầu phân tích cho đến khi nhận đủ thông tin kỹ thuật cần thiết về luồng này. Điều này đảm bảo mọi nhận xét đều dựa trên bằng chứng thực tế, tránh giả định. Tôi sẽ tập trung vào luồng pod details để kiểm tra tính toàn vẹn, performance, security, và reliability.
Dựa trên yêu cầu của bạn, tôi sẽ tạo checklist kiểm tra dưới dạng các câu hỏi markdown rõ ràng, có cấu trúc để bạn dễ dàng phản hồi từng phần một (ví dụ: bạn có thể copy-paste và trả lời bên dưới mỗi câu hỏi). Hãy cung cấp thông tin chi tiết nhất có thể, bao gồm code snippets, configs, diagrams, hoặc file đính kèm nếu cần.
1. Tổng quan Luồng Pod Details

Mô tả tổng quát luồng end-to-end cho pod details: Từ Agent thu thập (status, SBOM, runtime metrics, processes, network, risk) → gửi lên Core (qua stream/RPC) → Core xử lý (ingest, store, evaluate) → Dashboard/API expose?
High-level diagram cho luồng pod-specific (PNG/SVG hoặc text-based, ví dụ: Agent → gRPC EVENT → Core Ingest → DB → Risk Engine)?
Các loại pod data chính đang xử lý (status, SBOM, metrics, processes, network connections, events, risk scores)? Có thêm loại nào (e.g., capabilities từ PCE)?

2. Agent-Side Processing

Cách Agent thu thập pod status (list/watch via client-go, informer, hay custom loop)?
Luồng SBOM extraction: Trigger khi nào (pod create/update, watcher queue)? Công cụ dùng (containerd export, registry pull)? Code snippet từ SBOM processor (agent/internal/sbom/...)?
Runtime metrics collection: Interval (POD_DETAIL_RUNTIME_METRICS_INTERVAL)? Phương pháp (cAdvisor-like, exec vào container)? Code snippet từ PodDetail Reporter?
Process runtime: Cách lấy processes trong container (pods/exec, nsenter)? Interval và handling cho multi-container pods? Code snippet liên quan?
Network connections: Cách thu thập (netstat, ss, hay eBPF)? Lọc theo pod/namespace? Code snippet?
Risk-related data ở Agent: Agent có pre-compute risk (e.g., capabilities) hay chỉ thu thập raw data? Nếu có, mô tả.
Gửi data lên Core: Qua stream EVENT (kind=pod_runtime_metrics, pod_processes, etc.) hay RPC riêng (SendSBOMFinding)? Handling queue (bounded 256) và backoff nếu fail?
Configs liên quan ở Agent (env vars như POD_DETAIL_*, RUNTIME_EVENTS_ENABLED, SBOM_ENABLED)?

3. Core-Side Ingestion và Processing

Nhận data từ Agent: Handler cho từng EVENT type (IngestPodRuntimeMetricsPayload, IngestPodProcessesPayload, IngestSBOMFromStream)? Code snippet từ controlplane_server.go hoặc agent_handlers.go?
Lưu trữ: Bảng DB cho từng loại (pod_runtime_metrics, pod_processes, pod_network_connections, sboms, insights, risk_scores)? Schema chi tiết (fields, indexes)?
Risk evaluation: Trigger khi nào (sync hoàn tất, event ingest)? Engine dùng (CEL, custom logic)? Tích hợp PCE (Pod Capability Engine) như thế nào? Code snippet từ risk engine hoặc PCE scheduler?
Async processing: Sử dụng NATS cho pipeline (e.g., fortuna-events.sbom.> → CVE matcher → insights)? Subjects và workers cụ thể cho pod data?
Deduplication và idempotency: Cách xử lý duplicate data (message_id, in-memory map)? TTL?
Backpressure: Throttle cho pod events (STREAM_THROTTLE_MESSAGES_PER_SECOND)? Gửi CONTROL envelope retry_after?
Configs liên quan ở Core (env vars như RATE_LIMIT_, STREAM_THROTTLE_)?

4. Integration với Kubernetes và Security

Tương tác K8s cho pod details: RBAC cần cho Agent (get pods, exec)? Isolation (namespace selector)?
Security cho data: Encryption at-rest/transit cho sensitive pod info (processes, network)? mTLS cho stream?
Admission webhook liên quan: Policy check pod details trước deploy (e.g., block based on capabilities)?

5. Performance và Scalability cho Pod Flow

Throughput: Đo events/sec cho pod metrics/processes (per node, total cluster)?
Latency: End-to-end từ Agent collect → Core store → Dashboard view?
Scaling: Handling cho 1000+ pods? HPA cho workers xử lý pod data?
Caching: Cache pod details ở Core (in-memory, Redis) cho API queries?

6. Reliability và Observability

Error handling: Agent retry nếu collect fail (e.g., exec denied)? Core DLQ cho failed ingest?
Failover: Nếu Core replica down, pod data có mất? State sync giữa replicas?
Metrics: Prometheus keys cho pod flow (fortuna_pod_metrics_ingested_total, latency histograms)?
Logging/Tracing: Trace spans cho pod pipeline? Log levels cho errors (e.g., failed SBOM extract)?
Alerting: Rules cho anomalies (e.g., high pod event drop rate)?

7. Dashboard và API Exposure

API endpoints cho pod details (/api/v1/pods/:id/runtime-metrics, /api/v1/pods/:id/processes, /api/v1/pods/:id/network-connections, etc.)? Auth và rate limiting?
Dashboard rendering: Cách hiển thị pod data (React components)? Polling hay websocket?
Known issues: Bất kỳ bugs/open issues liên quan pod flow (e.g., slow SBOM, missing metrics)?

**Test cases (Dashboard Network):**
- **TC-NET-1** Dashboard Pod Detail page preloads runtime-metrics, processes, network-connections when pod loads so Overview shows counts and Network tab has data without requiring a tab click first.
- **TC-NET-2** Dashboard Network tab displays table with columns: Direction, Remote address, Local port, Protocol, Status, Timestamp; empty state when `items` is empty.
- **TC-NET-3** E2E `scripts/e2e/test-pod-detail-ping-flow.sh` asserts GET `/api/v1/pods/by-uid/:uid/network-connections` returns JSON with `items` (array) and `podUid`; fail the test if not (dashboard depends on this shape).
- **TC-NET-4** E2E `scripts/e2e/test-pod-detail-lodash-network.sh`: pod **website-vuln-lodash** (static server on TCP 3000) is used to test network connections; script deploys or reuses the pod, waits for Pod Detail reporter, then asserts API returns at least one connection (expect LISTEN on 3000). Image: build from `deploy/e2e/images/website-vuln-lodash`; see README there.

8. Thông tin Bổ sung (Rất Hữu ích)

Code snippets quan trọng: PodDetail Reporter (Agent), Ingest*Payload (Core), risk evaluation.
Benchmark reports: Load test cho pod data ingestion (e.g., 100 pods simultaneous).
GitHub links hoặc paste files: agent/internal/pod_detail/, core/internal/service/pod_service.go, etc.
Any migrations or changes recent cho pod flow?