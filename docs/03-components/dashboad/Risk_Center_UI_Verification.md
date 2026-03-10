# Risk Center – Đối chiếu Spec vs UI

*Cập nhật: 2026-02-02*

Tài liệu đối chiếu **Dashboard Page-level UX Specification** (Section 7–8) và **Gap Analysis** với giao diện hiện tại Risk Center và Risk Detail.

---

## 1. Risk Center (trang `/risks` – Insights.tsx)

### Spec (Section 7)

| Yêu cầu | Mô tả |
|--------|--------|
| **Hiển thị** | Bảng: Risk ID, Severity, Type, Assets, Status |
| **Tương tác** | Click row → Risk Detail |
| **Controls** | Filter: Severity, Status, Asset type; Search |

### Hiện trạng UI

| Spec | UI hiện tại | Khớp? |
|------|-------------|--------|
| **Risk ID** | Cột "Title": hiển thị `title` và `(cveId ?? id)` trong ngoặc | ✅ |
| **Severity** | Cột Severity (icon + màu) | ✅ |
| **Type** | Chưa có cột Type (insight_type: vulnerability, rbac, …) | ❌ Thiếu |
| **Assets** | Cột Namespace (1 namespace); chưa có "Assets" (số/tóm tắt) | ⚠️ Một phần |
| **Status** | Cột Status (active, resolved, acknowledged) | ✅ |
| **Click row → Risk Detail** | `onClick` row → `navigate(\`/risks/${risk.id}\`)` | ✅ |
| **Filter Severity** | Buttons: all, critical, high, medium, low | ✅ |
| **Filter Status** | Chưa có dropdown/button filter theo status | ❌ Thiếu |
| **Filter Asset type** | Chưa có filter resource_type (Pod, ServiceAccount) | ❌ Thiếu |
| **Search** | Ô search "Search risks, pods, resources..." | ✅ |

### Phần mở rộng so với spec

- **Tabs:** Risks, PCE, Reference (spec không nêu; phù hợp với PCE và Runtime Signals).
- **Summary bar:** Total, Critical/High/Medium/Low counts, Resolved (24h).
- **Runtime / Escape signals:** Khối hiển thị tín hiệu runtime; link "View all" → tab Reference.
- **Pagination:** page, pageSize, total.
- **Actions:** Nút "Details", "Attack Path" trên mỗi row.

### Đã bổ sung (2026-02-02)

1. **Cột Type:** Hiển thị `insightType` hoặc `category` (vulnerability, rbac, …); ẩn trên màn nhỏ (hidden sm:table-cell).
2. **Cột Assets:** Hiển thị "1 Pod" / "1 ServiceAccount" hoặc "N resources" từ `affectedResources`.
3. **Filter Status:** Buttons All, Active, Resolved, Acknowledged; gửi `?status=` tới API khi khác All.
4. **Filter Asset type:** Chưa thêm (có thể bổ sung sau với `?resource_type=`).

---

## 2. Risk Detail (trang `/risks/:id` – RiskDetail.tsx)

### Spec (Section 8)

| Yêu cầu | Mô tả |
|--------|--------|
| **Hiển thị** | Summary banner, Affected Assets, Evidence, Violated Rules, Timeline |
| **Tương tác** | Click asset → Pod/Identity Detail; Click rule → Rule Detail |
| **Controls** | Button: Mark as resolved |

### Hiện trạng UI

| Spec | UI hiện tại | Khớp? |
|------|-------------|--------|
| **Summary banner** | Card: severity, status, score, description, impact (recommendation) | ✅ |
| **Affected Assets** | Card "Affected Assets": list resource + nút "View" → Pod/Identity | ✅ |
| **Evidence** | Card "Evidence & Violated Rules" khi `insight.evidence != null` | ✅ |
| **Violated Rules** | Cùng card, hiển thị khi `insight.violatedRules != null` | ✅ |
| **Timeline** | Card "Timeline": Detected, Updated, Resolved (timestamp, updatedAt, resolvedAt) | ✅ |
| **Click asset → Pod/Identity** | Nút View → `/resources/pods/uid/:id` hoặc `/identities/uid/:id` | ✅ |
| **Click rule → Rule Detail** | Chưa có link (violated rules hiển thị JSON) | ⚠️ Backend chưa trả rule_id |
| **Mark as resolved** | Nút "Mark as resolved" gọi `api.resolveInsight(id)` | ✅ |
| **Back to Risk Center** | Nút "Back to Risk Center" → `/risks` | ✅ |

### Phần mở rộng so với spec

- **Runtime / Escape signals:** Card hiển thị runtime signals của các Pod bị ảnh hưởng (gọi `getRuntimeSignalsByPod` cho tối đa 3 pod).

### Khuyến nghị

1. **Rule Detail link:** Khi backend trả về `rule_id` hoặc rule identifier trong violated rules, thêm link tới `/rules/:id`.

---

## 3. Labels & Mô tả

| Nguồn | Nội dung |
|-------|----------|
| **RISK_CENTER_DESCRIPTION** (constants/labels.ts) | "Vulnerability findings (same count as Dashboard Security Risks). Triage, investigate, and remediate." |
| **Layout nav** | "Risk Center" → path `/risks` |
| **Dashboard → Risk Center** | Click severity card → `navigate(\`/risks?severity=...\`)` (pre-filtered) |

---

## 4. Tóm tắt

| Trang | Khớp spec | Thiếu / cần bổ sung |
|-------|------------|------------------------|
| **Risk Center** | Bảng Risk ID, Severity, Status; click row → Detail; filter Severity; Search | Cột Type, cột Assets; filter Status; filter Asset type (optional) |
| **Risk Detail** | Summary, Affected Assets, Evidence, Violated Rules, Timeline, Mark as resolved, link asset | Link Violated Rule → Rule Detail (khi backend có rule_id) |

Sau khi bổ sung cột Type, Assets và filter Status (và tùy chọn Asset type), Risk Center sẽ khớp đầy đủ mô tả spec Section 7.
