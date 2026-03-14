# Phân tích API Architecture & Kế hoạch điều chỉnh

**Tài liệu tham chiếu:** [API-ARCHITECT_AND_ROUTE_STANDARD.md](./API-ARCHITECT_AND_ROUTE_STANDARD.md)  
**Bổ sung:** [API_ROUTE_STANDAR.md](./API_ROUTE_STANDAR.md), [API_ROUTE_REMEDIATION_PLAN.md](./API_ROUTE_REMEDIATION_PLAN.md)

Phân tích kiến trúc API trong API-ARCHITECT_AND_ROUTE_STANDARD.md, so sánh với hiện trạng và với chuẩn flat, đưa ra nhận định và plan thống nhất trước khi thực hiện chỉnh sửa.

---

# Phần 1: Tóm tắt API Architecture (API-ARCHITECT_AND_ROUTE_STANDARD.md)

## 1.1 Mô hình domain

API được tổ chức theo **4 domain + audit**:

| Domain     | Vai trò                    | Ví dụ path                              |
|-----------|----------------------------|------------------------------------------|
| inventory | Tài sản cluster (assets)    | /inventory/pods, /inventory/serviceaccounts |
| runtime   | Telemetry bảo mật theo thời gian thực | /runtime/events, /runtime/pods/:uid/processes |
| risk      | Đánh giá rủi ro, báo cáo   | /risk/scores, /risk/pods/:uid/report     |
| graph     | Phân tích attack path      | /graph, /graph/attack-paths/:uid         |
| audit     | Log điều tra               | /audit/logs, /audit/reports              |

Base path: `/api/v1`.

---

## 1.2 Quy tắc identifier (giống cả hai tài liệu)

- **Không expose database ID** trong API công khai.
- Dùng **Kubernetes UID** cho Pod, ServiceAccount, Deployment.
- Dùng **Node name** cho Node.
- Chuẩn path: `/resource/:uid` hoặc theo domain: `/inventory/pods/:uid`, **không** dùng `/by-id/:id`, `/by-uid/:uid`, `:podUid`, `:saUid`.

---

## 1.3 Mapping Pod theo API-ARCHITECT (domain-based)

| Chức năng        | Path đích (API-ARCHITECT)              | Ghi chú                    |
|------------------|----------------------------------------|-----------------------------|
| List pods        | GET /api/v1/inventory/pods             | Domain inventory            |
| Pod detail       | GET /api/v1/inventory/pods/:uid        |                             |
| Spec, capabilities, sbom | GET /api/v1/inventory/pods/:uid/spec, .../capabilities, .../sbom | Gắn với asset |
| Processes        | GET /api/v1/runtime/pods/:uid/processes  | Domain runtime              |
| Network          | GET /api/v1/runtime/pods/:uid/network    |                             |
| Events           | GET /api/v1/runtime/pods/:uid/events     |                             |
| Metrics          | GET /api/v1/runtime/pods/:uid/metrics     | runtime-metrics → metrics   |
| Risk report      | GET /api/v1/risk/pods/:uid/report         | Domain risk                 |
| Runtime risk     | GET /api/v1/risk/pods/:uid/runtime        | (runtime-risk profile)      |

Đặc điểm: **pod list/detail/spec/capabilities/sbom** nằm dưới **inventory**; **processes/network/events/metrics** nằm dưới **runtime**; **risk report và runtime risk** nằm dưới **risk**.

---

## 1.4 Mapping ServiceAccount, Audit, Graph

- **ServiceAccount:** GET /api/v1/inventory/serviceaccounts/:uid (bỏ by-uid/:saUid, bỏ :id).
- **Audit:** Chỉ GET /api/v1/audit/logs và GET /api/v1/audit/reports (gộp audit, audit-logs, reports).
- **Graph:** GET /api/v1/graph, GET /api/v1/graph/attack-paths/:uid, GET /api/v1/graph/blast-radius/:uid (param :uid).

---

## 1.5 Cấu trúc router (API-ARCHITECT)

Đăng ký theo domain, không phụ thuộc thứ tự route:

```text
registerInventoryRoutes(api)
registerRuntimeRoutes(api)
registerRiskRoutes(api)
registerGraphRoutes(api)
registerAuditRoutes(api)
```

