# Hướng Dẫn Test Phase 1 Enhancements

## Các Tính Năng Cần Test

### 1. localStorage Persistence

#### Test Case 1: Lưu và Load Filters
1. Mở Dashboard Graph View
2. Chọn một cluster từ dropdown
3. Nhập một namespace (ví dụ: `default`)
4. **Reload trang** (F5 hoặc Cmd+R)
5. **Kỳ vọng**: Cluster và namespace đã chọn vẫn được giữ lại

#### Test Case 2: Recent Namespaces
1. Nhập namespace `default` và Enter
2. Clear filter
3. Nhập namespace `kube-system` và Enter
4. Clear filter
5. Click vào namespace input
6. **Kỳ vọng**: Thấy suggestions với `default` và `kube-system` (có icon clock)

#### Test Case 3: Clear Filters
1. Chọn cluster và namespace
2. Click "Clear Filters"
3. **Reload trang**
4. **Kỳ vọng**: Filters đã được clear, không còn trong localStorage

### 2. Loading States

#### Test Case 1: Loading Indicator trong Filters
1. Mở Dashboard Graph View
2. Thay đổi cluster hoặc namespace
3. **Kỳ vọng**: 
   - Thấy "Loading..." text với spinner trong Filters header
   - Inputs bị disabled khi đang loading

#### Test Case 2: Loading khi Load Clusters
1. Mở Dashboard Graph View
2. **Kỳ vọng**: 
   - Thấy spinner nhỏ bên cạnh label "Cluster" khi clusters đang load
   - Select dropdown bị disabled khi đang loading

### 3. Collapsible Filter Cards

#### Test Case 1: Collapse/Expand
1. Mở Dashboard Graph View
2. Click vào header "Filters" (có icon filter)
3. **Kỳ vọng**: 
   - Advanced filters panel collapse/expand
   - Icon mũi tên xoay 180 độ

#### Test Case 2: Active Badge
1. Chọn cluster hoặc namespace
2. **Kỳ vọng**: 
   - Thấy badge "Active" màu xanh bên cạnh "Filters"
3. Clear filters
4. **Kỳ vọng**: Badge "Active" biến mất

#### Test Case 3: Advanced Filters Panel
1. Click để expand Advanced Filters
2. **Kỳ vọng**: 
   - Panel có background màu xám nhạt (bg-gray-50)
   - Hiển thị 3 sections: Node Types, Connection Types, Layout Options
   - Có message "Advanced filters will be functional in Phase 2"

## Cách Kiểm Tra localStorage

### Trong Browser DevTools:

1. Mở DevTools (F12)
2. Vào tab **Application** (Chrome) hoặc **Storage** (Firefox)
3. Mở **Local Storage** → `http://localhost:3000` (hoặc URL của bạn)
4. Kiểm tra các keys:
   - `ksam-graph-filter-cluster`: Chứa cluster ID đã chọn
   - `ksam-graph-filter-namespace`: Chứa namespace đã nhập
   - `ksam-recent-namespaces`: Chứa array JSON của recent namespaces

### Test localStorage bằng Console:

```javascript
// Kiểm tra cluster filter
localStorage.getItem('ksam-graph-filter-cluster')

// Kiểm tra namespace filter
localStorage.getItem('ksam-graph-filter-namespace')

// Kiểm tra recent namespaces
JSON.parse(localStorage.getItem('ksam-recent-namespaces') || '[]')

// Clear tất cả filters
localStorage.removeItem('ksam-graph-filter-cluster')
localStorage.removeItem('ksam-graph-filter-namespace')
localStorage.removeItem('ksam-recent-namespaces')
// Sau đó reload trang
```

## Troubleshooting

### Nếu không thấy thay đổi:

1. **Clear Browser Cache:**
   - Hard Refresh: `Ctrl+F5` (Windows) hoặc `Cmd+Shift+R` (Mac)
   - Hoặc clear cache trong DevTools: Right-click Refresh → "Empty Cache and Hard Reload"

2. **Kiểm tra Pod đang dùng image mới:**
   ```bash
   kubectl describe pod -n ksam -l app=ksam-dashboard | grep Image
   ```
   Image ID phải là `b4a33052f15d` hoặc mới hơn

3. **Kiểm tra file JS trong pod:**
   ```bash
   kubectl exec -n ksam $(kubectl get pods -n ksam -l app=ksam-dashboard -o jsonpath='{.items[0].metadata.name}') -- ls -lh /usr/share/nginx/html/assets/*.js
   ```
   File phải có timestamp mới (Nov 12 04:21 hoặc mới hơn)

4. **Force rebuild lại:**
   ```bash
   ./scripts/rebuild-dashboard-force.sh
   ```

## Checklist Test

- [ ] Filters được lưu vào localStorage
- [ ] Filters được load lại khi reload trang
- [ ] Recent namespaces được lưu và hiển thị trong suggestions
- [ ] Loading indicator hiển thị khi data đang load
- [ ] Inputs bị disabled khi đang loading
- [ ] Collapsible header hoạt động (expand/collapse)
- [ ] Active badge hiển thị khi có filters
- [ ] Advanced filters panel có styling riêng
- [ ] Clear filters button hoạt động và clear localStorage

---

**Version**: 1.0  
**Date**: 2025-11-12

