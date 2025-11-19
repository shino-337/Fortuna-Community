# Testing Guide v1.0.1

## 🧪 HƯỚNG DẪN KIỂM TRA GRAPH VIEW FILTERS

### 1. Truy cập Dashboard
```bash
minikube service ksam-dashboard -n ksam --url
# Đăng nhập: admin/admin123
```

### 2. Cách sử dụng Filters

#### **A. API Filters (Query từ Backend)**
Ở phần trên của sidebar "Filters", có 2 dropdown:

**Cluster Filter:**
- Chọn cluster để chỉ hiển thị resources từ cluster đó
- Mặc định: hiển thị tất cả clusters

**Namespace Input:**
- Nhập tên namespace (ví dụ: `default`, `kube-system`)
- Suggestions sẽ xuất hiện khi bạn focus vào input
- Chọn từ suggestions hoặc nhập trực tiếp
- Nhấn Enter hoặc click ra ngoài để apply
- **Kết quả**: Graph CHỈ hiển thị ServiceAccounts trong namespace đó

#### **B. Client-Side Filters (Ẩn/hiện nodes trên graph)**
Trong "Advanced Filters" → "Node Types":

- ☑️ **ServiceAccount** - Toggle để ẩn/hiện tất cả ServiceAccount nodes
- ☑️ **Namespace** - Toggle để ẩn/hiện tất cả Namespace nodes
- ☑️ **Cluster** - Toggle để ẩn/hiện tất cả Cluster nodes
- ☑️ **Role** / **ClusterRole** - (Sẽ có trong v2.0.0)

### 3. Test Scenarios

#### **Test Case 1: Filter theo Namespace**
1. Vào Graph View
2. Trong sidebar, nhập `default` vào Namespace input
3. Nhấn Enter
4. **Expected Result:**
   - Graph hiển thị ~10-20 nodes
   - Chỉ ServiceAccounts trong namespace `default`
   - 1 Namespace node (`default`)
   - 1 Cluster node (`minikube`)

#### **Test Case 2: Node Type Filter**
1. Trong "Advanced Filters", click để mở
2. Uncheck ☐ **Namespace**
3. **Expected Result:**
   - Namespace nodes biến mất khỏi graph
   - ServiceAccount và Cluster nodes vẫn hiển thị
   - Edges từ SA → Namespace bị ẩn

4. Uncheck ☐ **ServiceAccount**
5. **Expected Result:**
   - Tất cả ServiceAccount nodes biến mất
   - Chỉ còn Namespace và Cluster nodes

#### **Test Case 3: Kết hợp cả 2 filters**
1. Namespace input: `kube-system`
2. Node Types: Check tất cả
3. **Expected Result:**
   - Graph hiển thị ServiceAccounts trong `kube-system`
   - Có Namespace và Cluster nodes

4. Uncheck ☐ **Cluster**
5. **Expected Result:**
   - Cluster node biến mất
   - ServiceAccounts và Namespace vẫn hiển thị
   - Edge từ Namespace → Cluster bị ẩn

#### **Test Case 4: Clear Filters**
1. Click nút "Clear" bên cạnh filters
2. **Expected Result:**
   - Namespace input cleared
   - Graph reload và hiển thị TẤT CẢ ServiceAccounts (86 nodes)

### 4. Browser Console Check

**Mở Console (F12):**
- ✅ Không có debug logs (🔍📊✅⚠️)
- ✅ Không có error `Symbol.toStringTag`
- ✅ Console sạch sẽ

**Nếu vẫn thấy logs:**
1. Hard refresh: `Ctrl+Shift+R` (Windows/Linux) hoặc `Cmd+Shift+R` (Mac)
2. Clear cache: Settings → Clear browsing data → Cached images and files
3. Reload lại trang

### 5. Common Issues & Solutions

#### **Vấn đề: Namespace filter không hoạt động**
**Nguyên nhân:** Browser cache
**Giải pháp:**
```bash
# Force rebuild và restart pod
kubectl delete pod -n ksam -l app=ksam-dashboard
# Đợi pod mới khởi động
kubectl get pods -n ksam -w
```

#### **Vấn đề: Vẫn thấy debug logs trong console**
**Nguyên nhân:** Browser cached JavaScript
**Giải pháp:**
1. Hard refresh: `Ctrl+Shift+R`
2. Hoặc: DevTools → Network tab → Check "Disable cache" → Reload

#### **Vấn đề: Node Type filter không ẩn nodes**
**Nguyên nhân:** Cần click vào checkbox, không phải label
**Giải pháp:** 
- Click trực tiếp vào ☐ checkbox
- Hoặc click vào label text

### 6. Verify Image Version

Kiểm tra version đang chạy:
```bash
kubectl get pod -n ksam -l app=ksam-dashboard \
  -o jsonpath='{.items[0].spec.containers[0].image}'
# Expected: ksam-dashboard:v1.0.1
```

Kiểm tra JS bundle:
```bash
kubectl exec -n ksam $(kubectl get pod -n ksam -l app=ksam-dashboard \
  -o jsonpath='{.items[0].metadata.name}') \
  -- cat /usr/share/nginx/html/index.html | grep -o 'assets/index-[^"]*'
# Expected: assets/index-D8EY9jzz.js (v1.0.1)
```

### 7. Expected Behavior Summary

| Filter Type | Location | Effect |
|------------|----------|--------|
| **Namespace (input)** | Sidebar top | Query backend → Show only SAs in that namespace |
| **Cluster (dropdown)** | Sidebar top | Query backend → Show only resources in that cluster |
| **Node Types (checkboxes)** | Advanced Filters | Client-side → Hide/show specific node types |
| **Connection Types** | Advanced Filters | Client-side → Hide/show specific edge types |

**Key Point:** 
- **API Filters** = What data to fetch
- **Client-side Filters** = What to display from fetched data

---

## 📊 Performance Metrics

**v1.0.1 Improvements:**
- Bundle size: 1011KB (↓5KB từ v1.0.0)
- Console logs: 0 (production)
- Graph filters: Working ✅
- No Symbol errors ✅

---

## 🐛 Bug Reports

Nếu gặp vấn đề:
1. Check browser console for errors
2. Verify image version (xem section 6)
3. Test API directly (xem TESTING_API.md)
4. Hard refresh browser cache
5. Report với screenshots + console logs

---

**Version:** v1.0.1  
**Date:** 2024-11-19  
**Status:** ✅ Stable

