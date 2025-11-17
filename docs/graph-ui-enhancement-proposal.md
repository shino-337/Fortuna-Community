# 🧭 Dashboard Enhancement Proposal
### (ServiceAccount / RBAC Graph Visualization)

## 1️⃣ Tổng quan hiện trạng
Hiện tại dashboard có hai phần chính:
- **GraphFilters.tsx**: quản lý các bộ lọc (cluster, namespace, loại node/edge, layout, autofit)
- **GraphVisualization.tsx**: hiển thị Cytoscape graph, xử lý zoom, fit, hover, tooltip, highlight, v.v.

Mặc dù chức năng đầy đủ và chi tiết, UI/UX còn gặp một số vấn đề:

| Nhóm | Hiện trạng | Ảnh hưởng |
|------|-------------|------------|
| **Giao diện tổng thể** | UI dày đặc, thiếu phân tầng thị giác | Người dùng mới khó định hướng |
| **Hiển thị Graph** | Cytoscape layout chưa tối ưu cho đồ thị lớn (1000+ node) | Giảm hiệu suất và gây rối thị giác |
| **Legend / Info Panel** | Cố định, chiếm diện tích và lặp nội dung | Giảm vùng xem graph |
| **Tooltip** | Cố định vị trí theo chuột, dễ che mất node | Gây khó đọc khi di chuyển nhanh |
| **Filter Panel** | Rất chi tiết nhưng nhiều thao tác click | Làm chậm trải nghiệm |
| **Zoom / Fit / Refresh Tool** | Rời rạc, không có trạng thái hiển thị | Thiếu trực quan về view state |

---

## 2️⃣ Đề xuất kiến trúc UI mới
### 🎨 Bố cục tổng thể
```
┌───────────────────────────┬────────────────────────────┬───────────────────────────┐
│ Filters / Legend Sidebar  │        Graph Canvas        │ Node / Detail Inspector   │
└───────────────────────────┴────────────────────────────┴───────────────────────────┘
```
- **Panel trái** (Filters + Legend): có thể thu gọn (collapse).
- **Panel phải** (Node Details): hiển thị động khi chọn node.
- **Giữa**: vùng render Cytoscape chiếm tối đa diện tích.

---

## 3️⃣ Cải tiến trực quan graph

### 🌐 Visualization Core (GraphVisualization.tsx)
| Mục tiêu | Giải pháp đề xuất |
|-----------|-------------------|
| **Hiệu suất** | Dùng layout **“fcose”** hoặc **“dagre”** cho đồ thị lớn (hiệu quả hơn `cose`). |
| **Màu sắc** | Chuyển sang theme màu nhẹ (Material3 palette), với contrast tốt hơn. |
| **Node shape** | Giữ icon + màu, nhưng dùng **badge nhỏ** thay vì chỉ màu nền để dễ phân loại. |
| **Edge clarity** | Áp dụng **curved edges** với màu gradient theo hướng (source→target). |
| **Zoom interaction** | Bật **“pan/zoom inertia”** để cảm giác mượt hơn. |
| **Tooltips** | Chuyển tooltip sang dạng **floating side panel** (bên phải) có thể mở rộng. |
| **Search node** | Thêm thanh tìm kiếm “Search Node/ServiceAccount” ở góc phải toolbar. |
| **Mini-map** | Thêm **mini overview map** ở góc dưới phải để di chuyển nhanh trên graph lớn. |

---

## 4️⃣ Cải tiến Filter & Layout

### ⚙️ GraphFilters.tsx
| Mục tiêu | Giải pháp đề xuất |
|-----------|-------------------|
| **Giảm độ phức tạp** | Gom các loại filter thành nhóm “Quick Filters” (Cluster, Namespace, NodeType). |
| **UX hiện đại** | Dùng **segmented toggle buttons** thay vì checkbox cho Node/Edge type. |
| **Namespace search** | Gợi ý realtime từ API thay vì danh sách cứng. |
| **Layout switching** | Dùng **dropdown + preview thumbnail** cho từng layout. |
| **Presets** | Cho phép lưu cấu hình filter hiện tại (Preset: “Namespace Focus”, “RBAC Overview”). |

---

## 5️⃣ Trải nghiệm người dùng (UX)
### 🚀 Tăng tốc thao tác và hiển thị
- Khi hover node → highlight **path toàn phần** đến resource (không chỉ kề cận).
- Khi chọn node → focus + tự động mở panel phải hiển thị chi tiết SA/Role/Namespace.
- Khi chọn nhiều node → cho phép “Compare Privileges”.
- Giữ **animation easing nhẹ** khi chuyển layout hoặc filter.

---

## 6️⃣ Đề xuất công nghệ hỗ trợ

| Hạng mục | Công nghệ đề xuất | Ghi chú |
|-----------|-------------------|---------|
| **Graph Engine** | `Cytoscape.js` (hiện có) + `cytoscape-dagre` | Cho layout có hướng, dễ đọc flow RBAC |
| **UI Framework** | React + TailwindCSS + ShadCN/UI | Đơn giản, đẹp, responsive |
| **State Management** | Zustand hoặc Recoil | Dễ dàng chia sẻ state giữa filters/graph/panel |
| **Search** | Fuse.js (client fuzzy search) | Tìm nhanh node theo tên |
| **Mini-map** | `cytoscape-minimap` plugin | Điều hướng đồ thị lớn |
| **Performance** | Virtualization (render subset), hoặc `cytoscape-graphml` lazy load | Cho đồ thị > 2000 node |

---

## 7️⃣ Trải nghiệm chuyên nghiệp hơn
- **Dark/Light Mode Auto Switch**
- **Persistent layout per cluster** (ghi nhớ vị trí node giữa các session)
- **Graph Screenshot Export** (PNG, SVG)
- **Drill-down Navigation**
  - Click cluster → show namespaces
  - Click namespace → show service accounts & roles
  - Click SA → show linked roles/secrets

---

## 8️⃣ Đề xuất giao diện mới (concept)
Một số cải tiến UI có thể áp dụng:
- **Toolbar**: gom tất cả thao tác zoom/reset/search vào 1 cụm floating top-right.
- **Legend + Filters**: chuyển về sidebar có tab (Filters / Legend).
- **Node Detail Panel**: hiển thị metadata, roles, bindings, và các path ngắn nhất đến ClusterRole.

---

## 9️⃣ Đánh giá tác động
| Mục tiêu | Kết quả kỳ vọng |
|-----------|-----------------|
| Thời gian load graph | Giảm 40–60% với layout tối ưu |
| Khả năng đọc graph | Tăng đáng kể với layout có hướng & edge gradient |
| Trải nghiệm người dùng | Giao diện thống nhất, dễ điều hướng |
| Khả năng mở rộng | Có thể thêm view “Risk Heatmap” và “Attack Path Simulation” |

---

📄 **File:** `graph-ui-enhancement-proposal.md`
🗓️ **Ngày:** 2025-11-13
✍️ **Tác giả:** DevSecOps UI/UX Design Team