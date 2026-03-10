# Dashboard Page-level UX Specification

Tài liệu này mô tả **CHI TIẾT TỪNG PAGE** trong dashboard: hiển thị gì, người dùng dùng ra sao, có button / filter nào, và tương tác hợp lệ. Đây là **UI/UX spec ở mức implementable**, dùng trực tiếp cho frontend + backend.

---

## 1. Dashboard (Home)

### Mục tiêu sử dụng

* Cái nhìn nhanh về **tình trạng an ninh tổng thể**
* Điểm vào (entry point) cho điều tra

### Nội dung hiển thị

* Cluster selector (global)
* Summary cards:

  * Total Risks (Critical / High / Medium / Low)
  * Exposed Capabilities (PCE)
  * Affected Workloads
* Top Risks (5–10)
* Recent Changes / Events

### Tương tác

* Click severity card → Risk Center (pre-filtered)
* Click capability count → Capabilities page
* Click cluster name → Cluster Detail

### Controls

* Button: Refresh
* Filter: Cluster (global)

---

## 2. Cluster List

### Mục tiêu

* Quản lý & chọn cluster

### Hiển thị

* Table: Cluster Name, Version, Health, Risk Count, Agents

### Tương tác

* Click row → Cluster Detail

### Controls

* Search by name
* Filter: Health status

---

## 3. Cluster Detail

### Hiển thị

* Header: Cluster name, distro, version
* Tabs:

  * Overview
  * Inventory
  * Agents
  * Security Summary

#### Overview

* Node count, namespace count, pod count

#### Inventory

* Nodes list
* Namespaces list

#### Agents

* Agent status, heartbeat, version

#### Security Summary

* Risk by severity
* Capability exposure summary

### Tương tác

* Click node → Node Detail
* Click risk count → Risk Center (cluster scoped)

### Controls

* Filter: Namespace

---

## 4. Node Detail

### Hiển thị

* Node metadata: role, OS, runtime
* Tabs:

  * Overview
  * Workloads
  * Risks

### Tương tác

* Click pod → Pod Detail

### Controls

* Filter workloads by namespace

---

## 5. Workloads / Pod List

### Hiển thị

* Table: Pod name, namespace, node, risk count

### Tương tác

* Click pod → Pod Detail

### Controls

* Search
* Filter: Namespace, Risk severity

---

## 6. Pod Detail (CORE PAGE)

### Mục tiêu

* Trung tâm phân tích kỹ thuật

### Tabs & Nội dung

#### Overview

* Image, labels, namespace, node

#### Runtime

* Containers, ports, processes

#### Security Context

* Privileged, capabilities, seccomp

#### SBOM

* Package list
* CVE mapping

#### RBAC Context

* ServiceAccount
* Roles / bindings

#### Related Risks

* Risks impacting this pod

### Tương tác

* Click CVE → CVE external
* Click role → Identity Detail
* Click risk → Risk Detail

### Controls

* Filter SBOM by severity

---

## 7. Risk Center

### Hiển thị

* Table: Risk ID, Severity, Type, Assets, Status

### Tương tác

* Click row → Risk Detail

### Controls

* Filter: Severity, Status, Asset type
* Search

---

## 8. Risk Detail

### Hiển thị

* Summary banner
* Affected Assets
* Evidence
* Violated Rules
* Timeline

### Tương tác

* Click asset → Pod / Identity Detail
* Click rule → Rule Detail

### Controls

* Button: Mark as resolved (nếu có)

---

## 9. Capabilities (PCE)

### Hiển thị

* List capabilities
* Severity, affected assets

### Tương tác

* Click capability → Capability Detail

### Controls

* Filter: Severity

---

## 10. Capability Detail

### Hiển thị

* Description
* Preconditions
* Attack steps
* Affected pods

### Tương tác

* Click pod → Pod Detail

---

## 11. Identities (RBAC)

### Hiển thị

* List ServiceAccounts / Roles

### Tương tác

* Click → Identity Detail

### Controls

* Filter: Namespace, Type

---

## 12. Identity Detail

### Hiển thị

* Permissions matrix
* Bound workloads
* Related risks

### Tương tác

* Click pod → Pod Detail
* Click risk → Risk Detail

---

## 13. Rules & Policies

### Hiển thị

* Rule list

### Tương tác

