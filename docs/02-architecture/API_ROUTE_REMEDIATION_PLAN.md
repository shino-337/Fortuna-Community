# Kế hoạch khắc phục API Route theo chuẩn

**Phân tích kiến trúc (bắt buộc đọc trước):** [API_ARCHITECT_ANALYSIS_AND_PLAN.md](./API_ARCHITECT_ANALYSIS_AND_PLAN.md)  
**Chuẩn tham chiếu:** [API_ROUTE_STANDAR.md](./API_ROUTE_STANDAR.md) (flat), [API-ARCHITECT_AND_ROUTE_STANDARD.md](./API-ARCHITECT_AND_ROUTE_STANDARD.md) (domain)

Tài liệu này so sánh **thực tế** với chuẩn và chi tiết hóa kế hoạch khắc phục. **Hướng đi đã chốt trong phân tích:** Giai đoạn 1 = flat (identifier + nested, không domain prefix); Giai đoạn 2 = optional migration sang domain (API-ARCHITECT).

---

## 1. So sánh thực tế vs chuẩn

### 1.1 Pods

| Chuẩn (API_ROUTE_STANDAR.md) | Thực tế hiện tại | Gap |
|------------------------------|------------------|-----|
| GET /api/v1/pods | GET /api/v1/pods | ✅ Khớp |
| GET /api/v1/pods/:uid | GET /api/v1/pods/by-uid/:uid, GET /api/v1/pods/by-id/:id | ❌ Có by-uid/by-id; chuẩn chỉ :uid. Chuẩn không expose DB id. |
| GET /api/v1/pods/:uid/processes | GET /api/v1/pods/:podUid/processes | ❌ Param phải là :uid |
| GET /api/v1/pods/:uid/network | GET /api/v1/pods/:podUid/network-connections | ❌ Path "network" vs "network-connections"; param :uid |
| GET /api/v1/pods/:uid/events | GET /api/v1/pods/:podUid/events | ❌ Param :uid |
| GET /api/v1/pods/:uid/runtime-metrics | GET /api/v1/pods/:podUid/runtime-metrics | ❌ Param :uid |
| GET /api/v1/pods/:uid/capabilities | GET /api/v1/pods/:podUid/capabilities | ❌ Param :uid |
| GET /api/v1/pods/:uid/spec | GET /api/v1/pods/:podUid/spec | ❌ Param :uid |
| GET /api/v1/pods/:uid/sbom | GET /api/v1/sbom/:podUid | ❌ Phải nest: /pods/:uid/sbom |
| GET /api/v1/pods/:uid/risk-report | GET /api/v1/risks/pods/:podUid/report | ❌ Phải nest: /pods/:uid/risk-report |
| GET /api/v1/pods/:uid/runtime-risk | GET /api/v1/runtime-risk/pods/:podUid | ❌ Phải nest: /pods/:uid/runtime-risk |

**Handlers đang dùng:** `c.Param("podUid")` (pod_detail_services_handlers, pod_capability_handlers, pod_spec_yaml, sbom_handlers, attack_steps_handlers, runtime_signals_handlers, runtime_risk_handlers, risk_reports), `c.Param("uid")` (GetPodByUID), `c.Param("id")` (GetPod).

---

### 1.2 ServiceAccounts

| Chuẩn | Thực tế | Gap |
|-------|---------|-----|
| GET /api/v1/serviceaccounts | GET /api/v1/serviceaccounts | ✅ |
| GET /api/v1/serviceaccounts/:uid | GET /api/v1/serviceaccounts/by-uid/:saUid, GET /api/v1/serviceaccounts/:id | ❌ Chuẩn chỉ :uid; bỏ by-uid, thống nhất :uid |
| GET /api/v1/serviceaccounts/:uid/permissions | GET /api/v1/serviceaccounts/:id/permissions | ❌ Param :uid |

**Handlers:** GetServiceAccountByUID dùng `c.Param("saUid")`, GetServiceAccount/GetServiceAccountPermissions dùng `c.Param("id")`.

---

### 1.3 Audit