---

# Phần 2: So sánh hai chuẩn (Flat vs Domain)

## 2.1 API_ROUTE_STANDAR.md (flat)

- Pod: GET /api/v1/pods, GET /api/v1/pods/:uid, GET /api/v1/pods/:uid/processes, .../network, .../sbom, .../risk-report, .../runtime-risk.
- Mọi thứ liên quan pod nằm dưới **một prefix** `/pods` hoặc `/pods/:uid/...`.
- Không có prefix /inventory, /runtime, /risk.

## 2.2 API-ARCHITECT_AND_ROUTE_STANDARD.md (domain)

- Pod list/detail/spec/capabilities/sbom: **/inventory/pods**, **/inventory/pods/:uid**, **/inventory/pods/:uid/...**
- Pod processes/network/events/metrics: **/runtime/pods/:uid/...**
- Pod risk report, runtime risk: **/risk/pods/:uid/report**, **/risk/pods/:uid/runtime**
- ServiceAccount: **/inventory/serviceaccounts/:uid**
- Audit: **/audit/logs**, **/audit/reports**

Khác biệt chính:

| Khía cạnh        | Flat (API_ROUTE_STANDAR)     | Domain (API-ARCHITECT)              |
|------------------|------------------------------|-------------------------------------|
| Pod list/detail  | /pods, /pods/:uid            | /inventory/pods, /inventory/pods/:uid |
| Pod processes    | /pods/:uid/processes         | /runtime/pods/:uid/processes        |
| Pod risk report  | /pods/:uid/risk-report       | /risk/pods/:uid/report              |
| SBOM             | /pods/:uid/sbom              | /inventory/pods/:uid/sbom           |
| Breaking change  | Ít (chỉ đổi param, bỏ by-*)  | Lớn (đổi toàn bộ path)              |
| Độ phức tạp      | Thấp                         | Cao (thêm layer domain)             |
| Scale / mở rộng  | Đủ dùng                      | Rõ ràng theo domain, dễ mở rộng     |

---

# Phần 3: Hiện trạng so với từng chuẩn

## 3.1 So với Flat

- **Đã khớp:** /pods, /risk/scores, /risk/analytics/..., /graph, /audit/reports (một phần).
- **Chưa khớp:** by-id/:id, by-uid/:uid, :podUid, network-connections, sbom/:podUid, risks/pods/:podUid/report, runtime-risk/pods/:podUid, serviceaccounts/by-uid/:saUid, audit trùng (audit, audit-logs, reports), router order hack.

## 3.2 So với Domain (API-ARCHITECT)

- **Hiện không có** prefix /inventory, /runtime (theo nghĩa domain).
- **Có** /risk/..., /graph/..., /audit/... nhưng chưa thống nhất tên (ví dụ /risks vs /risk, audit vs audit-logs).
- Toàn bộ path pod, serviceaccount, runtime pod-level, risk report cần **đổi prefix** sang inventory/runtime/risk mới khớp API-ARCHITECT.

---

# Phần 4: Nhận định

## 4.1 Mâu thuẫn giữa hai tài liệu

- **API_ROUTE_STANDAR.md:** Pod-centric, flat: mọi thứ pod dưới /pods/:uid/...
- **API-ARCHITECT:** Domain-centric: inventory vs runtime vs risk, pod xuất hiện ở nhiều domain (inventory/pods, runtime/pods/:uid, risk/pods/:uid).

Hai chuẩn **không tương thích path**; cần chọn **một** làm target chính cho từng giai đoạn.

## 4.2 Ưu điểm từng hướng

**Flat (API_ROUTE_STANDAR):**

- Ít thay đổi path, dễ làm trước, ít breaking change.
- Giải quyết nhanh Gin conflict (một :uid), duplicate, bỏ by-id/by-uid.
- Phù hợp nếu ưu tiên ổn định và giảm rủi ro.

**Domain (API-ARCHITECT):**

- Rõ ràng theo domain (inventory / runtime / risk / graph / audit), dễ mở rộng, gần mô hình Sysdig/Aqua.
- Đòi hỏi refactor lớn: Core routes, handlers, Dashboard, E2E, tài liệu.
- Phù hợp nếu coi API-ARCHITECT là kiến trúc dài hạn và chấp nhận một đợt breaking change có kế hoạch.

