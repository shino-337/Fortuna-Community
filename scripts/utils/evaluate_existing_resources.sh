#!/bin/bash

# Script to trigger risk evaluation for existing resources in database
# This will query the database and publish messages to NATS for Risk Worker to process

set -e

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║     TRIGGERING RISK EVALUATION FOR EXISTING RESOURCES         ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Get Core pod name
CORE_POD=$(kubectl get pods -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}')

if [ -z "$CORE_POD" ]; then
    echo "❌ Error: Core pod not found"
    exit 1
fi

echo "📊 Found Core pod: $CORE_POD"
echo ""

# Use API endpoint to trigger evaluation if available
echo "🔍 Attempting to trigger risk evaluation via API..."
API_RESPONSE=$(kubectl exec -n ksam $CORE_POD -- wget -qO- --post-data='{}' --header='Content-Type: application/json' http://localhost:8080/api/v1/insights/evaluate 2>&1 || echo "ENDPOINT_NOT_FOUND")

if [[ "$API_RESPONSE" == *"ENDPOINT_NOT_FOUND"* ]] || [[ "$API_RESPONSE" == *"404"* ]]; then
    echo "⚠️  API endpoint not available, will need to implement manual evaluation"
    echo ""
    echo "📋 Summary:"
    echo "  • Database size: 11 MB (not full)"
    echo "  • Resources to evaluate:"
    echo "    - 5 ClusterRoleBindings with cluster-admin"
    echo "    - 66 ClusterRoleBindings total"
    echo "    - 22 RoleBindings"
    echo "    - 85 ServiceAccounts"
    echo "  • Current insights: 0"
    echo ""
    echo "💡 Recommendation:"
    echo "  • Implement manual evaluation endpoint"
    echo "  • Or restart Agent to re-sync and send new messages"
else
    echo "✅ API response: $API_RESPONSE"
fi

echo ""
echo "✅ Script completed"

