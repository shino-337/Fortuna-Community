# Risk Center – Kiểm tra Dashboard đã hiển thị các thay đổi

Tài liệu này đối chiếu **từng cải tiến Risk Center** với **vị trí hiển thị thực tế** trên Dashboard để bạn kiểm tra "đã thấy thay đổi chưa".

---

## 0. Loại dashboard đang chạy & cách kiểm tra

- **Chỉ có một loại dashboard:** build từ thư mục `dashboard/` trong repo KSAM, không có variant "lite" hay "full" tách biệt. Mọi tính năng (kể cả Export CSV/PDF) nằm trong cùng một codebase.
- **Deploy:** Kubernetes dùng image `fortuna-dashboard:latest` (xem `deploy/dashboard-deployment.yaml`). Image được build bởi `dashboard/Dockerfile` (npm run build từ `dashboard/`).
- **Cách xác nhận build đang chạy:**
  1. **Trên giao diện:** cuối trang (footer) có dòng **"Fortuna Dashboard · build &lt;timestamp UTC&gt;"**. Mỗi lần build mới, timestamp đổi; so sánh với thời điểm build image để biết có đang chạy đúng bản mới không.
  2. **Trên cluster:** `kubectl get deployment fortuna-dashboard -n fortuna -o jsonpath='{.spec.template.spec.containers[0].image}'` và (nếu dùng image tag theo ngày) so sánh với image vừa build.
- **Export CSV / Export PDF:** có trong code tại **Risk Center → tab "Risk Findings"** (cùng dòng với Total findings, Scope, Time window). Nếu không thấy: (1) đảm bảo đang ở tab **Risk Findings** (không phải PCE hay Evidence & References), (2) rebuild image dashboard và rollout lại, (3) hard refresh trình duyệt (Ctrl+Shift+R).

---

## 1. Cách vào Risk Center trên Dashboard

- **Sidebar (trái):** mục **Security** → **Risk Center** (icon ShieldAlert).
- **URL:** `/#/risks` (hoặc `http://localhost:8081/#/risks` nếu port-forward 8081:80).
- Nếu không thấy: đảm bảo đã đăng nhập và đang dùng Layout có sidebar (không phải trang Login).

---

## 2. Bảng: Thay đổi ↔ Hiển thị ở đâu

| # | Thay đổi (Phase) | Đã có trong code? | Hiển thị ở đâu (đường đi trong UI) | Điều kiện / Ghi chú |
|---|------------------|-------------------|-------------------------------------|----------------------|
| 1 | **Risks + score & priority (Phase 3.1)** | Có: `getRisks({ withScores, priorityLevel })`, sort score | **Risk Center** → tab **"Risk Findings"** → hàng filter có **Priority** (dropdown P0–P4) và **Sort** (Risk score high/low) → bảng có cột **Risk Score** (score/100 + P0–P4). | Cột "Risk Score" luôn có; Priority filter và sort "Risk score" nằm **bên phải** filter Severity/Workflow, có thể cần cuộn ngang trên màn nhỏ. |
| 2 | **Export CSV (Phase 1.3)** | Có: `exportRisksCSV()` | **Risk Center** → tab **"Risk Findings"** → **cùng dòng** với "Total findings", "Scope", "Time window" → hai nút **Export CSV** và **Export PDF**. | Chỉ hiện khi tab đang chọn là **Risk Findings** (không phải PCE hay Evidence & References). |
| 3 | **Export PDF (Phase 3.3)** | Có: `exportRisksPDF()` | Cùng vị trí với Export CSV (xem trên). | Như trên. |
| 4 | **Insights summary (severity bar)** | Có: `getInsightsSummary()` | **Risk Center** → tab **Risk Findings** → khối **"Risk Level Overview"** (4 ô Critical/High/Medium/Low + Resolved). | Luôn có khi có dữ liệu. |
| 5 | **Risks by cluster (Phase 2.2)** | Có: `getInsightsSummaryByCluster()` | **Risk Center** → tab **Risk Findings** → **bên dưới** "Risk Level Overview" → tiêu đề **"Risks by cluster"** + bảng theo cluster. | **Chỉ hiện khi scope = "all clusters"** (không chọn cluster cụ thể). Nếu đã chọn 1 cluster thì khối này ẩn. Cách kiểm tra: ở DataControlBar/scope chọn **"All clusters"** hoặc bỏ chọn cluster. |
| 6 | **WebSocket risks (Phase 2.3)** | Có: `getRisksWsUrl()`, subscribe trong Insights | Không có nút riêng; Risk Center **tự refetch** khi nhận message `insights_updated` từ WS. | Hoạt động nền; khi Core broadcast (sau khi có insight mới), danh sách risks cập nhật mà không cần refresh tay. |
| 7 | **PCE Trend 7 ngày (Phase 4)** | Có: `getPceTrend(7)` | **Risk Center** → tab **"Capability Exposure (PCE)"** → khối **"PCE Trend (7 Days)"** (biểu đồ line). | **Chỉ hiện khi `pceTrend.length > 0`** (có dữ liệu PCE theo ngày). Nếu cluster chưa có pod capabilities, khối này không xuất hiện. |
| 8 | **PCE Heatmap – Exposure by Namespace (Phase 4)** | Có: `getPceSummaryByNamespace()` | **Risk Center** → tab **"Capability Exposure (PCE)"** → khối **"Exposure by Namespace (heatmap)"** (bảng namespace × severity). | **Chỉ hiện khi `pceHeatmap.length > 0`**. Cùng điều kiện dữ liệu PCE như trên. |
| 9 | **Risk Rules CRUD (Phase 4)** | Có: `getRiskRules`, create/update/delete trong Settings | **Sidebar** → **Administration** → **Settings** → tab **"Risk Rules"** (cùng hàng với Users, Audit Logs). Trong tab: bảng rules, **Source** (db/files), nút **Add rule** (khi source=db), Edit/Delete từng dòng. | **Add/Edit/Delete chỉ hiện khi Source = db.** Nếu Core trả `source: "files"` thì chỉ xem danh sách, không có nút thêm/sửa/xóa. |