## 4.3 Rủi ro kỹ thuật chung

1. **Gin:** Cùng cấp path chỉ được một tên param (ví dụ :uid). Thống nhất :uid cho mọi pod route là bắt buộc dù chọn flat hay domain.
2. **Dashboard / E2E:** Mọi URL pod, serviceaccount, audit, risk report phải đổi theo target; cần cập nhật api client và test.
3. **Agent / tích hợp:** Nếu có client gọi trực tiếp path cũ, cần deprecation hoặc redirect (và Sunset header theo doc).

---

# Phần 5: Đề xuất hướng đi và plan chính xác

## 5.1 Hướng đi đề xuất: Hai giai đoạn rõ ràng

- **Giai đoạn 1 (ngắn hạn):** Áp dụng **chuẩn flat** (API_ROUTE_STANDAR) làm target: thống nhất identifier (:uid), bỏ by-id/by-uid, nested pod dưới /pods/:uid/..., gộp audit, tách module router. **Mục tiêu:** ổn định API, hết conflict và duplicate, không đổi prefix domain.
- **Giai đoạn 2 (trung/dài hạn):** Nếu product quyết định theo **API-ARCHITECT**, thực hiện **migration sang domain**: thêm /inventory, /runtime, /risk cho đúng resource, chuyển path từ flat sang domain, deprecation route cũ (Sunset header). **Mục tiêu:** API khớp hoàn toàn API-ARCHITECT_AND_ROUTE_STANDARD.md.

Lý do: API-ARCHITECT là kiến trúc “proposed”, refactor domain là breaking change lớn; làm flat trước cho phép sửa nhanh vấn đề hiện tại (Gin, duplicate, identifier) và giữ một bản plan chính xác cho bước sau.

## 5.2 Plan chính xác (Giai đoạn 1 – Flat, chuẩn API_ROUTE_STANDAR)

Chỉ thực hiện sau khi **chốt** rằng Giai đoạn 1 target là flat. Các bước dưới đây là plan “chính xác” theo phân tích trên.

### Phase 1A – Identifier và Pod routes (flat)

1. **Backend**
   - Bỏ GET /pods/by-id/:id (hoặc trả 410 + Deprecation).
   - Bỏ GET /pods/by-uid/:uid; thay bằng GET /pods/:uid (GetPodByUID), đọc c.Param("uid").
   - Đổi toàn bộ pod sub-route từ :podUid sang :uid:  
     /pods/:uid/processes, /pods/:uid/network-connections → /pods/:uid/network, /pods/:uid/events, /pods/:uid/runtime-metrics, /pods/:uid/spec, /pods/:uid/capabilities.  
     Trong handler: c.Param("podUid") → c.Param("uid").
   - Đảm bảo không còn route nào cùng cấp với /pods/:uid mà dùng param khác tên (tránh Gin conflict).
2. **Dashboard**
   - Pod detail: luôn dùng uid; gọi GET /api/v1/pods/:uid.
   - Các sub-resource: GET /api/v1/pods/:uid/processes, .../network, .../events, .../runtime-metrics, .../spec, .../capabilities.
3. **Test**
   - Cập nhật test route và param; chạy test pod.

### Phase 1B – Nested resource (flat, dưới /pods/:uid)

1. **Backend**
   - SBOM: GET /sbom/:podUid → GET /pods/:uid/sbom (handler đọc c.Param("uid")).
   - Risk report: GET /risks/pods/:podUid/report → GET /pods/:uid/risk-report.
   - Runtime risk: GET /runtime-risk/pods/:podUid (và .../events) → GET /pods/:uid/runtime-risk, GET /pods/:uid/runtime-risk/events.
   - Attack steps: GET /attack-steps/pods/:podUid → GET /pods/:uid/attack-steps.
   - Runtime signals: GET /runtime-signals/pods/:podUid → GET /pods/:uid/runtime-signals.
   - Handlers: thống nhất c.Param("uid").
2. **Dashboard**
   - Cập nhật URL tương ứng (sbom, risk-report, runtime-risk, attack-steps, runtime-signals).

### Phase 1C – ServiceAccount và Audit (flat)

