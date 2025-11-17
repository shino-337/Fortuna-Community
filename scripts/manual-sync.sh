#!/bin/bash

# Script to manually trigger Agent sync by collecting data and sending to Core
# This can be run from a pod with kubectl access or locally with kubeconfig

set -e

CLUSTER_ID="${CLUSTER_ID:-minikube}"
CORE_ENDPOINT="${CORE_ENDPOINT:-http://ksam-core:8080}"
NAMESPACE="${NAMESPACE:-}"

echo "=========================================="
echo "Manual Sync Script"
echo "=========================================="
echo "Cluster ID: $CLUSTER_ID"
echo "Core Endpoint: $CORE_ENDPOINT"
echo "Namespace filter: ${NAMESPACE:-all namespaces}"
echo ""

# Function to collect ServiceAccounts
collect_serviceaccounts() {
  local ns_filter="$1"
  if [ -z "$ns_filter" ]; then
    kubectl get serviceaccounts --all-namespaces -o json
  else
    kubectl get serviceaccounts -n "$ns_filter" -o json
  fi
}

# Function to collect RoleBindings
collect_rolebindings() {
  local ns_filter="$1"
  if [ -z "$ns_filter" ]; then
    kubectl get rolebindings --all-namespaces -o json
  else
    kubectl get rolebindings -n "$ns_filter" -o json
  fi
}

# Function to collect ClusterRoleBindings
collect_clusterrolebindings() {
  kubectl get clusterrolebindings -o json
}

# Function to collect Roles
collect_roles() {
  local ns_filter="$1"
  if [ -z "$ns_filter" ]; then
    kubectl get roles --all-namespaces -o json
  else
    kubectl get roles -n "$ns_filter" -o json
  fi
}

# Function to collect ClusterRoles
collect_clusterroles() {
  kubectl get clusterroles -o json
}

# Function to collect Pods
collect_pods() {
  local ns_filter="$1"
  if [ -z "$ns_filter" ]; then
    kubectl get pods --all-namespaces -o json
  else
    kubectl get pods -n "$ns_filter" -o json
  fi
}

# Function to convert ServiceAccount to JSON format
convert_sa() {
  local sa_json="$1"
  echo "$sa_json" | jq -c '{
    name: .metadata.name,
    namespace: .metadata.namespace,
    uid: .metadata.uid,
    labels: (.metadata.labels // {}),
    secrets: ([.secrets[]?.name // empty] | if length > 0 then . else [] end)
  }'
}

# Function to convert RoleBinding to JSON format
convert_rb() {
  local rb_json="$1"
  echo "$rb_json" | jq -c '{
    name: .metadata.name,
    namespace: .metadata.namespace,
    uid: .metadata.uid,
    roleRef: .roleRef,
    subjects: .subjects
  }'
}

# Function to convert ClusterRoleBinding to JSON format
convert_crb() {
  local crb_json="$1"
  echo "$crb_json" | jq -c '{
    name: .metadata.name,
    uid: .metadata.uid,
    roleRef: .roleRef,
    subjects: .subjects
  }'
}

# Function to convert Role to JSON format
convert_role() {
  local role_json="$1"
  echo "$role_json" | jq -c '{
    name: .metadata.name,
    namespace: .metadata.namespace,
    uid: .metadata.uid,
    rules: .rules
  }'
}

# Function to convert ClusterRole to JSON format
convert_cr() {
  local cr_json="$1"
  echo "$cr_json" | jq -c '{
    name: .metadata.name,
    uid: .metadata.uid,
    rules: .rules
  }'
}

# Function to convert Pod to JSON format
convert_pod() {
  local pod_json="$1"
  local sa_name=$(echo "$pod_json" | jq -r '.spec.serviceAccountName // "default"')
  echo "$pod_json" | jq -c --arg sa "$sa_name" '{
    name: .metadata.name,
    namespace: .metadata.namespace,
    serviceAccount: $sa,
    uid: .metadata.uid
  }'
}

