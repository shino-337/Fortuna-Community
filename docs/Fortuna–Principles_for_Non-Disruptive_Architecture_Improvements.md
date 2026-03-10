Fortuna – Principles for Non-Disruptive Architecture Improvements
Document Version: 1.0
Date: March 06, 2026
Author: Senior Software Engineer and System Architect
Project Context: Fortuna – Kubernetes Security Asset Management (KSAM) Platform
Cảm ơn bạn đã cung cấp phân tích và ưu tiên cải tiến từ các findings trước đó. Dựa trên yêu cầu của bạn, tôi sẽ đưa ra các nguyên tắc kỹ thuật cụ thể để đảm bảo rằng việc triển khai các cải tiến (như thêm distributed tracing, thay đổi webhook policy, hoặc tích hợp caching) không phá vỡ kiến trúc hiện tại. Các nguyên tắc này được xây dựng dựa trên best practices cho Kubernetes-native systems, nhấn mạnh vào tính bền vững, backward compatibility, và giảm thiểu rủi ro downtime.
Tôi sẽ cấu trúc các nguyên tắc theo các lĩnh vực chính (Architecture Preservation, Implementation Strategy, Testing & Validation, và Monitoring & Rollback). Mỗi nguyên tắc bao gồm mô tả, lý do áp dụng cho Fortuna, ví dụ cụ thể liên kết với findings trước (từ review), và cách thực thi.

1. Architecture Preservation Principles
Những nguyên tắc này tập trung vào việc giữ nguyên core design (e.g., gRPC stream unified, NATS JetStream pipeline, PostgreSQL storage) trong khi thêm features.
Principle #1: Maintain Backward Compatibility for Interfaces and Data Models

