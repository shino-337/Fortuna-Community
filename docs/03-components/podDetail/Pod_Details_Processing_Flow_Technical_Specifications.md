Fortuna – Pod Details Processing Flow Technical Specifications
Document Version: 1.1
Review Date: March 07, 2026
Reviewer: Senior Software Engineer and System Architect
Project: Fortuna – Kubernetes Security Asset Management (KSAM) Platform
This document provides an updated and in-depth technical specification for the Pod Details Processing Flow in Fortuna, based on a re-evaluation of the provided information in "podDetail_QA_Answers.md". The review focuses on deepening the analysis of the 11 findings from the previous assessment, incorporating proposed execution flows (e.g., sequence diagrams or step-by-step processes), and detailing information processing mechanisms (e.g., data ingestion, transformation, storage, and error handling).
Findings have been re-checked for accuracy against the supplied data (e.g., Agent's eventCh bounded at 256, Core's synchronous ingestion, lack of network collection). Each finding now includes:

Enhanced root cause analysis with code references.
Impact with quantifiable risks where possible.
Recommendations with:
Proposed Execution Flow: Step-by-step or pseudo-diagram for the improved mechanism.
Information Processing Mechanism: Detailed handling of data (ingestion, validation, transformation, storage).
Implementation Notes: Tech stack suggestions, potential refactor effort (low/medium/high), and testing considerations.


Findings are prioritized by risk level (Critical/High first). This spec aims for architectural sustainability, aligning with Kubernetes-native best practices (e.g., async processing via NATS, eBPF for runtime data).

Finding #1: Network Connections và Kubernetes Events Không Được Thu thập Từ Agent
Description
Agent thiếu hoàn toàn logic thu thập network connections (e.g., open ports, connections via ss/netstat) và Kubernetes events (e.g., pod lifecycle events từ K8s API watcher). Core đã có sẵn bảng (pod_network_connections, k8s_events) và API endpoints (GET/POST), nhưng dữ liệu luôn rỗng trừ khi ingest thủ công qua POST.
Technical Root Cause

Không có code collector ở Agent (e.g., trong reporter.go hoặc watcher). Core chỉ hỗ trợ ingestion qua stream EVENT hoặc POST, nhưng Agent chỉ gửi STATE/EVENT cho metrics/processes/SBOM/runtime_events – không có kind cho network/events.
Reference: Agent's eventCh chỉ handle "pod_runtime_metrics", "pod_processes", "runtime_events", "sbom"; Core's Ingest handlers không có cho network/events từ stream.

Impact

Dashboard tabs (Network, Events) vô dụng → mất 20-30% giá trị Pod Detail page.
Không detect real-time threats (e.g., anomalous connections → lateral movement).
Compliance gaps (e.g., audit logs thiếu events cho SOC2).

Risk Level
High
Recommendation

Effort: Medium (thêm collector mới ở Agent, extend stream proto).
Proposed Execution Flow:
Agent khởi động → Init network watcher (ticker 5m) và event informer (client-go SharedInformer cho Events).
Collect: Network – exec ss -tunap per container → parse connections; Events – OnAdd/OnUpdate từ informer → filter by pod UID.
Marshal payload (JSON: podUid, clusterId, namespace, connections[] / events[]).
Send qua eventCh → gRPC Stream EVENT (new kinds: "pod_network_connections", "pod_events").
Core Recv → IngestPodNetworkConnectionsPayload / IngestPodEventsPayload → DB insert.
Dashboard fetch → real-time display.

Information Processing Mechanism:
Ingestion: Agent validate payload (e.g., filter sensitive IPs), compress nếu >1KB (snappy).
Transformation: Core parse JSON → normalize (e.g., resolve IP to service if possible via K8s API).
Storage: Append to tables with retention policy (e.g., delete >7 days via cronjob).
Error Handling: Retry collect 3x nếu exec fail; DLQ nếu ingest error.

Implementation Notes: Sử dụng client-go cho event informer; eBPF (cilium/ebpf) cho network nếu muốn low-overhead. Test: Unit (mock exec output); E2E (deploy pod with netcat → verify connections in DB).


Finding #2: Drop Data Khi Event Channel Đầy (Bounded 256) – Không Có Backpressure Hoặc Retry
Description
Agent drop metrics/processes/SBOM khi eventCh đầy (select default: drop & log) – không retry, không buffer bền vững.
Technical Root Cause

