1.1 Luồng kết nối thực tế

Xác nhận rõ:

Agent có connect đến Core local node hay master-node?

Core có giao tiếp chéo node không?

Dashboard gọi Core nào?

NATS được dùng để:

log only?

event?

runtime stream?

Cần cung cấp:

Sơ đồ kết nối thực tế (ASCII ok)

Port mapping

Service type (ClusterIP / NodePort / HostNetwork)

1.2 Network Transport

Xác nhận:

Channel	Protocol	TLS	mTLS
Agent → Core	?	?	?
Core → NATS	?	?	?
Dashboard → Core	?	?	?
1.3 Latency Baseline

Chạy test:

Ping Agent → Core

Agent → NATS

Dashboard → Core

Cung cấp:

Avg latency

95th percentile

2️⃣ gRPC / REST State
2.1 Agent → Core

Hiện đang dùng:

REST?

gRPC?

Hybrid?

Cho biết:

Số endpoint Agent đang gọi

Tần suất gọi (per minute)

Payload size trung bình

2.2 Streaming

Có đang dùng:

gRPC streaming?

WebSocket?

NATS as relay?

3️⃣ NATS Review (Critical)
3.1 Deployment Mode

Xác nhận:

NATS standalone?

NATS cluster?

JetStream enabled?

Persistence enabled?

3.2 Current Usage

NATS đang dùng để:

Log stream?

Runtime events?

Process events?

Internal service communication?

Cho biết:

Subject naming pattern

Message size trung bình

Msg/sec peak

3.3 Retention & Backpressure

Xác nhận:

Có limit message size?

Có consumer lag monitoring?

Khi consumer down thì log mất hay queue lại?

4️⃣ Core Deployment Model
4.1 Core Instances

Hiện có:

2 core instance (mỗi node 1 cái)?

Xác nhận:

Core có stateless không?

Có shared DB?

Có cache layer không?

4.2 Data Ownership

Pod data được lưu:

Local DB mỗi node?

Central DB?

In-memory only?

5️⃣ Resource & Load Baseline

Cho biết:

Tổng số pod hiện tại?

Pod per node?

Avg process per pod?

Log volume (MB/hour)?

6️⃣ Failure Scenario Testing

Thực hiện test và báo cáo:

6.1 Kill 1 Agent

Core có detect mất agent?

UI hiển thị thế nào?

Data stale bao lâu?

6.2 Kill NATS

Log có mất?

Core có retry?

Agent có block?

6.3 Kill 1 Core Node

Dashboard còn hoạt động?

Agent reconnect sang core khác không?

7️⃣ Security Audit Quick Check

Xác nhận:

Agent có xác thực identity không?

Cert rotation bao lâu?

JWT expiry bao lâu?

Có audit log API access không?

8️⃣ Performance Snapshot

Chạy:

top
htop
kubectl top nodes
kubectl top pods

Cung cấp:

CPU usage core

Memory usage core

CPU usage agent

NATS CPU usage

9️⃣ Log Pipeline Specific

Vì bạn dùng NATS stream log, cần xác nhận:

Agent → NATS trực tiếp?

Hay Agent → Core → NATS?

Core có consume log không?

Có multiple consumers không?

10️⃣ Đồng Bộ Kiến Trúc (Rất Quan Trọng)

Hiện tại có 2 core node, khả năng đáp ứng từ 50-200 node như thế nào ?

Xác nhận:

Có leader election không?

Có duplicate processing không?

Có risk double write DB không?

🎯 Format Báo Cáo Lại

Vui lòng trả lời theo format:

1. Network topology:
2. Agent-Core protocol:
3. NATS deployment:
4. Core deployment:
5. Load metrics:
6. Failure test results:
7. Security setup:
8. Observed bottlenecks: