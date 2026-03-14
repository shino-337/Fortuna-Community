# FORTUNA UI – Khoảng trống giữa tài liệu và hiện trạng

Mục đích: Đối chiếu các tài liệu (Component Library, Design System, Layout Patterns, Data Viz, Security UX) với code dashboard hiện tại, giải thích **tại sao sau khi build người dùng chưa thấy thay đổi UI/UX rõ ràng**, và liệt kê thay đổi cần làm để đồng bộ.

---

## 1. Tóm tắt

- **Tài liệu** mô tả: layout chuẩn (PageLayout, Section, Card variants, Badge), typography/spacing tokens, max-width 1600px, KPI/Detail/Table patterns.
- **Thực tế**: Chỉ một phần trang dùng design-system (Dashboard, Risk Center, PodDetail, ClusterDetail, RiskDetail); phần lớn trang vẫn dùng `components/PageLayout` (không có max-width, không spacing chuẩn). Card không có variant, Badge là inline style rải rác, Section và tokens hầu như không được dùng.
- **Kết quả**: Giao diện nhìn gần như không đổi sau build vì đa số trang và component chưa áp dụng design system.

---

## 2. Đối chiếu theo tài liệu

### 2.1 FORTUNA_COMPONENT_LIBRARY & FORTUNA_UI_DESIGN_SYSTEM

| Yêu cầu doc | Hiện trạng | Ghi chú |
|-------------|------------|--------|
| **PageLayout**: max-w-[1600px], mx-auto, px-6, space-y-8 | Chỉ `design-system/layouts/PageLayout.tsx` có wrapper max-w-[1600px] + SPACING. Nhiều trang dùng `components/PageLayout` (không wrapper) → **không** max-width. | Dashboard, Insights, PodDetail, ClusterDetail, RiskDetail dùng design-system; Sbom, Settings, Resources, Rules, Metrics, Clusters, v.v. dùng components/PageLayout. |
| **PageHeader**: title text-2xl, description text-sm, actions bên phải | Đã có trong `components/PageLayout` (BasePageLayout). Chưa dùng typography tokens. | Đúng pattern nhưng chưa token hóa. |
| **Section**: nhóm nội dung, space-y-6 | `design-system/layouts/Section.tsx` tồn tại (re-export PageSection). **Hầu như không trang nào import Section từ design-system.** | Spacing giữa section trong trang là ad-hoc (space-y-4, space-y-6, không thống nhất space-y-8). |
| **Card**: variant primary (p-6), secondary (p-4), panel (p-3) | `components/ui/Card.tsx` **không có prop variant**. Luôn p-6 cho content, header px-6 py-4. | Doc yêu cầu 3 variant; code chỉ có một kiểu. |
| **Badge**: severity (critical/high/medium/low/info), text-[11px] font-semibold | **Không có component Badge.** Mỗi trang dùng `<span className={getSeverityBadgeClass(...)}>` với text-[10px], text-xs, text-[11px] lẫn lộn. | Thiếu component chuẩn và token. |
| **Typography tokens**: pageTitle, cardTitle, body, helper, badge, microLabel | File `design-system/tokens/typography.ts` có đủ. **Pages không import hay dùng.** | Token tồn tại nhưng chưa áp dụng. |
| **Spacing tokens**: sectionY (space-y-8), pageX (px-6), cardPrimary/Secondary/Panel | `design-system/tokens/spacing.ts` có. Chỉ design-system PageLayout dùng SPACING.pageX, sectionY. | Đa số trang không dùng. |

### 2.2 FORTUNA_UI_LAYOUT_PATTERNS

| Pattern | Doc | Hiện trạng |
|---------|-----|------------|
| Dashboard layout | Header → KPI → Trend → Panels/Tables | Dashboard.tsx có cấu trúc tương tự nhưng không dùng Section, spacing không chuẩn space-y-8. |
| Detail page | Header → Summary → Tabs → Content → Related | PodDetail/ClusterDetail dùng Tabs từ design-system; không dùng Section cho từng khối. |
| Table page | Header → Filters → Table | Resources, Clusters, Rules dùng components/PageLayout, không design-system. |

### 2.3 FORTUNA_DATA_VISUALIZATION_GUIDELINES & FORTUNA_SECURITY_PRODUCT_UX_GUIDE

