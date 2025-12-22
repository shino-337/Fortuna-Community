#!/bin/bash
# Compare Database vs Kubernetes Cluster Data

set -e

NAMESPACE="ksam"
POSTGRES_POD=$(kubectl get pods -n ${NAMESPACE} -l app=postgres -o jsonpath='{.items[0].metadata.name}')

echo "=========================================="
echo "Database vs Kubernetes Comparison"
echo "=========================================="
echo ""

# Database counts
echo "[1] Database Counts"
echo "-----------------------------------"
DB_PODS=$(kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM pods;" | xargs)
DB_SAS=$(kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM service_accounts;" | xargs)
DB_ROLES=$(kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM roles;" | xargs)
DB_CLUSTER_ROLES=$(kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM cluster_roles;" | xargs)
DB_ROLE_BINDINGS=$(kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM role_bindings;" | xargs)
DB_CLUSTER_ROLE_BINDINGS=$(kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM cluster_role_bindings;" | xargs)
DB_DEPLOYMENTS=$(kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM deployments;" | xargs)
DB_INSIGHTS=$(kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights;" | xargs)

echo "  Pods: ${DB_PODS}"
echo "  ServiceAccounts: ${DB_SAS}"
echo "  Roles: ${DB_ROLES}"
echo "  ClusterRoles: ${DB_CLUSTER_ROLES}"
echo "  RoleBindings: ${DB_ROLE_BINDINGS}"
echo "  ClusterRoleBindings: ${DB_CLUSTER_ROLE_BINDINGS}"
echo "  Deployments: ${DB_DEPLOYMENTS}"
echo "  Insights: ${DB_INSIGHTS}"
echo ""

# Kubernetes counts
echo "[2] Kubernetes Cluster Counts"
echo "-----------------------------------"
K8S_PODS=$(kubectl get pods --all-namespaces --no-headers 2>/dev/null | wc -l | xargs)
K8S_SAS=$(kubectl get serviceaccounts --all-namespaces --no-headers 2>/dev/null | wc -l | xargs)
K8S_ROLES=$(kubectl get roles --all-namespaces --no-headers 2>/dev/null | wc -l | xargs)
K8S_CLUSTER_ROLES=$(kubectl get clusterroles --no-headers 2>/dev/null | wc -l | xargs)
K8S_ROLE_BINDINGS=$(kubectl get rolebindings --all-namespaces --no-headers 2>/dev/null | wc -l | xargs)
K8S_CLUSTER_ROLE_BINDINGS=$(kubectl get clusterrolebindings --no-headers 2>/dev/null | wc -l | xargs)
K8S_DEPLOYMENTS=$(kubectl get deployments --all-namespaces --no-headers 2>/dev/null | wc -l | xargs)

echo "  Pods: ${K8S_PODS}"
echo "  ServiceAccounts: ${K8S_SAS}"
echo "  Roles: ${K8S_ROLES}"
echo "  ClusterRoles: ${K8S_CLUSTER_ROLES}"
echo "  RoleBindings: ${K8S_ROLE_BINDINGS}"
echo "  ClusterRoleBindings: ${K8S_CLUSTER_ROLE_BINDINGS}"
echo "  Deployments: ${K8S_DEPLOYMENTS}"
echo ""

# Comparison
echo "[3] Comparison"
echo "-----------------------------------"
compare() {
  local resource=$1
  local db=$2
  local k8s=$3
  local diff=$((db - k8s))
  
  if [ "$db" -eq "$k8s" ]; then
    echo "  ✅ ${resource}: Match (${db})"
  else
    echo "  ⚠️  ${resource}: Mismatch (DB: ${db}, K8s: ${k8s}, Diff: ${diff})"
  fi
}

compare "Pods" "$DB_PODS" "$K8S_PODS"
compare "ServiceAccounts" "$DB_SAS" "$K8S_SAS"
compare "Roles" "$DB_ROLES" "$K8S_ROLES"
compare "ClusterRoles" "$DB_CLUSTER_ROLES" "$K8S_CLUSTER_ROLES"
compare "RoleBindings" "$DB_ROLE_BINDINGS" "$K8S_ROLE_BINDINGS"
compare "ClusterRoleBindings" "$DB_CLUSTER_ROLE_BINDINGS" "$K8S_CLUSTER_ROLE_BINDINGS"
compare "Deployments" "$DB_DEPLOYMENTS" "$K8S_DEPLOYMENTS"
echo ""

# Namespace breakdown
echo "[4] Database - Pods by Namespace"
echo "-----------------------------------"
kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -c "SELECT namespace, COUNT(*) as count FROM pods GROUP BY namespace ORDER BY count DESC LIMIT 10;" 2>/dev/null
echo ""

echo "[5] Database - ServiceAccounts by Namespace"
echo "-----------------------------------"
kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -c "SELECT namespace, COUNT(*) as count FROM service_accounts GROUP BY namespace ORDER BY count DESC LIMIT 10;" 2>/dev/null
echo ""

echo "[6] Database - Insights by Severity"
echo "-----------------------------------"
kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -c "SELECT LOWER(severity) as severity, COUNT(*) as count FROM insights GROUP BY LOWER(severity) ORDER BY count DESC;" 2>/dev/null
echo ""

echo "=========================================="
echo "Comparison Complete"
echo "=========================================="