* Click rule → Rule Detail

### Controls

* Filter: Severity

---

## 14. Rule Detail

### Hiển thị

* Rule intent
* Definition
* Violations

### Tương tác

* Click violation → Risk Detail

---

## 15. Attack Paths

### Hiển thị

* Graph visualization

### Tương tác

* Click node → Object Detail

### Controls

* Filter: Severity, Asset type

---

## 16. Monitoring

### Hiển thị

* Agent health
* Sync status
* Errors

### Tương tác

* None (read-only)

---

## 17. Global UI Controls

* Global Cluster Selector
* Global Time Range (nếu có)
* Manual Refresh

---

## 18. UX Guardrails (BẮT BUỘC)

* Không hiển thị tab nếu backend chưa có API
* Không button "tương lai"
* Không click dẫn tới empty page
* Mọi số liệu phải trace được về agent/core

---

## 19. Definition of Done (UI)

* Người dùng hiểu được:

  * Rủi ro là gì
  * Ảnh hưởng tới đâu
  * Vì sao xảy ra
  * Xử lý ở đâu

---

## 20. Layout & Visual Structure Specification

### 20.1 Nguyên tắc layout tổng quát

* Thiết kế theo **desktop-first** (≥1440px), responsive xuống 1280px và 1024px
* Grid chuẩn: **12-column grid**, gutter 16–24px
* Không thiết kế mobile trong giai đoạn này
* Ưu tiên **đọc – quét – so sánh**, không ưu tiên animation

---

### 20.2 Kích thước & bố cục chung

#### Header toàn cục

* Height: 56px
* Chứa:

  * Logo
  * Global Cluster Selector
  * Time / Refresh indicator
  * User / Settings

#### Left Navigation

* Width: 240px (fixed)
* Có thể collapse còn 64px
* Không scroll cùng content

#### Main Content Area

* Max width: 1440px
* Padding: 24px
* Scroll theo trang

---

### 20.3 Dashboard Layout

#### Cấu trúc

```
[Header]
[Summary Cards Row]
[Risk Overview | Capability Overview]
[Top Risks Table]
```

#### Summary Cards

* 4–6 cards
* Width: 1 card = 3 columns
* Height: ~120px
* Clickable toàn card

---

### 20.4 List / Table Pages Layout (Cluster, Risk, Workload)

#### Cấu trúc

```
[Page Title + Filters]
[Table]
[Pagination]
```

* Filter bar cao 48–56px
* Table:

  * Sticky header
  * Row height: 44–48px
* Pagination bottom-right

---

### 20.5 Detail Page Layout (CHUẨN)

Áp dụng cho: Pod, Risk, Identity, Rule, Capability

#### Cấu trúc chuẩn

```
[Object Header]
[Context Summary]
[Tabs]
[Tab Content]
```

#### Object Header

* Height: ~72px
* Hiển thị:

  * Object name (primary)
  * Namespace / Cluster (secondary)
  * Status / Severity badge

#### Context Summary

* 2–4 cards ngang
* Chỉ số cốt lõi, không scroll

---

### 20.6 Tabs Layout

* Tab bar sticky dưới header
* Không quá 6 tabs
* Mỗi tab:

  * Có empty-state rõ ràng
  * Không nested tab quá 1 cấp

---

### 20.7 SBOM & RBAC Specialized Layout

#### SBOM

* 2-column layout:

  * Left: Package list (table)
  * Right: CVE / metadata

#### RBAC

* Permission matrix:

  * Rows: resources
  * Columns: verbs
* Scroll ngang cho matrix lớn

---

### 20.8 Graph / Attack Path Layout

* Full-width canvas
* Min height: 600px
* Side panel (right): 360px
* Không render graph trong modal

---

### 20.9 Responsive Rules

* <1280px:

  * Collapse side panels
  * Tables → horizontal scroll
* Không ẩn thông tin quan trọng

---

### 20.10 Visual Hierarchy Rules

* Severity = màu + icon (không chỉ màu)
* Critical luôn ở top-left / first
* Không dùng màu để convey 2 ý nghĩa

---

### 20.11 Definition of Done – Layout

* Không page nào vượt quá 2 scroll cho content chính
* Không table > 10 columns
* Không thông tin quan trọng nằm dưới fold mặc định

---

**END OF SPEC**