| Chuẩn | Thực tế | Gap |
|-------|---------|-----|
| GET /api/v1/audit/logs | GET /api/v1/audit, GET /api/v1/audit-logs | ❌ Trùng; chuẩn chỉ /audit/logs |
| GET /api/v1/audit/reports | GET /api/v1/audit/reports, GET /api/v1/reports | ❌ Trùng; chuẩn chỉ /audit/reports |

---

### 1.4 Risk

| Chuẩn | Thực tế | Gap |
|-------|---------|-----|
| GET /api/v1/risk/scores | GET /api/v1/risk/scores | ✅ |
| GET /api/v1/pods/:uid/risk-score | Không có | ⚠️ Chuẩn có; hiện có /risk/scores/:uid |
| RPC → REST: POST /pods/:uid/risk-score/recalculate | POST /api/v1/risk/scores/:uid/calculate | ❌ Doc đề xuất REST style |

---

### 1.5 Runtime risk / Attack steps / Runtime signals (nested)

| Chuẩn | Thực tế | Gap |
|-------|---------|-----|
| /pods/:uid/runtime-risk | /runtime-risk/pods/:podUid, /runtime-risk/pods/:podUid/events | Chuyển sang /pods/:uid/runtime-risk, /pods/:uid/runtime-risk/events |
| (attack steps) | /attack-steps/pods/:podUid | Chuẩn nested: /pods/:uid/attack-steps |
| (runtime signals) | /runtime-signals/pods/:podUid | Chuẩn nested: /pods/:uid/runtime-signals |

---

### 1.6 Graph

| Chuẩn | Thực tế | Gap |
|-------|---------|-----|
| GET /api/v1/graph | GET /api/v1/graph | ✅ |
| GET /api/v1/graph/attack-paths/:uid | GET /api/v1/graph/attack-paths/:uid | ✅ |
| GET /api/v1/graph/blast-radius/:uid | GET /api/v1/graph/blast-radius/:id | Param nên :uid (doc) |

---

### 1.7 Router order / structure

- **Thực tế:** Một file routes.go lớn, comment "Register FIRST to avoid route conflict", "Register BEFORE /pods/:id".
- **Chuẩn:** Tách module (routes_pods.go, routes_risk.go, …), không phụ thuộc thứ tự đăng ký.

---

## 2. Rủi ro khi áp dụng chuẩn

1. **Gin param name:** Toàn bộ pod sub-routes phải dùng **cùng một tên param** (ví dụ `:uid`) ở một cấp path. Đổi `:podUid` → `:uid` và dùng `c.Param("uid")` trong handler là đủ, không đụng Gin.
2. **Breaking change:** Frontend và bất kỳ client nào đang gọi `/pods/by-id/:id`, `/pods/by-uid/:uid`, `/risks/pods/:uid/report`, `/sbom/:uid`, … sẽ lỗi nếu đổi route mà không có compatibility layer.
3. **GetPod by DB id:** Chuẩn nói "do not expose database IDs". Nếu bỏ hẳn GET /pods/by-id/:id thì list/detail từ Resources (click pod.id) phải chuyển sang dùng UID; list đã có `pod.uid`.

---

## 3. Kế hoạch khắc phục theo phase

### Phase 1 – Chuẩn hóa Pod (identifier + nested, không breaking)

**Mục tiêu:** Một bộ route chuẩn `/pods/:uid` và `/pods/:uid/...`, param duy nhất `:uid`, không xóa route cũ ngay.