- Charts (Trend, Histogram, severity) đã có và dùng severity color; có thể tinh chỉnh thêm theo doc.
- Vấn đề chính: **layout và component** chưa đồng bộ nên tổng thể UI chưa phản ánh đầy đủ Data Viz và Security UX.

---

## 3. Trang nào dùng design-system PageLayout?

**Đang dùng** `design-system/layouts/PageLayout` (có max-w-[1600px], spacing):

- Dashboard, Insights (Risk Center), PodDetail, ClusterDetail, RiskDetail

**Chưa dùng** (vẫn `components/PageLayout` hoặc không có wrapper):

- Sbom, Settings, IdentityDetail, NodeDetail, Resources, AttackPaths, Rules, Metrics, Notifications, Certificates, Reports, Clusters, RuleDetail, ErrorLogs, Audit, Users, Capabilities (một phần)

Hệ quả: Trên màn rộng, chỉ vài trang bị giới hạn width; phần còn lại full width → cảm giác “không có gì thay đổi” nếu user mở Clusters, Settings, Resources, v.v.

---

## 4. Đề xuất thay đổi (để thấy rõ UI/UX sau build)

1. **Thống nhất PageLayout**  
   Tất cả trang dùng `design-system/layouts/PageLayout` (hoặc wrapper tương đương) để có max-w-[1600px], px-6, space-y-8.

2. **Card variants**  
   Thêm prop `variant?: 'primary' | 'secondary' | 'panel'` cho Card, map padding đúng doc (p-6 / p-4 / p-3). Áp dụng variant ở Dashboard, Risk Center, PodDetail.

3. **Component Badge**  
   Tạo `design-system/components/Badge.tsx` (hoặc `components/ui/Badge.tsx`) với variant severity, dùng typography token badge (text-[11px] font-semibold). Thay thế dần các span severity hiện tại.

4. **Section + spacing**  
   Trang Dashboard và Risk Center dùng `Section` từ design-system cho từng khối (KPI, Trend, Tables), với space-y-8 giữa section (đúng spacing tokens).

5. **Typography tokens**  
   PageLayout/PageHeader và Card title dùng TYPOGRAPHY.pageTitle, cardTitle; Badge dùng TYPOGRAPHY.badge. Có thể áp dụng từ từ cho từng component.

Sau khi làm (1)–(4), build lại sẽ thấy:

- Mọi trang có container rộng tối đa 1600px, lề ngang và khoảng cách dọc thống nhất.
- Card có phân cấp rõ (primary/secondary/panel).
- Badge severity thống nhất kiểu chữ và màu.
- Dashboard và Risk Center có cấu trúc section rõ ràng, dễ quét.

---

## 5. Đã thực hiện (sau khi rà soát)

| Thay đổi | Trạng thái |
|----------|------------|
| Tất cả trang dùng `design-system/layouts/PageLayout` | ✅ Đã chuyển Sbom, Settings, IdentityDetail, Resources, NodeDetail, AttackPaths, Notifications, Certificates, Reports, Clusters, Rules, Metrics, RuleDetail, ErrorLogs, Audit sang design-system PageLayout. |
| PageLayout spacing | ✅ `components/PageLayout.tsx` dùng `space-y-8` thay cho `space-y-6`. |
| Card variants | ✅ `components/ui/Card.tsx` thêm `variant?: 'primary' \| 'secondary' \| 'panel'` (p-6 / p-4 / p-3). Dashboard dùng variant primary/secondary. |
| Badge component | ✅ `design-system/components/Badge.tsx` dùng typography token `text-[11px] font-semibold`, tích hợp `getSeverityBadgeClass`. Dashboard dùng Badge cho nhãn Critical. |
| Section trên Dashboard | ✅ Hai khối Infrastructure và Security Risks dùng `design-system/layouts/Section`. |

Sau khi build lại, cần thấy: container tối đa 1600px + lề ngang trên mọi trang; khoảng cách dọc giữa header và nội dung lớn hơn (space-y-8); Dashboard có section có tiêu đề chuẩn (Section); thẻ risk dùng Badge chuẩn.

---

## 6. Tài liệu tham chiếu

- `FORTUNA_COMPONENT_LIBRARY.md`
- `FORTUNA_UI_DESIGN_SYSTEM_03132026.md`
- `FORTUNA_UI_LAYOUT_PATTERNS.md`
- `FORTUNA_DATA_VISUALIZATION_GUIDELINES.md`
- `FORTUNA_SECURITY_PRODUCT_UX_GUIDE.md`
