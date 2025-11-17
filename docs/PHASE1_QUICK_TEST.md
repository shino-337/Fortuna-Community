# Phase 1 - Quick Test Guide

## ⚠️ Lưu ý quan trọng

localStorage sẽ **trống** cho đến khi bạn:
1. **Chọn một cluster** từ dropdown
2. **Nhập một namespace** vào input

## Test Steps (Thứ tự quan trọng!)

### Bước 1: Mở Graph View
```
http://localhost:3000/graph
```

### Bước 2: Chọn Cluster
1. Click vào dropdown "Cluster"
2. Chọn một cluster (ví dụ: "minikube")
3. **Ngay lập tức** chạy trong Console:
   ```javascript
   localStorage.getItem('ksam-graph-filter-cluster')
   ```
4. **Kỳ vọng**: Thấy cluster ID (ví dụ: "minikube")

### Bước 3: Nhập Namespace
1. Click vào input "Namespace"
2. Nhập "default"
3. **Đợi 500ms** (debounce delay)
4. Chạy trong Console:
   ```javascript
   localStorage.getItem('ksam-graph-filter-namespace')
   ```
5. **Kỳ vọng**: Thấy "default"

### Bước 4: Test Reload Persistence
1. Reload trang (F5)
2. **Kỳ vọng**: 
   - Cluster dropdown vẫn hiển thị cluster đã chọn
   - Namespace input vẫn có giá trị "default"

### Bước 5: Test Recent Namespaces
1. Clear namespace filter (click X button)
2. Nhập "kube-system" và Enter
3. Clear filter
4. Nhập "default" và Enter
5. Clear filter
6. Click vào namespace input
7. **Kỳ vọng**: Suggestions dropdown hiển thị "kube-system" và "default" với icon clock

### Bước 6: Test Loading States
1. Thay đổi cluster hoặc namespace
2. **Quan sát Filters header**
3. **Kỳ vọng**: 
   - Thấy "Loading..." text với spinner
   - Inputs bị disabled (màu xám)

### Bước 7: Test Collapsible Cards
1. **Quan sát Filters header**
2. **Kỳ vọng**: 
   - Có icon filter
   - Có thể click được
   - Có badge "Active" (màu xanh) nếu đã chọn filter
3. Click vào header "Filters"
4. **Kỳ vọng**: Advanced filters panel expand/collapse

## Debug Commands (Console)

### Kiểm tra localStorage
```javascript
// Check all localStorage keys
Object.keys(localStorage).filter(k => k.startsWith('ksam-'))

// Check specific values
localStorage.getItem('ksam-graph-filter-cluster')
localStorage.getItem('ksam-graph-filter-namespace')
JSON.parse(localStorage.getItem('ksam-recent-namespaces') || '[]')

// Clear all filters
localStorage.removeItem('ksam-graph-filter-cluster')
localStorage.removeItem('ksam-graph-filter-namespace')
localStorage.removeItem('ksam-recent-namespaces')
location.reload()
```

### Monitor localStorage changes
```javascript
// Watch for localStorage changes
const originalSetItem = localStorage.setItem;
localStorage.setItem = function(key, value) {
  if (key.startsWith('ksam-')) {
    console.log('localStorage.setItem:', key, '=', value);
  }
  originalSetItem.apply(this, arguments);
};
```

## Troubleshooting

### Nếu localStorage không save:

1. **Kiểm tra Console có lỗi không**
   - Mở DevTools → Console
   - Xem có lỗi JavaScript nào không

2. **Kiểm tra code có được load không**
   - DevTools → Sources
   - Tìm `GraphView.tsx` hoặc `GraphFilters.tsx`
   - Xem có code localStorage không

3. **Kiểm tra Network tab**
   - Xem file JS có được load không
   - Status code phải là 200 (không phải 304)

4. **Thử Incognito mode**
   - Mở cửa sổ Incognito
   - Truy cập http://localhost:3000/graph
   - Test lại

### Nếu không thấy UI changes:

1. **Hard Refresh**
   - Mac: Cmd + Shift + R
   - Windows: Ctrl + F5

2. **Clear browser cache**
   - Settings → Privacy → Clear browsing data
   - Chọn "Cached images and files"

3. **Verify pod có code mới**
   ```bash
   ./scripts/test-phase1.sh
   ```

## Expected Results

Sau khi hoàn thành tất cả test steps:

✅ localStorage có 3 keys:
- `ksam-graph-filter-cluster`: cluster ID
- `ksam-graph-filter-namespace`: namespace string
- `ksam-recent-namespaces`: JSON array of strings

✅ UI có:
- Loading indicator khi data đang load
- Collapsible Filters header
- Active badge khi có filters
- Recent namespaces trong suggestions

✅ Persistence:
- Filters được giữ sau khi reload
- Recent namespaces được giữ sau khi reload


