#!/bin/bash
set -e

echo "🚀 Deploying K8s Workload Management Platform v3.0.5"
echo "=================================================="
echo ""

# Load image vào minikube
echo "📦 Loading image into minikube..."
minikube image load ksam-dashboard:v3.0.5

echo ""
echo "🔄 Deleting old pods..."
kubectl delete pod -n ksam -l app=ksam-dashboard --force --grace-period=0

echo ""
echo "⏳ Waiting for new pod to start (30 seconds)..."
sleep 30

echo ""
echo "✅ Checking deployment status..."
kubectl get pods -n ksam -l app=ksam-dashboard

echo ""
echo "📊 Checking running image..."
kubectl get pods -n ksam -l app=ksam-dashboard -o jsonpath='{.items[0].spec.containers[0].image}'
echo ""

echo ""
echo "🌐 Dashboard URL:"
echo "http://$(minikube ip):$(kubectl get service ksam-dashboard -n ksam -o jsonpath='{.spec.ports[0].nodePort}')/graph"

echo ""
echo "✅ Deployment complete!"