1. **ServiceAccount:** GET /serviceaccounts/by-uid/:saUid → GET /serviceaccounts/:uid; GET /serviceaccounts/:id/permissions → GET /serviceaccounts/:uid/permissions. Handler dùng c.Param("uid"). Quyết định giữ hay deprecate /serviceaccounts/:id.
2. **Audit:** Chỉ giữ GET /audit/logs và GET /audit/reports; xóa hoặc redirect /audit, /audit-logs, /reports.

### Phase 1D – Router structure

1. Tách module: routes_inventory.go (pods, serviceaccounts nếu muốn nhóm), routes_runtime.go (runtime pod, runtime-risk, runtime-signals), routes_risk.go, routes_graph.go, routes_audit.go.
2. SetupRoutes gọi register* theo thứ tự cố định; không còn comment “register FIRST / BEFORE”.

---

## 5.3 Plan tham chiếu (Giai đoạn 2 – Domain, API-ARCHITECT)

**Chỉ thực hiện khi product chốt theo kiến trúc domain.** Khi đó:

1. **Inventory:** Thêm api.Group("/inventory"); pods, serviceaccounts chuyển sang /inventory/pods, /inventory/serviceaccounts; pod spec/capabilities/sbom under /inventory/pods/:uid.
2. **Runtime:** Thêm api.Group("/runtime"); pod processes/network/events/metrics chuyển sang /runtime/pods/:uid/...
3. **Risk:** Giữ /risk/...; pod risk report và runtime risk: /risk/pods/:uid/report, /risk/pods/:uid/runtime.
4. **Audit:** Giữ /audit/logs, /audit/reports.
5. **Deprecation:** Route flat cũ trả header Deprecation + Sunset; sau Sunset thì xóa.

---

# Phần 6: Checklist trước khi bắt đầu Phase 1A

- [ ] **Chốt target Giai đoạn 1:** Flat (API_ROUTE_STANDAR), không thêm prefix /inventory, /runtime trong bước này.
- [ ] **Chốt bỏ GET /pods/by-id/:id:** Dashboard và client dùng uid cho pod detail; list đã có uid.
- [ ] **Tài liệu:** Cập nhật API_ROUTE_STANDAR.md / API-ARCHITECT nếu có điều chỉnh nhỏ (ví dụ tên path network vs network-connections).
- [ ] **Repo:** Tag hoặc branch trước khi đổi route.
- [ ] **E2E / tích hợp:** Liệt kê endpoint đang gọi để cập nhật sau khi đổi path.

---

# Phần 7: Bảng mapping nhanh (Flat – Giai đoạn 1)

| Hiện tại | Target Phase 1 (Flat) |
|----------|------------------------|
| GET /pods/by-id/:id | Bỏ (hoặc 410 Deprecation) |
| GET /pods/by-uid/:uid | GET /pods/:uid |
| GET /pods/:podUid/processes | GET /pods/:uid/processes |
| GET /pods/:podUid/network-connections | GET /pods/:uid/network |
| GET /pods/:podUid/events | GET /pods/:uid/events |
| GET /pods/:podUid/runtime-metrics | GET /pods/:uid/runtime-metrics |
| GET /pods/:podUid/spec | GET /pods/:uid/spec |
| GET /pods/:podUid/capabilities | GET /pods/:uid/capabilities |
| GET /sbom/:podUid | GET /pods/:uid/sbom |
| GET /risks/pods/:podUid/report | GET /pods/:uid/risk-report |
| GET /runtime-risk/pods/:podUid | GET /pods/:uid/runtime-risk |
| GET /attack-steps/pods/:podUid | GET /pods/:uid/attack-steps |
| GET /runtime-signals/pods/:podUid | GET /pods/:uid/runtime-signals |
| GET /serviceaccounts/by-uid/:saUid | GET /serviceaccounts/:uid |
| GET /audit, /audit-logs, /reports | GET /audit/logs, GET /audit/reports |

Sau khi hoàn thành phân tích này, nên cập nhật **API_ROUTE_REMEDIATION_PLAN.md** để tham chiếu tới API-ARCHITECT và ghi rõ: Giai đoạn 1 = flat (plan trên), Giai đoạn 2 = domain (plan tham chiếu 5.3).
