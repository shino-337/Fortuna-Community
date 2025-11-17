#!/bin/bash

# Script to create test audit logs for testing the audit logs UI

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}Creating test audit logs...${NC}"

# Get postgres pod name
POSTGRES_POD=$(kubectl get pods -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}')

if [ -z "$POSTGRES_POD" ]; then
    echo -e "${YELLOW}Error: Postgres pod not found${NC}"
    exit 1
fi

# Get admin user ID
ADMIN_USER_ID=$(kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT id FROM users WHERE username = 'admin' LIMIT 1;" 2>/dev/null | tr -d ' ')

if [ -z "$ADMIN_USER_ID" ]; then
    echo -e "${YELLOW}Warning: Admin user not found, using user_id = 1${NC}"
    ADMIN_USER_ID=1
fi

# Get cluster ID
CLUSTER_ID=$(kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT id FROM clusters LIMIT 1;" 2>/dev/null | tr -d ' ')

if [ -z "$CLUSTER_ID" ]; then
    echo -e "${YELLOW}Warning: No cluster found, using 'minikube'${NC}"
    CLUSTER_ID="minikube"
fi

echo -e "${BLUE}Using user_id: $ADMIN_USER_ID, cluster_id: $CLUSTER_ID${NC}"

# Create test audit logs
kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam <<EOF
-- Create audit logs for different actions and resources
INSERT INTO audit_logs (cluster_id, user_id, action, resource, resource_id, "user", ip, details, created_at) VALUES
('$CLUSTER_ID', $ADMIN_USER_ID, 'create', 'serviceaccount', '1', 'system', 'agent-sync', '{"source":"agent-sync","namespace":"default","name":"test-sa-1"}', NOW()),
('$CLUSTER_ID', $ADMIN_USER_ID, 'update', 'serviceaccount', '2', 'admin', '192.168.1.100', '{"source":"dashboard","namespace":"default","name":"test-sa-2"}', NOW() - INTERVAL '30 minutes'),
('$CLUSTER_ID', $ADMIN_USER_ID, 'delete', 'serviceaccount', '3', 'admin', '192.168.1.100', '{"source":"dashboard","namespace":"default","name":"test-sa-3"}', NOW() - INTERVAL '1 hour'),
('$CLUSTER_ID', $ADMIN_USER_ID, 'create', 'rolebinding', '1', 'system', 'agent-sync', '{"source":"agent-sync","namespace":"default"}', NOW() - INTERVAL '2 hours'),
('$CLUSTER_ID', $ADMIN_USER_ID, 'update', 'clusterrole', '1', 'admin', '192.168.1.100', '{"source":"dashboard"}', NOW() - INTERVAL '3 hours'),
('$CLUSTER_ID', $ADMIN_USER_ID, 'create', 'serviceaccount', '4', 'system', 'agent-sync', '{"source":"agent-sync","namespace":"kube-system"}', NOW() - INTERVAL '4 hours'),
('$CLUSTER_ID', $ADMIN_USER_ID, 'delete', 'rolebinding', '2', 'admin', '192.168.1.100', '{"source":"dashboard","namespace":"default"}', NOW() - INTERVAL '5 hours')
ON CONFLICT DO NOTHING;
EOF

echo -e "${GREEN}✓ Test audit logs created${NC}"

# Show created logs
echo -e "\n${BLUE}Recent audit logs:${NC}"
kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT id, action, resource, \"user\", ip, created_at FROM audit_logs ORDER BY created_at DESC LIMIT 10;" 2>&1

echo -e "\n${BLUE}Total audit logs:${NC}"
kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT COUNT(*) as total FROM audit_logs;" 2>&1

echo -e "\n${YELLOW}Note: Check the audit logs page at http://localhost:3000/audit${NC}"