1. **Backend**
   - Thêm route chuẩn (đăng ký **sau** các route hiện tại để ưu tiên route cũ):
     - GET `/pods/:uid` → handler hiện GetPodByUID (đọc `c.Param("uid")`).
     - GET `/pods/:uid/processes`, `.../network`, `.../events`, `.../runtime-metrics`, `.../capabilities`, `.../spec` → wrapper gọi handler cũ, truyền `c.Param("uid")` vào logic (hoặc đổi handler cũ sang đọc "uid").
   - **Quan trọng:** Trong Gin, nếu đã có `/pods/by-id/:id` và `/pods/by-uid/:uid`, khi thêm `/pods/:uid` sẽ conflict với `/pods/:podUid/...` vì cùng dạng một segment. Giải pháp: **xóa** `/pods/by-id/:id` và `/pods/by-uid/:uid`, chỉ giữ **một** route GET `/pods/:uid` (GetPodByUID) và tất cả sub-route dùng **cùng** `:uid`: GET `/pods/:uid/processes`, … Khi đó không còn conflict.
   - Đổi toàn bộ pod sub-route hiện tại từ `:podUid` sang `:uid`, và trong handler đổi `c.Param("podUid")` → `c.Param("uid")`.
   - Bỏ GET `/pods/by-id/:id` (hoặc trả 410 Gone + header Deprecation). Bỏ GET `/pods/by-uid/:uid` (thay bằng GET `/pods/:uid`).
   - Đổi path: `/pods/:uid/network-connections` → `/pods/:uid/network` (chuẩn doc).
2. **Dashboard**
   - Pod detail: dùng UID làm key (list đã có uid). Gọi GET `/api/v1/pods/${uid}` thay vì by-uid hay by-id.
   - Các API con: `/pods/${uid}/processes`, `/pods/${uid}/network`, … (và `/pods/${uid}/runtime-metrics` nếu tên giữ nguyên).
3. **Kiểm tra**
   - E2E / test: cập nhật URL và param; chạy test pod detail, pod sub-resources.

**Deliverable:** Pod API đúng chuẩn doc (/:uid và /:uid/...), một identifier, không conflict Gin.

---

### Phase 2 – Nested resource (runtime-risk, sbom, risk-report, attack-steps, runtime-signals)

**Mục tiêu:** Chuyển resource “theo pod” từ prefix riêng sang nest dưới `/pods/:uid/...`.

1. **Backend**
   - Thêm (hoặc thay) route:
     - GET `/pods/:uid/sbom` (thay GET `/sbom/:podUid`).
     - GET `/pods/:uid/risk-report` (thay GET `/risks/pods/:podUid/report`).
     - GET `/pods/:uid/runtime-risk`, GET `/pods/:uid/runtime-risk/events` (thay /runtime-risk/pods/:podUid và .../events).
     - GET `/pods/:uid/attack-steps` (thay /attack-steps/pods/:podUid).
     - GET `/pods/:uid/runtime-signals` (thay /runtime-signals/pods/:podUid).
   - Handlers: đọc `c.Param("uid")` (đã thống nhất từ Phase 1).
   - Giữ route cũ với header Deprecation (optional) hoặc xóa sau khi dashboard chuyển xong.
2. **Dashboard**
   - Đổi URL: `/sbom/${uid}` → `/pods/${uid}/sbom`, `/risks/pods/${uid}/report` → `/pods/${uid}/risk-report`, tương tự runtime-risk, attack-steps, runtime-signals.
3. **Tests**
   - Cập nhật test gọi đúng path mới.

**Deliverable:** Cấu trúc nested đúng chuẩn, không còn “resource không hierarchical”.

---

### Phase 3 – ServiceAccounts chuẩn :uid

1. **Backend**
   - Thêm GET `/serviceaccounts/:uid` (trỏ GetServiceAccountByUID), GET `/serviceaccounts/:uid/permissions`.
   - Đổi handler: đọc `c.Param("uid")` thay vì "saUid" / "id" cho các route dùng UID.
   - Deprecate hoặc xóa GET `/serviceaccounts/by-uid/:saUid`. Quyết định giữ/xóa GET `/serviceaccounts/:id` (theo chuẩn chỉ dùng uid).
2. **Dashboard**
   - Gọi GET `/serviceaccounts/${uid}` và `/serviceaccounts/${uid}/permissions` (khi mở từ Risk/Identity bằng uid).

**Deliverable:** ServiceAccount chỉ dùng :uid, không by-uid, không :id (hoặc chỉ deprecate :id).

---

### Phase 4 – Audit gộp endpoint

1. **Backend**
   - Giữ duy nhất: GET `/audit/logs`, GET `/audit/reports` (cùng handler với GetAuditLogs, GetAuditReports).
   - Xóa hoặc redirect 301: `/audit`, `/audit-logs`, `/reports` → `/audit/logs` hoặc `/audit/reports`.
