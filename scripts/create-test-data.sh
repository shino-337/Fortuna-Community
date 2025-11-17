#!/bin/bash

# Script to create test data in Kubernetes cluster

set -e

echo "🧪 Creating test data for KSAM..."

# Create test namespace
kubectl create namespace test-ksam --dry-run=client -o yaml | kubectl apply -f -

# Create test ServiceAccounts
echo "Creating test ServiceAccounts..."
kubectl create serviceaccount test-sa-1 -n test-ksam --dry-run=client -o yaml | kubectl apply -f -
kubectl create serviceaccount test-sa-2 -n test-ksam --dry-run=client -o yaml | kubectl apply -f -
kubectl create serviceaccount test-sa-3 -n default --dry-run=client -o yaml | kubectl apply -f -
kubectl create serviceaccount test-sa-4 -n kube-system --dry-run=client -o yaml | kubectl apply -f -

# Create test Roles
echo "Creating test Roles..."
kubectl create role test-role -n test-ksam \
  --verb=get,list,watch \
  --resource=pods \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create role test-role-2 -n test-ksam \
  --verb=* \
  --resource=* \
  --dry-run=client -o yaml | kubectl apply -f -

# Create test RoleBindings
echo "Creating test RoleBindings..."
kubectl create rolebinding test-rb-1 -n test-ksam \
  --role=test-role \
  --serviceaccount=test-ksam:test-sa-1 \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl create rolebinding test-rb-2 -n test-ksam \
  --role=test-role-2 \
  --serviceaccount=test-ksam:test-sa-2 \
  --dry-run=client -o yaml | kubectl apply -f -

# Create test ClusterRoleBinding
echo "Creating test ClusterRoleBinding..."
kubectl create clusterrolebinding test-crb \
  --clusterrole=view \
  --serviceaccount=test-ksam:test-sa-3 \
  --dry-run=client -o yaml | kubectl apply -f -

# Create test Pods using ServiceAccounts
echo "Creating test Pods..."
kubectl run test-pod-1 -n test-ksam \
  --image=nginx:alpine \
  --serviceaccount=test-sa-1 \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl run test-pod-2 -n test-ksam \
  --image=nginx:alpine \
  --serviceaccount=test-sa-2 \
  --dry-run=client -o yaml | kubectl apply -f -

# Wait for pods
echo "Waiting for pods to be ready..."
kubectl wait --for=condition=ready pod -l run=test-pod-1 -n test-ksam --timeout=60s || true
kubectl wait --for=condition=ready pod -l run=test-pod-2 -n test-ksam --timeout=60s || true

echo "✅ Test data created!"
echo ""
echo "Created resources:"
echo "  - Namespace: test-ksam"
echo "  - ServiceAccounts: 4"
echo "  - Roles: 2"
echo "  - RoleBindings: 2"
echo "  - ClusterRoleBinding: 1"
echo "  - Pods: 2"
echo ""
echo "Check in Dashboard after Agent syncs (30s interval)"

