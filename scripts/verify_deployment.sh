#!/bin/bash
# Verify Deployment and CORS

set -e

NAMESPACE="ksam"
DEPLOYMENT="ksam-core"

echo "=========================================="
echo "Deployment Verification"
echo "=========================================="
echo ""

# 1. Check Deployment
echo "[1] Deployment Status:"
echo "-----------------------------------"
kubectl get deployment -n ${NAMESPACE} ${DEPLOYMENT} 2>&1 || echo "Deployment not found"
echo ""

# 2. Check Pods
echo "[2] Pod Status:"
echo "-----------------------------------"
kubectl get pods -n ${NAMESPACE} -l app=${DEPLOYMENT} -o wide
echo ""

# 3. Check Image
echo "[3] Image Configuration:"
echo "-----------------------------------"
IMAGE=$(kubectl get deployment -n ${NAMESPACE} ${DEPLOYMENT} -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null || echo "N/A")
PULL_POLICY=$(kubectl get deployment -n ${NAMESPACE} ${DEPLOYMENT} -o jsonpath='{.spec.template.spec.containers[0].imagePullPolicy}' 2>/dev/null || echo "N/A")
echo "Image: ${IMAGE}"
echo "Pull Policy: ${PULL_POLICY}"
echo ""

# 4. Check Running Pod Image
echo "[4] Running Pod Image:"
echo "-----------------------------------"
POD_NAME=$(kubectl get pods -n ${NAMESPACE} -l app=${DEPLOYMENT} -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -n "$POD_NAME" ]; then
    POD_IMAGE=$(kubectl get pod -n ${NAMESPACE} ${POD_NAME} -o jsonpath='{.spec.containers[0].image}' 2>/dev/null || echo "N/A")
    POD_IMAGE_ID=$(kubectl get pod -n ${NAMESPACE} ${POD_NAME} -o jsonpath='{.status.containerStatuses[0].imageID}' 2>/dev/null || echo "N/A")
    echo "Pod: ${POD_NAME}"
    echo "Image: ${POD_IMAGE}"
    echo "Image ID: ${POD_IMAGE_ID}"
else
    echo "No pods found"
fi
echo ""

# 5. Check Minikube Images
echo "[5] Minikube Docker Images:"
echo "-----------------------------------"
eval $(minikube docker-env) && docker images ksam/core:latest --format "table {{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.CreatedAt}}\t{{.Size}}" 2>/dev/null || echo "Image not found in Minikube"
echo ""

# 6. Check Pod Logs
echo "[6] Recent Pod Logs:"
echo "-----------------------------------"
if [ -n "$POD_NAME" ]; then
    kubectl logs -n ${NAMESPACE} ${POD_NAME} --tail=20 2>&1 | grep -E "Starting HTTP|HTTP server|CORS|middleware|error|Error" | head -10 || echo "No relevant logs"
else
    echo "No pods to check logs"
fi
echo ""

# 7. Test CORS
echo "[7] CORS Test:"
echo "-----------------------------------"
pkill -f "kubectl port-forward.*8080" 2>/dev/null; sleep 2
kubectl port-forward -n ${NAMESPACE} svc/${DEPLOYMENT} 8080:8080 >/dev/null 2>&1 &
PF_PID=$!
sleep 5

if ps -p $PF_PID > /dev/null; then
    echo "Port-forward active (PID: $PF_PID)"
    
    # Test OPTIONS
    OPTIONS_RESPONSE=$(curl -s -i -X OPTIONS http://localhost:8080/api/v1/auth/login \
        -H "Origin: http://localhost:3000" \
        -H "Access-Control-Request-Method: POST" \
        -H "Access-Control-Request-Headers: Content-Type" 2>/dev/null)
    
    HTTP_CODE=$(echo "$OPTIONS_RESPONSE" | head -1 | grep -oE "HTTP/[0-9.]+ [0-9]+" | awk '{print $2}')
    CORS_ORIGIN=$(echo "$OPTIONS_RESPONSE" | grep -i "access-control-allow-origin" | head -1)
    
    echo "OPTIONS Request:"
    echo "  HTTP Code: ${HTTP_CODE}"
    if [ -n "$CORS_ORIGIN" ]; then
        echo "  ✅ CORS Origin: $CORS_ORIGIN"
    else
        echo "  ❌ CORS Origin: MISSING"
    fi
    
    # Test POST
    POST_RESPONSE=$(curl -s -i -X POST http://localhost:8080/api/v1/auth/login \
        -H "Origin: http://localhost:3000" \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"admin123"}' 2>/dev/null)
    
    POST_HTTP_CODE=$(echo "$POST_RESPONSE" | head -1 | grep -oE "HTTP/[0-9.]+ [0-9]+" | awk '{print $2}')
    POST_CORS_ORIGIN=$(echo "$POST_RESPONSE" | grep -i "access-control-allow-origin" | head -1)
    
    echo "POST Request:"
    echo "  HTTP Code: ${POST_HTTP_CODE}"
    if [ -n "$POST_CORS_ORIGIN" ]; then
        echo "  ✅ CORS Origin: $POST_CORS_ORIGIN"
    else
        echo "  ❌ CORS Origin: MISSING"
    fi
    
    kill $PF_PID 2>/dev/null || true
else
    echo "❌ Port-forward failed"
fi
echo ""

# 8. Summary
echo "=========================================="
echo "Summary"
echo "=========================================="
echo ""
if [ -n "$CORS_ORIGIN" ] && [ -n "$POST_CORS_ORIGIN" ]; then
    echo "✅ Deployment: OK"
    echo "✅ CORS: Working"
    echo ""
    echo "🌐 If browser still shows CORS error:"
    echo "  1. Clear browser cache"
    echo "  2. Use incognito window"
    echo "  3. Check browser DevTools > Network > OPTIONS request"
else
    echo "❌ CORS: Not working"
    echo ""
    echo "🔧 Check:"
    echo "  - Image is built and loaded in Minikube"
    echo "  - Deployment uses correct image"
    echo "  - Pod is running latest image"
    echo "  - CORS middleware is in code"
fi
echo ""