Channel fixed-size (make(chan, 256)) ưu tiên non-blocking, nhưng thiếu overflow strategy. Throttle chỉ từ Core (CONTROL envelope), không nội bộ ở Agent.
Reference: reporter.go – select send to eventCh; nếu đầy → log "event channel full, drop".

Impact

Mất dữ liệu ở high-load nodes (e.g., 500+ pods churn) → metrics outdated >50%.
Không alert drop → vấn đề ẩn.

Risk Level
High
Recommendation

Effort: Low (refactor channel handling).
Proposed Execution Flow:
Agent collect data → Enqueue to unbounded queue (or ring buffer).
Worker pool (3-5 goroutines) drain queue → send to gRPC.
Nếu send fail hoặc Core throttle → backoff retry (exponential, max 5min).
Monitor queue depth → nếu >1000, reduce collect rate (e.g., skip non-critical pods).

Information Processing Mechanism:
Ingestion: Prioritize queue by type (e.g., SBOM > metrics > processes).
Transformation: Add retry_count to payload; discard sau 3 retries.
Storage: N/A (xử lý trước send).
Error Handling: Log with rate-limit; metric agent_queue_depth, dropped_total.

Implementation Notes: Sử dụng github.com/eapache/queue cho unbounded. Test: Load sim (inject 1000 events) → verify no drop.


Finding #3: Stream Handler Ở Core Xử Lý Đồng Bộ – Bottleneck Khi Scale
Description
Toàn bộ Recv → dedup → ingest → ACK synchronous trong goroutine stream → block nếu ingest chậm (DB write).
Technical Root Cause

Sequential processing trong controlplane_server.go; không offload sang async worker.
Reference: case "pod_runtime_metrics": Ingest... (DB.Create synchronous).

Impact

Latency >1s per message ở large payloads → stream timeout, reconnect flood.
Không scale đến 1000 nodes (e.g., 10k events/min).

Risk Level
Medium-High
Recommendation

Effort: Medium (tích hợp NATS cho ingest).
Proposed Execution Flow:
Stream Recv envelope → dedup → Publish to NATS "fortuna.internal.pod_ingest.{type}".
Send ACK ngay (fast response).
Worker pool (HPA-scaled) consume NATS → IngestPayload → DB batch insert.
Nếu NATS backlog, alert.

Information Processing Mechanism:
Ingestion: Publish with trace ID for correlation.
Transformation: Batch 100 records before DB.CreateInBatches.
Storage: Use transactions for consistency.
Error Handling: NACK message nếu fail → auto-retry via NATS redelivery.

Implementation Notes: Extend existing NATS streams (fortuna-events). Test: Locust sim 100 Agents → measure throughput.


Finding #4: Deduplication Dựa Trên In-Memory Map Per Replica – Duplicate Data Khi Multi-Replica Core
Description
Dedup không shared → duplicates nếu load balancer route message khác replica.
Technical Root Cause

Map[string]time.Time per instance, TTL 10min; không sync.
Reference: seenOrMark in controlplane_server.go.

Impact

DB bloat (e.g., duplicate metrics rows) → query slow 2x, storage +20%.

Risk Level
Medium
Recommendation

Effort: Low (thêm Redis).
Proposed Execution Flow:
Recv message_id → Redis GET "dedup:{message_id}".
Nếu exist → skip & inc duplicate metric.
Else → SETEX "dedup:{message_id}" TTL 600 → proceed ingest.

Information Processing Mechanism:
Ingestion: Atomic check-and-set.
Transformation: N/A.
Storage: Redis as cache, fallback to DB nếu Redis down.
Error Handling: Nếu Redis fail, fallback to in-memory.

Implementation Notes: Use go-redis. Test: Multi-replica sim → verify no duplicates.


Finding #5: Không Có DLQ Hoặc Retry Cho Failed Ingest Ở Core
Description
Ingest error → log & ACK → mất data.
Technical Root Cause

Không retry trong Ingest functions; luôn ACK.
Reference: _ = Ingest... (ignore error).

Impact

Transient DB errors → mất telemetry 10-20%.

Risk Level
Medium
Recommendation

