# Dashboard UX Specification

## 1. Mục tiêu & Nguyên tắc thiết kế

### Mục tiêu

* Xây dựng dashboard **đáng tin cậy**, không hiển thị dữ liệu giả hoặc suy diễn
* Điều hướng theo **object-centric model** (Cluster, Workload, Risk, Identity…)
* Người dùng click để **hiểu – phân tích – ra quyết định**, không click để “xem thử”

### Nguyên tắc cốt lõi

* Mỗi object có **Detail Page riêng**
* Dashboard chỉ là **entry & summary**, không phải nơi drill-down sâu
* UI chỉ hiển thị khi **agent + core đã xử lý thật**

---

## 2. Page Tree (Cây trang chuẩn)

```
Dashboard
├── Clusters
│   └── Cluster Detail
│       ├── Overview
│       ├── Inventory (Nodes / Namespaces)
│       ├── Agents
│       └── Security Summary
│
├── Resources
│   ├── Workloads
│   │   └── Pod Detail
│   │       ├── Overview
│   │       ├── Runtime
│   │       ├── Security Context
│   │       ├── SBOM
│   │       ├── RBAC Context
│   │       └── Related Risks
│   └── Nodes
│       └── Node Detail
│
├── Risk Center
│   └── Risk Detail
│       ├── Summary
│       ├── Affected Assets
│       ├── Evidence
│       ├── Violated Rules
│       └── Timeline
│
├── Capabilities (PCE)
│   └── Capability Detail
│
├── Identities (RBAC)
│   └── Identity Detail
│
├── Rules & Policies
│   └── Rule Detail
│
├── Attack Paths
│   └── Attack Graph Explorer
│
└── Monitoring
    └── Agent & System Health
```

---

## 3. Navigation Map (Luồng điều hướng chuẩn)

### Dashboard → Drill-down

* Dashboard

  * Click **Risk Count** → Risk Center (filtered)
  * Click **Capability Count** → Capabilities list
  * Click **Cluster Name** → Cluster Detail

### Risk-centric Flow (Mental Model chính)

```
Dashboard
 → Risk Center
   → Risk Detail
     → Affected Pod
       → Pod Detail
         → SBOM / RBAC / Capability
```

### Resource-centric Flow

```
Cluster Detail
 → Inventory
   → Pod
     → Pod Detail
       → Related Risks
         → Risk Detail
```

### Rule / RBAC Flow

```
Risk Detail
 → Violated Rule
   → Rule Detail

Pod Detail
 → RBAC Context
   → Identity Detail
```

---

## 4. API Contract chuẩn cho từng Detail Page

### 4.1 Cluster Detail

**Page:** `/clusters/:clusterId`

**APIs:**

```http
GET /api/v1/clusters/:id
GET /api/v1/clusters/:id/overview
GET /api/v1/clusters/:id/inventory
GET /api/v1/clusters/:id/security-summary
```

**Refresh:** 5 phút hoặc manual refresh

---

### 4.2 Pod (Workload) Detail

**Page:** `/resources/pods/:uid`

**APIs:**

```http
GET /api/v1/pods/:uid
GET /api/v1/pods/:uid/runtime
GET /api/v1/pods/:uid/security-context
GET /api/v1/pods/:uid/sbom
GET /api/v1/pods/:uid/rbac
GET /api/v1/pods/:uid/risks
```

**Refresh:**

* Runtime: 30–60s
* Security data: event-driven / scan-complete

---

### 4.3 Risk Detail

**Page:** `/risks/:riskId`

**APIs:**

```http
GET /api/v1/risks/:id
GET /api/v1/risks/:id/assets
GET /api/v1/risks/:id/evidence
GET /api/v1/risks/:id/violated-rules
GET /api/v1/risks/:id/timeline
```

**Refresh:** near-real-time (event-based)

---

### 4.4 SBOM Detail (contextual)

**Hiển thị ở:** Pod Detail → SBOM tab

**APIs:**

```http
GET /api/v1/sbom/pods/:uid
GET /api/v1/sbom/:sbomId/components
```

**Nguyên tắc:**

* Không có SBOM → không render tab

---

### 4.5 Capability (PCE) Detail

**Page:** `/capabilities/:capabilityId`

**APIs:**

```http
GET /api/v1/capabilities/:id
GET /api/v1/capabilities/:id/affected-assets
GET /api/v1/capabilities/:id/attack-steps
```

---

### 4.6 Identity / RBAC Detail

**Page:** `/identities/:type/:uid`

**APIs:**

```http
GET /api/v1/rbac/identities/:uid
GET /api/v1/rbac/identities/:uid/permissions
GET /api/v1/rbac/identities/:uid/bindings
GET /api/v1/rbac/identities/:uid/risks
```

---

### 4.7 Rule Detail

**Page:** `/rules/:ruleId`

**APIs:**

```http
GET /api/v1/rules/:id
GET /api/v1/rules/:id/violations
GET /api/v1/rules/:id/history
```

---

## 5. Mental Model Chuẩn (Tổng hợp luồng click)

### Người dùng tự hỏi:

1. *“Tôi đang gặp rủi ro gì?”* → Dashboard / Risk Center
2. *“Rủi ro này ảnh hưởng tới đâu?”* → Risk Detail
3. *“Tài nguyên cụ thể là gì?”* → Pod / Identity Detail
4. *“Vì sao nó xảy ra?”* → SBOM / Capability / Rule
5. *“Tôi xử lý thế nào?”* → Context + Evidence

---

## 6. Tiêu chí nghiệm thu (Definition of Done)

* Không hiển thị mock / hardcode data
* Mỗi số liệu dashboard trace được về:

  * Agent signal
  * Core computation
* Không có API → không có UI
* Không có click dẫn tới trang rỗng
* Dashboard không over-scope chức năng

---

## 7. Ghi chú triển khai

* Spec này là **source of truth** cho:

  * UI/UX
  * Backend API
  * Agent data collection
* Mọi thay đổi scope phải cập nhật lại spec

### 7.1 Đã áp dụng (2026-02-02)

* **Navigation**: Sidebar theo đúng Page Tree – Dashboard, Clusters, Resources, Risk Center, Capabilities, Identities (RBAC), Rules & Policies, Attack Paths, Monitoring, Settings. Đã bỏ khỏi sidebar: SBOM Analysis (chỉ contextual trong Pod Detail), Certificates, Reports, Audit Logs, Error Logs, Notifications – các trang này vẫn tồn tại và có thể truy cập qua Monitoring (nút Audit Logs, Error Logs, Certificates, Reports) hoặc URL trực tiếp.
* **Identities**: Mục "Identities (RBAC)" → redirect `/identities` → `/resources?tab=ServiceAccount` (Resources hỗ trợ query `?tab=ServiceAccount`).
* **SBOM**: `/sbom` redirect → `/resources` (SBOM theo spec chỉ hiển thị trong Pod Detail).
* **Dashboard drill-down**: Click Clusters/Risks/Pods/Agents stat card → điều hướng tới Clusters, Risk Center, Resources, Monitoring; Cluster Health và Pod Capabilities có link tới Clusters, Capabilities.
* **Monitoring**: Tiêu đề "Monitoring (Agent & System Health)"; nút Audit Logs, Error Logs, Certificates, Reports để mở các trang phụ.
