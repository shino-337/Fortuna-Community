# 🎉 Release Notes v1.0.2

**Release Date:** November 19, 2024  
**Status:** ✅ Stable - Production Ready

---

## 🐛 Critical Bug Fix: Namespace Filter

### Issue
Namespace filter trên Graph View không hoạt động đúng:
- API được gọi đúng với params (`?namespace=default`)
- Nhưng UI không cập nhật graph
- Sau mỗi lần filter, có một API call thứ 2 không có params → override kết quả

### Root Cause
**Debounce timer race condition** trong `GraphFilters.tsx`:

1. User nhập "kube" → Debounce timer starts (500ms)
2. User click suggestion "kube-system" → `selectNamespace()` called
3. Timer KHÔNG được clear → Sau 500ms, timer fire với giá trị cũ → Gọi API không đúng

### Solution
✅ **Clear debounce timer** trong:
- `selectNamespace()` - Khi chọn từ suggestions
- `handleClearFilters()` - Khi clear filters
- Component unmount - Cleanup

✅ **Fix React Query cache invalidation**:
- Changed `queryKey: ['graph', params]` 
- To: `queryKey: ['graph', params?.cluster, params?.namespace]`
- Lý do: Object reference không thay đổi → Query không refetch

### Files Changed
```
dashboard/src/components/Graph/GraphFilters.tsx
- Added timer cleanup in selectNamespace()
- Added timer cleanup in handleClearFilters()

dashboard/src/hooks/useGraph.ts
- Explicit queryKey with cluster/namespace
- Added placeholderData: undefined
```

---

## ✅ Testing Results

### Test Case 1: Filter by Namespace
**Steps:**
1. Open Graph View
2. Type "default" → Select from suggestions
3. Graph updates immediately

**Expected:** 
- API call: `GET /api/v1/graph?namespace=default`
- Graph shows ~21 nodes (only ServiceAccounts in default namespace)

**Result:** ✅ **PASS**

---

### Test Case 2: Clear Filters
**Steps:**
1. With filter applied (e.g., namespace=default)
2. Click "Clear Filters" button
3. Graph reloads all data

**Expected:**
- API call: `GET /api/v1/graph` (no params)
- Graph shows 86 nodes (all ServiceAccounts)

**Result:** ✅ **PASS**

---

### Test Case 3: Switch Between Namespaces
**Steps:**
1. Filter by "default"
2. Change to "kube-system"
3. Change to "kube-public"
4. Clear filter

**Expected:**
- Each filter change triggers ONE API call (not multiple)
- Graph updates immediately without lag

**Result:** ✅ **PASS**

---

## 🎯 Key Features (v1.0.0 → v1.0.2)

### Dashboard
- ✅ Dark mode across all pages (Dashboard, Graph, ServiceAccounts, AuditLogs, Login)
- ✅ RBAC guards for admin/user roles
- ✅ Modern UI with Tailwind CSS
- ✅ Favicon and branding: "K8s Workload Management Platform"

### Graph View
- ✅ Interactive Cytoscape graph visualization
- ✅ Namespace filter (now working!)
- ✅ Cluster filter
- ✅ Node type filters (ServiceAccount, Namespace, Cluster)
- ✅ Connection type filters
- ✅ Layout options (cose, fcose, dagre, etc.)
- ✅ Node details panel
- ✅ Search functionality
- ✅ Responsive sidebar with collapse

### ServiceAccounts
- ✅ List all ServiceAccounts across clusters
- ✅ Filter by cluster/namespace
- ✅ View permissions (Roles, ClusterRoles)
- ✅ Delete ServiceAccounts (RBAC protected)
- ✅ Pagination

### Audit Logs
- ✅ Real-time log syncing (10s refresh)
- ✅ Filter by resource/action
- ✅ Default filter: ServiceAccount logs
- ✅ User/timestamp tracking
- ✅ Immediate filter application (no "Apply" button needed)

### Backend
- ✅ Agent: Delta sync + Full sync (every 10th sync)
- ✅ Core: Batch processing + TTL for audit logs
- ✅ Watcher: Real-time RBAC event sync
- ✅ PostgreSQL: Optimized schema with indexes
- ✅ gRPC communication: Agent → Core

