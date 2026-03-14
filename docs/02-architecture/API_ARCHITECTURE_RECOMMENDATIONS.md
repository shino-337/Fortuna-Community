# Khuyến nghị kiến trúc API và maturity

Version: 1.0  
Date: 2026-03  
Tham chiếu: [ROUTE_MIGRATION_MAPPING.md](./ROUTE_MIGRATION_MAPPING.md), [API_ROUTE_CLASSIFICATION_ABC.md](./API_ROUTE_CLASSIFICATION_ABC.md)

---

## 6. Điểm nên chỉnh sớm (không bắt buộc)

### 6.1 /cluster domain nên mở rộng

Hiện tại cluster domain chỉ có:

- `GET/POST /cluster/certificates/*`

Thực tế cluster domain nên có:

- `/cluster/info` — thông tin cluster (danh sách / summary)
- `/cluster/nodes` — nodes thuộc cluster (tránh node API chỉ nằm dưới inventory)
- `/cluster/health` — (tùy chọn) health theo cluster
- `/cluster/events` — (tùy chọn) events liên quan cluster

**Đề xuất đã áp dụng:**

- **GET /cluster/info** — Danh sách cluster với thông tin cơ bản (id, name, last_sync); dùng chung nguồn với inventory nhưng namespace rõ là infrastructure.
- **GET /cluster/:id/nodes** — Danh sách node names của cluster; tránh node API chỉ nằm dưới inventory.

Inventory vẫn giữ `GET /inventory/clusters`, `GET /inventory/clusters/:id/nodes/:nodeName` (chi tiết node) để tương thích; dashboard và client mới có thể ưu tiên `/cluster/info`, `/cluster/:id/nodes`.

---

### 6.2 /dashboard là aggregate API

Dashboard APIs:

- `GET /dashboard/stats`
- `GET /dashboard/metrics/*` (ví dụ threat-velocity)

**Ghi rõ trong docs:**

- **Aggregate endpoints** — Tổng hợp từ nhiều nguồn (DB, cache, compose).
- **Not stable for external clients** — Có thể cache, thay đổi format hoặc query để tối ưu UI; không nên dùng làm contract cho tích hợp bên ngoài.

Các API domain (inventory, runtime, risk, …) là contract ổn định; dashboard dùng chúng và thêm lớp aggregate cho UI.

---

### 6.3 Agent ingest nên tách namespace (lâu dài)

Hiện tại:

- `POST /api/v1/agent/*` (sync, pod-runtime-metrics, pod-processes, …)

Về lâu dài nên chuyển sang:

- **POST /api/v1/ingest/v1/*** (hoặc tương đương)

Lý do:

- Agent ingest ≠ user API.
- Kiến trúc pipeline rõ ràng: **Agent → ingest → (NATS) → core → risk engine**.

Roadmap: có thể thêm alias `/ingest/v1/*` trỏ cùng handler, deprecate `/agent/*` sau khi agent đã chuyển.

---

## 7. Nhất quán identifier

Trong `routes_risk.go` đã đổi `:riskId` → `:id` (đúng).

Toàn bộ platform nên dùng thống nhất:

- **:id** — primary key hoặc stable ID (rule id, insight id, cluster id).
- **:uid** — Kubernetes UID (pod, serviceaccount, …).
- **:name** — tên resource khi thích hợp (nodeName, …).

Không nên mix: `:riskId`, `:podUid`, `:saUid` → chuẩn hóa thành `:id` hoặc `:uid` tùy ngữ cảnh.

---

## 8. Kiến trúc API sau refactor

Tổng thể:

```
/api/v1

platform
 ├ auth
 ├ health
 ├ users
 └ ws

infrastructure
 └ cluster
     ├ info
     ├ :id/nodes
     └ certificates

security
 ├ inventory
 ├ runtime
 ├ risk
 ├ graph
 ├ audit
 └ policy

ui
 └ dashboard   (aggregate; not stable for external clients)
```

Clean separation: platform, infrastructure (cluster), security domains, ui (dashboard).

---

## 9. Đánh giá maturity

| Level   | Mô tả                |
|--------|-----------------------|
| 1 Prototype | Route lộn xộn        |
| 2 Intermediate | Partial domain   |
| 3 Production | Domain API        |
| 4 Enterprise | Domain + versioning |

Hệ thống hiện tại: **Production architecture**, khoảng **Level 3.5 / 5**.

---

## 10. Những việc nên làm tiếp theo

Sau khi API refactor ổn định, 3 subsystem quan trọng nhất của security platform:

1. **Runtime event pipeline**  
   Agent → ingest → (NATS) → core → risk engine.

2. **Attack graph engine**  
   Pods, service accounts, RBAC, network → graph database.

3. **Risk scoring engine**  
   Privilege, runtime anomalies, network exposure, image CVE.

---

## 11. Kết luận

Refactor hiện tại:

- ✔ Domain API chuẩn
- ✔ Namespace rõ (inventory, runtime, risk, graph, audit, policy, cluster)
- ✔ Dashboard sync với path mới
- ✔ Router modular
- ✔ Legacy cleanup

Hệ thống đủ tốt để scale 50–200 nodes. Tiếp tục đúng hướng (ingest namespace, graph engine, risk scoring) có thể đạt kiến trúc tương tự các platform như Wiz.
