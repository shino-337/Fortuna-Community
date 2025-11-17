#!/bin/bash

# Quick test script for Phase 1 features
# This script helps verify that Phase 1 enhancements are working

set -e

echo "=========================================="
echo "Phase 1 Features Test Script"
echo "=========================================="
echo ""

# Check if port-forward is running
if ! lsof -Pi :3000 -sTCP:LISTEN -t >/dev/null 2>&1 ; then
  echo "⚠️  Port-forward không đang chạy trên port 3000"
  echo "   Chạy: kubectl port-forward -n ksam svc/ksam-dashboard 3000:80"
  echo ""
fi

# Check pod status
echo "1. Kiểm tra Pod status..."
POD_NAME=$(kubectl get pods -n ksam -l app=ksam-dashboard -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$POD_NAME" ]; then
  echo "   ❌ Không tìm thấy Dashboard pod"
  exit 1
fi

POD_STATUS=$(kubectl get pod -n ksam "$POD_NAME" -o jsonpath='{.status.phase}')
echo "   Pod: $POD_NAME"
echo "   Status: $POD_STATUS"

if [ "$POD_STATUS" != "Running" ]; then
  echo "   ⚠️  Pod không ở trạng thái Running"
fi

# Check image
echo ""
echo "2. Kiểm tra Docker image..."
IMAGE_ID=$(kubectl describe pod -n ksam "$POD_NAME" 2>/dev/null | grep "Image ID" | awk '{print $3}' || echo "")
echo "   Image ID: $IMAGE_ID"

# Check JS file in pod
echo ""
echo "3. Kiểm tra file JS trong pod..."
JS_FILE=$(kubectl exec -n ksam "$POD_NAME" -- sh -c "ls -1 /usr/share/nginx/html/assets/*.js 2>/dev/null | head -1" 2>/dev/null || echo "")
if [ -n "$JS_FILE" ]; then
  JS_SIZE=$(kubectl exec -n ksam "$POD_NAME" -- sh -c "ls -lh $JS_FILE 2>/dev/null" 2>/dev/null | awk '{print $5}' || echo "")
  JS_TIME=$(kubectl exec -n ksam "$POD_NAME" -- sh -c "ls -lh $JS_FILE 2>/dev/null" 2>/dev/null | awk '{print $6, $7, $8}' || echo "")
  echo "   File: $(basename $JS_FILE)"
  echo "   Size: $JS_SIZE"
  echo "   Time: $JS_TIME"
  
  # Check if localStorage code exists
  HAS_LOCALSTORAGE=$(kubectl exec -n ksam "$POD_NAME" -- sh -c "grep -q 'localStorage.getItem' $JS_FILE 2>/dev/null && echo 'yes' || echo 'no'" 2>/dev/null || echo "unknown")
  if [ "$HAS_LOCALSTORAGE" = "yes" ]; then
    echo "   ✅ Code localStorage có trong file"
  else
    echo "   ❌ Code localStorage không tìm thấy"
  fi
else
  echo "   ❌ Không tìm thấy file JS"
fi

echo ""
echo "=========================================="
echo "Test Checklist"
echo "=========================================="
echo ""
echo "Mở browser và test các tính năng sau:"
echo ""
echo "✅ localStorage Persistence:"
echo "   1. Chọn cluster → Reload → Filter còn giữ?"
echo "   2. Nhập namespace → Reload → Filter còn giữ?"
echo "   3. Check localStorage trong DevTools Console"
echo ""
echo "✅ Loading States:"
echo "   1. Thay đổi filter → Có 'Loading...' không?"
echo "   2. Inputs có bị disabled khi loading không?"
echo ""
echo "✅ Collapsible Cards:"
echo "   1. Click 'Filters' header → Expand/collapse hoạt động?"
echo "   2. Chọn filter → Badge 'Active' xuất hiện?"
echo ""
echo "=========================================="
echo "Browser Test Commands (DevTools Console):"
echo "=========================================="
echo ""
echo "// Check localStorage"
echo "localStorage.getItem('ksam-graph-filter-cluster')"
echo "localStorage.getItem('ksam-graph-filter-namespace')"
echo "JSON.parse(localStorage.getItem('ksam-recent-namespaces') || '[]')"
echo ""
echo "// Clear all filters"
echo "localStorage.removeItem('ksam-graph-filter-cluster')"
echo "localStorage.removeItem('ksam-graph-filter-namespace')"
echo "localStorage.removeItem('ksam-recent-namespaces')"
echo "location.reload()"
echo ""
echo "=========================================="
echo "Nếu không thấy thay đổi:"
echo "=========================================="
echo "1. Hard Refresh: Cmd+Shift+R (Mac) hoặc Ctrl+F5 (Windows)"
echo "2. Clear browser cache hoàn toàn"
echo "3. Kiểm tra Network tab → Xem file JS có timestamp mới"
echo "4. Chạy lại: ./scripts/rebuild-dashboard-force.sh"
echo ""