Effort: Low (add retry).
Proposed Execution Flow:
Ingest fail → Retry 3x with backoff (1s, 2s, 4s).
Vẫn fail → Publish to DLQ NATS "fortuna.dlq.pod_ingest".
DLQ consumer → manual review/alert.

Information Processing Mechanism:
Ingestion: Wrap Ingest in retry loop.
Transformation: Add error_msg to DLQ payload.
Storage: DLQ retention infinite.
Error Handling: Metric ingest_errors_total{type}.

Implementation Notes: Use cenkalti/backoff. Test: Inject DB error → verify DLQ.


Finding #6: SBOM Drop Khi sbomStreamCh Đầy – Không Retry
Description
Tương tự #2, SBOM drop khi channel đầy.
Technical Root Cause

Non-blocking send in SBOM processor.

Impact

Mất SBOM → no CVE/insights cho pod.

Risk Level
High
Recommendation

Effort: Low (tương tự #2).
Proposed Execution Flow: Như #2, nhưng prioritize SBOM in queue (separate high-priority channel).
Information Processing Mechanism: Compress SBOM JSON; store local temp nếu queue full → retry later.
Implementation Notes: Test: High pod churn → verify all SBOM ingested.


Finding #7: Không Có Caching Cho Pod Detail API – DB Load Cao Khi Nhiều User Xem Pod Detail
Description
Direct DB query cho GET endpoints.
Technical Root Cause

No middleware cache in Gin routes.

Impact

High DB load ở multi-user (e.g., 100 req/s → CPU spike).

Risk Level
Medium
Recommendation

Effort: Low (add Redis).
Proposed Execution Flow:
API req → Redis GET "pod:{uid}:metrics".
Miss → DB query → JSON marshal → Redis SETEX TTL 120s.
Hit → return cached.

Information Processing Mechanism:
Ingestion: N/A.
Transformation: Serialize to JSON.
Storage: TTL = collect interval.
Error Handling: Fallback to DB nếu Redis fail.

Implementation Notes: Invalidate on ingest. Test: Benchmark req/s.


Finding #8: Thiếu Metrics Chi Tiết Và Histogram Cho Pod Flow
Description
Chỉ metric tổng quát, thiếu per-type latency/drop.
Technical Root Cause

Chưa promhttp instrument chi tiết.

Impact

Không debug bottlenecks.

Risk Level
Medium
Recommendation

Effort: Low (add Prometheus).
Proposed Execution Flow: Wrap Ingest with timer → Observe histogram.
Information Processing Mechanism: Label {type="metrics"}.
Implementation Notes: Test: Verify Grafana dashboard.


Finding #9: Process Collection Qua Exec Ps – Fragile Và Không Hiệu Suất Cao
Description
Exec "ps" dễ fail, overhead cao.
Technical Root Cause

pods/exec dependent on container bins.

Impact

Thiếu data ở distroless images.

Risk Level
Medium
Recommendation

Effort: Medium (switch to procfs).
Proposed Execution Flow:
Agent nsenter pod namespace → read /proc/<pid>/.
Parse stat/cmdline → build process list.

Information Processing Mechanism: Filter system processes; aggregate per container.
Implementation Notes: Require CAP_SYS_PTRACE. Test: Distroless pod.


Finding #10: Không Có Encryption At-Rest Rõ Ràng Cho Sensitive Pod Data
Description
Command/binary_path không encrypt.
Technical Root Cause

PostgreSQL default storage.

Impact

Data leak nếu DB breach.

Risk Level
Medium
Recommendation

Effort: Medium (pgcrypto).
Proposed Execution Flow: Ingest → Encrypt(command) → Store.
Information Processing Mechanism: Use AES key from Secrets.
Implementation Notes: Test: Verify decrypt in API.


Finding #11: Polling-Based Dashboard – Không Real-Time
Description
Fetch on load/tab → stale data.
Technical Root Cause

No websocket in React.

Impact

User phải refresh.

Risk Level
Low
Recommendation

Effort: Medium (add WS).
Proposed Execution Flow:
Dashboard connect WS "/ws/pod/:uid".
Core on ingest → Publish NATS → WS push update.

Information Processing Mechanism: Delta updates only.
Implementation Notes: Use gorilla/websocket. Test: Real-time sim.