---

## 📊 Performance Metrics

| Metric | Value |
|--------|-------|
| **Bundle Size** | 1,011.29 KB (gzip: 308.32 KB) |
| **Graph API Response** | ~20-25ms (with filter) |
| **Graph Render Time** | <500ms (86 nodes) |
| **Audit Log Refresh** | 10s auto-refresh |
| **Console Logs (prod)** | 0 (all removed) |

---

## 🚀 Deployment

### Current Deployment
```bash
# Image
ksam-dashboard:v1.0.2

# Pods
kubectl get pods -n ksam -l app=ksam-dashboard
# Expected: 2 replicas, all Running

# Service
minikube service ksam-dashboard -n ksam --url
```

### Rollback (if needed)
```bash
# Rollback to v1.0.1
kubectl patch deployment ksam-dashboard -n ksam \
  --type='json' \
  -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/image", "value":"ksam-dashboard:v1.0.1"}]'

kubectl delete pod -n ksam -l app=ksam-dashboard
```

---

## 🐛 Known Issues

### None identified in v1.0.2

All critical issues from v1.0.0 and v1.0.1 have been resolved:
- ✅ Namespace filter working
- ✅ Dark mode synchronized across all pages
- ✅ Graph container height issue fixed
- ✅ Cytoscape race condition fixed
- ✅ Symbol.toStringTag error fixed
- ✅ Debug logs removed

---

## 📝 Breaking Changes

**None** - Fully backward compatible with v1.0.0 and v1.0.1

---

## 🔜 Next Steps (v2.0.0)

### RBAC Expansion
Currently in planning phase. The system will expand from managing only ServiceAccounts to the full Kubernetes RBAC pipeline:

1. **Resources to Manage:**
   - ✅ ServiceAccount (current)
   - 🔄 Role / ClusterRole
   - 🔄 RoleBinding / ClusterRoleBinding
   - 🔄 Pod & Workload → ServiceAccount mapping

2. **Features to Add:**
   - RBAC Audit Log lifecycle
   - RBAC Graph (relationship model)
   - Optimized sync/reconciliation (no duplicate data)
   - Permission management
   - RBAC Drift Detection (detect unusual changes)

3. **UI Updates:**
   - Add RBAC to Dashboard overview
   - Add RBAC to Graph View (new node types)
   - Add RBAC to Audit Logs (filter by Role/Binding)
   - RBAC permissions editor

**Target:** Q1 2025

---

## 🙏 Credits

**Developed by:** KSAM Team  
**Framework:** React + Vite + TypeScript  
**Graph Library:** Cytoscape.js  
**Backend:** Go + Gin + GORM  
**Infrastructure:** Kubernetes + PostgreSQL + gRPC

---

## 📖 Documentation

- [TESTING_v1.0.1.md](./TESTING_v1.0.1.md) - Testing guide
- [DEBUG_FILTER_ISSUE.md](./DEBUG_FILTER_ISSUE.md) - Debug process documentation
- [README.md](./README.md) - Project overview
- [docs/k8s-event-sync-architecture.md](./docs/k8s-event-sync-architecture.md) - Architecture design

---

**Git Tag:** `v1.0.2`  
**Commit:** `e05d185`  
**Previous Release:** `v1.0.1`

---

## 🎬 How to Test

1. **Access Dashboard:**
```bash
minikube service ksam-dashboard -n ksam --url
# Login: admin / admin123
```

2. **Test Namespace Filter:**
   - Go to Graph View
   - Type "default" in Namespace input
   - Select from suggestions
   - ✅ Graph should show ~21 nodes immediately
   - ✅ Network tab shows ONE API call with `?namespace=default`

3. **Test Clear Filter:**
   - Click "Clear Filters" button
   - ✅ Graph should show all 86 nodes
   - ✅ Namespace input is cleared

4. **Test Switch Namespaces:**
   - Filter by "default"
   - Change to "kube-system"
   - Change to "kube-public"
   - ✅ Each change shows different nodes
   - ✅ No duplicate API calls

5. **Verify Console:**
   - Open DevTools (F12) → Console tab
   - ✅ No debug logs ([DEBUG ...])
   - ✅ No errors

---

**Status:** ✅ Ready for production use