2. **Dashboard**
   - Đổi gọi audit từ `/audit` hoặc `/audit-logs` sang `/audit/logs`; reports sang `/audit/reports`.

**Deliverable:** Không còn duplicate audit endpoints.

---

### Phase 5 – Cấu trúc router (modules, không phụ thuộc thứ tự)

1. Tách file: `routes.go` (SetupRoutes, register*), `routes_pods.go`, `routes_risk.go`, `routes_graph.go`, `routes_audit.go`, `routes_serviceaccounts.go`, …
2. Mỗi nhóm: `registerPodRoutes(api)`, `registerRiskRoutes(api)`, … Đăng ký theo thứ tự cố định, không comment “register FIRST”.
3. Trong từng module: route cụ thể (path dài) đăng ký trước path có param (ví dụ /pods/.../risk-report trước /pods/:uid nếu có conflict tiềm ẩn).

**Deliverable:** Code dễ bảo trì, API contract rõ, không còn “router order hack”.

---

### Phase 6 (tùy chọn) – Risk RPC → REST, Graph :id → :uid

- POST `/risk/scores/:uid/calculate` → POST `/pods/:uid/risk-score/recalculate` (nếu product cần REST thuần).
- Graph: đổi `/graph/blast-radius/:id` sang `:uid` nếu resource đó là UID.

---

## 4. Thứ tự thực hiện đề xuất

| Thứ tự | Phase | Lý do |
|--------|--------|--------|
| 1 | Phase 1 – Pod identifier + param :uid | Giải quyết Gin conflict, chuẩn hóa pod, nền cho mọi route pod |
| 2 | Phase 4 – Audit | Ít phụ thuộc, gộp nhanh, giảm duplicate |
| 3 | Phase 2 – Nested pod (sbom, risk-report, runtime-risk, …) | Dùng lại :uid từ Phase 1, thống nhất resource hierarchy |
| 4 | Phase 3 – ServiceAccount | Chuẩn :uid, tách biệt với pod |
| 5 | Phase 5 – Tách module routes | Refactor code, không đổi contract nếu đã xong 1–4 |
| 6 | Phase 6 (optional) | Cải thiện REST, ít ưu tiên hơn |

---

## 5. Checklist trước khi bắt đầu Phase 1

- [ ] Đồng ý bỏ (hoặc deprecate) GET /pods/by-id/:id; toàn bộ pod detail dùng UID.
- [ ] Dashboard: xác nhận list pods trả về `uid` và dùng uid cho navigation đến pod detail.
- [ ] Backup / tag repo trước khi đổi route.
- [ ] Cập nhật API_ROUTE_STANDAR.md nếu có điều chỉnh (ví dụ tạm giữ by-id với Deprecation).

Sau khi hoàn thành từng phase, nên cập nhật tài liệu chuẩn và OpenAPI/spec (nếu có) để contract và thực tế luôn khớp.

---

## Tóm tắt gap (trước khi khắc phục)

| Nhóm | Số gap chính | Hành động |
|------|----------------|-----------|
| Pods | Nhiều: by-id/by-uid/:podUid, path không nested | Phase 1+2: một :uid, nested /pods/:uid/... (flat) |
| ServiceAccounts | by-uid/:saUid, :id | Phase 3: chỉ :uid |
| Audit | 4 endpoint trùng chức năng | Phase 4: chỉ /audit/logs, /audit/reports |
| Runtime risk / SBOM / Risk report | Không nested dưới pod | Phase 2: chuyển vào /pods/:uid/... (flat) |
| Router | Một file, phụ thuộc thứ tự | Phase 5: tách module |

**Lưu ý:** Bảng mapping chi tiết **Hiện tại → Target (Flat)** và kế hoạch **Giai đoạn 2 (Domain)** nằm trong [API_ARCHITECT_ANALYSIS_AND_PLAN.md](./API_ARCHITECT_ANALYSIS_AND_PLAN.md). Các phase dưới đây áp dụng cho **Giai đoạn 1 (Flat)** theo nhận định trong phân tích.