echo "Step 1: Collecting ServiceAccounts..."
SAS=$(collect_serviceaccounts "$NAMESPACE")
SA_ARRAY=$(echo "$SAS" | jq -c '[.items[] | {
  name: .metadata.name,
  namespace: .metadata.namespace,
  uid: .metadata.uid,
  labels: (.metadata.labels // {}),
  secrets: ([.secrets[]?.name // empty] | if length > 0 then . else [] end)
}]')
SA_COUNT=$(echo "$SA_ARRAY" | jq 'length')
echo "✓ Collected $SA_COUNT ServiceAccounts"

echo ""
echo "Step 2: Collecting RoleBindings..."
RBS=$(collect_rolebindings "$NAMESPACE")
RB_ARRAY=$(echo "$RBS" | jq -c '[.items[] | {
  name: .metadata.name,
  namespace: .metadata.namespace,
  uid: .metadata.uid,
  roleRef: .roleRef,
  subjects: .subjects
}]')
RB_COUNT=$(echo "$RB_ARRAY" | jq 'length')
echo "✓ Collected $RB_COUNT RoleBindings"

echo ""
echo "Step 3: Collecting ClusterRoleBindings..."
CRBS=$(collect_clusterrolebindings)
CRB_ARRAY=$(echo "$CRBS" | jq -c '[.items[] | {
  name: .metadata.name,
  uid: .metadata.uid,
  roleRef: .roleRef,
  subjects: .subjects
}]')
CRB_COUNT=$(echo "$CRB_ARRAY" | jq 'length')
echo "✓ Collected $CRB_COUNT ClusterRoleBindings"

echo ""
echo "Step 4: Collecting Roles..."
ROLES=$(collect_roles "$NAMESPACE")
ROLE_ARRAY=$(echo "$ROLES" | jq -c '[.items[] | {
  name: .metadata.name,
  namespace: .metadata.namespace,
  uid: .metadata.uid,
  rules: .rules
}]')
ROLE_COUNT=$(echo "$ROLE_ARRAY" | jq 'length')
echo "✓ Collected $ROLE_COUNT Roles"

echo ""
echo "Step 5: Collecting ClusterRoles..."
CRS=$(collect_clusterroles)
CR_ARRAY=$(echo "$CRS" | jq -c '[.items[] | {
  name: .metadata.name,
  uid: .metadata.uid,
  rules: .rules
}]')
CR_COUNT=$(echo "$CR_ARRAY" | jq 'length')
echo "✓ Collected $CR_COUNT ClusterRoles"

echo ""
echo "Step 6: Collecting Pods..."
PODS=$(collect_pods "$NAMESPACE")
POD_ARRAY=$(echo "$PODS" | jq -c '[.items[] | {
  name: .metadata.name,
  namespace: .metadata.namespace,
  serviceAccount: (.spec.serviceAccountName // "default"),
  uid: .metadata.uid
}]')
POD_COUNT=$(echo "$POD_ARRAY" | jq 'length')
echo "✓ Collected $POD_COUNT Pods"

echo ""
echo "Step 7: Sending data to Core Controller..."

# Build payload
PAYLOAD=$(jq -n --arg cluster "$CLUSTER_ID" \
  --argjson sas "$SA_ARRAY" \
  --argjson rbs "$RB_ARRAY" \
  --argjson crbs "$CRB_ARRAY" \
  --argjson roles "$ROLE_ARRAY" \
  --argjson crs "$CR_ARRAY" \
  --argjson pods "$POD_ARRAY" \
  '{
    clusterId: $cluster,
    data: {
      clusterId: $cluster,
      serviceAccounts: $sas,
      roleBindings: $rbs,
      clusterRoleBindings: $crbs,
      roles: $roles,
      clusterRoles: $crs,
      pods: $pods
    }
  }')

# Send to Core
RESPONSE=$(curl -s -X POST "$CORE_ENDPOINT/api/v1/agent/sync" \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD")

if echo "$RESPONSE" | jq -e '.success' > /dev/null 2>&1; then
  echo "✓ Sync successful!"
  echo "Response: $RESPONSE"
else
  echo "✗ Sync failed!"
  echo "Response: $RESPONSE"
  exit 1
fi

echo ""
echo "=========================================="
echo "Sync Summary"
echo "=========================================="
echo "ServiceAccounts: $SA_COUNT"
echo "RoleBindings: $RB_COUNT"
echo "ClusterRoleBindings: $CRB_COUNT"
echo "Roles: $ROLE_COUNT"
echo "ClusterRoles: $CR_COUNT"
echo "Pods: $POD_COUNT"
echo ""