Description: Bất kỳ thay đổi nào cũng phải hỗ trợ các interface cũ (API, gRPC proto, DB schema) ít nhất qua một transition period. Sử dụng versioning (e.g., API v1/v2) hoặc optional fields để tránh breaking changes.
Rationale: Fortuna dựa vào gRPC stream (ControlPlaneService.Stream) và REST API (HTTP 8080) làm backbone; breaking chúng có thể làm Agent hoặc Dashboard fail.
Example in Fortuna: Khi implement RS256 cho JWT (Finding #3), giữ HS256 làm fallback qua env flag (e.g., AUTH_JWT_ALGO=HS256|RS256), và chỉ deprecate HS256 sau 2 releases.
Implementation Guidelines:
Sử dụng protobuf evolution rules (add optional fields, avoid removing).
Test với old clients (e.g., Agent version cũ kết nối Core mới).


Principle #2: Isolate Changes with Modular Design

Description: Áp dụng modularization bằng cách tách changes vào separate modules hoặc microservices, tránh modify core logic trực tiếp.
Rationale: Core hiện stateless và multi-replica; thêm features như tracing không nên ảnh hưởng đến stream handling.
Example in Fortuna: Cho distributed tracing (Finding #1), thêm OTel instrumentation như middleware cho gRPC/HTTP, không chỉnh sửa business logic trong controlplane_server.go.
Implementation Guidelines:
Sử dụng Go modules hoặc packages riêng (e.g., pkg/tracing).
Deploy new components (e.g., Jaeger sidecar) mà không thay đổi existing YAML.


Principle #3: Preserve Data Durability and Consistency Guarantees

Description: Không thay đổi semantics của data storage hoặc sync (e.g., idempotency via message_id, NATS WorkQueuePolicy).
Rationale: Fortuna đảm bảo data durability qua PostgreSQL và NATS replicas=3; changes phải giữ ACID properties.
Example in Fortuna: Khi thêm sharding cho PostgreSQL (Finding #7), sử dụng Citus extension mà không drop tables cũ, migrate data incrementally.
Implementation Guidelines:
Sử dụng read replicas cho testing changes.
Áp dụng schema migrations với zero-downtime tools như Goose hoặc GORM.



2. Implementation Strategy Principles
Tập trung vào cách rollout changes an toàn trong Kubernetes environment.
Principle #4: Use Feature Flags and Progressive Rollout

Description: Giới thiệu features mới qua flags (env vars hoặc ConfigMap), và rollout dần dần (canary, blue-green deployments).
Rationale: Giảm rủi ro cho production multi-cluster, nơi Agents sync định kỳ.
Example in Fortuna: Cho webhook failurePolicy: Fail (Finding #2), thêm env WEBHOOK_FAILURE_POLICY=Ignore|Fail, rollout canary Deployment với 10% traffic.
Implementation Guidelines:
Sử dụng Flagger hoặc Argo Rollouts cho Kubernetes.
Monitor flag-enabled metrics (e.g., fortuna_webhook_failures_total).


Principle #5: Incremental Refactoring Over Big Bang Changes

Description: Phân chia refactor lớn thành small, independent PRs; chỉ merge sau khi test full path.
Rationale: Fortuna có dependencies chặt chẽ (Agent → Core → DB); big changes có thể break stream reconnection.
Example in Fortuna: Cho batch insert CVE loader (Finding #4), bắt đầu bằng optional batch mode trong SyncData, sau đó mandatory.
Implementation Guidelines:
Sử dụng trunk-based development với short-lived branches.
Review PRs với focus trên non-breaking tests.


Principle #6: Align with Existing Tech Stack

Description: Ưu tiên libraries và tools compatible với current stack (Go, GORM, Gin, NATS, Prometheus).
Rationale: Tránh introduce new languages hoặc heavy dependencies, giữ build/deploy simple.
Example in Fortuna: Cho DLQ in NATS (Finding #8), sử dụng NATS built-in features thay vì thêm Kafka.
Implementation Guidelines:
Check go.mod compatibility.
Ưu tiên open-source tools Kubernetes-native (e.g., cert-manager cho rotation).



3. Testing & Validation Principles
Đảm bảo changes không introduce regressions.
Principle #7: Comprehensive Testing at Multiple Levels

Description: Cover unit, integration, và E2E tests; bao gồm negative cases (e.g., network failure).
Rationale: Fortuna có complex flows (stream bidirectional, webhook); tests phải validate end-to-end.
Example in Fortuna: Cho Agent reconnect (Finding #5), thêm unit test cho backoff logic, integration test với mock Core, E2E với minikube (simulate 50 Agents).
Implementation Guidelines:
Unit: Go testing package, coverage >80%.
Integration: Testcontainers cho PostgreSQL/NATS.
E2E: k6 hoặc Locust cho load, assert no data loss.


Principle #8: Chaos Engineering for Resilience Validation

Description: Simulate failures (pod kill, network partition) để test changes.
Rationale: Fortuna cần HA (Core replicas, Agent offline handling); changes phải survive chaos.
Example in Fortuna: Sau thêm caching (Finding #14), dùng Chaos Mesh để inject DB latency, verify fallback to DB.
Implementation Guidelines:
Deploy Chaos Mesh in test cluster.
Define experiments (e.g., kill 1 Core pod, check stream reconnect).



4. Monitoring & Rollback Principles
Theo dõi và revert nhanh nếu issue.
Principle #9: Enhanced Monitoring During Rollout

Description: Thêm metrics/traces cụ thể cho changes, set alerts cho anomalies.
Rationale: Fortuna có Prometheus metrics (stream_messages_received_total); extend để detect breaks.
Example in Fortuna: Cho alerting rules (Finding #15), thêm metric fortuna_change_latency_seconds{change="tracing"}, alert nếu > baseline 10%.
Implementation Guidelines:
Sử dụng existing /metrics endpoint.
Integrate Grafana dashboards cho A/B comparison (old vs new).


Principle #10: Robust Rollback Plan

Description: Luôn có rollback strategy (e.g., helm rollback nếu switch to Helm, hoặc git revert).
Rationale: Giảm downtime trong production.
Example in Fortuna: Cho backup/restore (Finding #13), test rollback bằng cách restore từ Velero snapshot trước change.
Implementation Guidelines:
Document rollback steps in PR.
Sử dụng GitOps (ArgoCD) để automate revert.