---

## 3. Checklist kiểm tra nhanh (theo thứ tự trên màn hình)

1. **Vào Risk Center:** Sidebar → Security → **Risk Center** → URL `/#/risks`.
2. **Tab "Risk Findings":**
   - Có dòng **Total findings**, **Scope**, **Time window** và ngay bên cạnh có **Export CSV**, **Export PDF**.
   - Dưới biểu đồ "Risk Trend (7 Days)" có **Risk Level Overview** (4 ô severity).
   - Nếu **không chọn cluster** (scope = all clusters), xuất hiện khối **"Risks by cluster"**.
   - Ở hàng filter (Severity, Workflow, Search): bên phải có **Priority** (All / P0–P4) và **Sort** (trong đó có "Risk score high to low" / "Risk score low to high").
   - Bảng findings có cột **Risk Score** (số/100 và có thể kèm P0–P4).
3. **Tab "Capability Exposure (PCE)":**
   - Nếu có dữ liệu PCE: có **PCE Trend (7 Days)** và **Exposure by Namespace (heatmap)**.
4. **Risk Rules:** Sidebar → **Settings** → tab **"Risk Rules"** → thấy Source (db/files), bảng rules, và (khi source=db) nút **Add rule** + Edit/Delete.

---

## 4. Nếu vẫn "chưa thấy thay đổi"

- **Đảm bảo build/deploy Dashboard mới nhất** (sau các commit Risk Center). Nếu chạy container: rebuild image dashboard và rollout lại.
- **Hard refresh trình duyệt:** Ctrl+Shift+R (hoặc Cmd+Shift+R) để tránh cache JS cũ.
- **Priority / Sort / Risk Score:** Nằm trên tab **Risk Findings**, cùng hàng với filter Severity/Workflow; màn hình hẹp có thể cần cuộn ngang.
- **Risks by cluster:** Chỉ có khi **không chọn cluster** (chọn "All clusters" hoặc xóa lựa chọn cluster).
- **PCE Trend & Heatmap:** Chỉ có khi **đã có dữ liệu pod capabilities**; nếu mới deploy, đợi agent sync và có pods với capabilities.
- **Risk Rules:** Vào **Settings** (không phải Risk Center) → tab **"Risk Rules"** (tab thứ 3, bên cạnh Users và Audit Logs).

---

## 5. Tham chiếu code (đã xử lý)

| Thành phần | File | Ghi chú |
|------------|------|--------|
| Risk Center page | `dashboard/pages/Insights.tsx` | Tab risks/pce/reference, fetch getRisks(withScores, priorityLevel), getInsightsSummaryByCluster, getPceTrend, getPceSummaryByNamespace, export CSV/PDF, WS refetch. |
| Risk Score / Priority / Sort | `Insights.tsx` | state priorityLevel, riskSort; dropdown Priority (dòng ~651); Sort có score_desc/score_asc; cột Risk Score (dòng ~697, 757). |
| Export CSV/PDF | `Insights.tsx` | Nút trong block activeTab === 'risks' (dòng ~446–484). |
| Risks by cluster | `Insights.tsx` | Block "Risks by cluster" (dòng ~566–607); render khi !clusterId. |
| PCE Trend & heatmap | `Insights.tsx` | PCE tab; pceTrend (dòng ~833), pceHeatmap (dòng ~855). |
| Risk Rules tab | `dashboard/pages/Settings.tsx` | Tab "Risk Rules", getRiskRules, Add/Edit/Delete khi source=db. |
| API | `dashboard/lib/api.ts` | getRisks(params với withScores, priorityLevel), exportRisksCSV/PDF, getInsightsSummaryByCluster, getRiskRules, getPceTrend, getPceSummaryByNamespace, getRisksWsUrl. |
| Route | `dashboard/App.tsx` | `/risks` → RiskCenter (Insights); `/settings` → Settings. |
| Sidebar | `dashboard/components/Layout.tsx` | Security → Risk Center path `/risks`; Administration → Settings path `/settings`. |

Kết luận: **Dashboard đã xử lý và hiển thị đủ các thay đổi**; cần vào đúng trang/tab và (với một số mục) đúng điều kiện (all clusters, có dữ liệu PCE, source=db cho CRUD) để thấy.
