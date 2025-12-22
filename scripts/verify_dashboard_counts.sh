#!/bin/bash
# Verify Dashboard Counts Accuracy

set -e

API_URL="${API_URL:-http://localhost:8080}"
POSTGRES_POD=$(kubectl get pods -n ksam | grep postgres | head -1 | awk '{print $1}')

echo "=========================================="
echo "Dashboard Counts Verification"
echo "=========================================="
echo ""

# Get API Token
TOKEN=$(curl -s -X POST "$API_URL/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -H "Origin: http://localhost:3000" \
    -d '{"username":"admin","password":"admin123"}' 2>/dev/null | \
    python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('token', ''))" 2>/dev/null || echo "")

if [ -n "$TOKEN" ]; then
    AUTH_HEADER="Authorization: Bearer $TOKEN"
else
    AUTH_HEADER=""
fi

# Database Counts
echo "[1] Database Counts (Source of Truth):"
echo "-----------------------------------"
DB_CLUSTERS=$(kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM clusters;" 2>/dev/null | tr -d ' ')
DB_PODS=$(kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM pods;" 2>/dev/null | tr -d ' ')
DB_SAS=$(kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM service_accounts;" 2>/dev/null | tr -d ' ')
DB_INSIGHTS=$(kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights;" 2>/dev/null | tr -d ' ')
DB_CRITICAL=$(kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights WHERE LOWER(severity)='critical';" 2>/dev/null | tr -d ' ')
DB_HIGH=$(kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights WHERE LOWER(severity)='high';" 2>/dev/null | tr -d ' ')
DB_LOW=$(kubectl exec -n ksam "$POSTGRES_POD" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights WHERE LOWER(severity)='low';" 2>/dev/null | tr -d ' ')

echo "  Clusters: $DB_CLUSTERS"
echo "  Pods: $DB_PODS"
echo "  ServiceAccounts: $DB_SAS"
echo "  Insights Total: $DB_INSIGHTS"
echo "  Critical: $DB_CRITICAL"
echo "  High: $DB_HIGH"
echo "  Low: $DB_LOW"
echo ""

# API Counts
echo "[2] API Counts (What Dashboard Receives):"
echo "-----------------------------------"
CLUSTERS_STATS=$(curl -s -H "$AUTH_HEADER" -H "Origin: http://localhost:3000" "$API_URL/api/v1/clusters/stats" 2>/dev/null)
INSIGHTS_SUMMARY=$(curl -s -H "$AUTH_HEADER" -H "Origin: http://localhost:3000" "$API_URL/api/v1/insights/summary" 2>/dev/null)

API_CLUSTER_COUNT=$(echo "$CLUSTERS_STATS" | python3 -c "import sys, json; data=json.load(sys.stdin); clusters=data.get('clusters', []); print(len(clusters))" 2>/dev/null || echo "0")
API_TOTAL_PODS=$(echo "$CLUSTERS_STATS" | python3 -c "import sys, json; data=json.load(sys.stdin); clusters=data.get('clusters', []); print(sum(c.get('podCount', 0) for c in clusters))" 2>/dev/null || echo "0")
API_TOTAL_SAS=$(echo "$CLUSTERS_STATS" | python3 -c "import sys, json; data=json.load(sys.stdin); clusters=data.get('clusters', []); print(sum(c.get('serviceAccountCount', 0) for c in clusters))" 2>/dev/null || echo "0")

API_INSIGHTS_TOTAL=$(echo "$INSIGHTS_SUMMARY" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('total', 0))" 2>/dev/null || echo "0")
API_INSIGHTS_CRITICAL=$(echo "$INSIGHTS_SUMMARY" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('critical', 0))" 2>/dev/null || echo "0")
API_INSIGHTS_HIGH=$(echo "$INSIGHTS_SUMMARY" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('high', 0))" 2>/dev/null || echo "0")
API_INSIGHTS_LOW=$(echo "$INSIGHTS_SUMMARY" | python3 -c "import sys, json; data=json.load(sys.stdin); print(data.get('low', 0))" 2>/dev/null || echo "0")

echo "  Clusters: $API_CLUSTER_COUNT"
echo "  Pods (sum from clusters): $API_TOTAL_PODS"
echo "  ServiceAccounts (sum from clusters): $API_TOTAL_SAS"
echo "  Insights Total: $API_INSIGHTS_TOTAL"
echo "  Critical: $API_INSIGHTS_CRITICAL"
echo "  High: $API_INSIGHTS_HIGH"
echo "  Low: $API_INSIGHTS_LOW"
echo ""

# Detailed Cluster Stats
echo "[3] Cluster Details:"
echo "-----------------------------------"
echo "$CLUSTERS_STATS" | python3 -c "
import sys, json
data = json.load(sys.stdin)
clusters = data.get('clusters', [])
for c in clusters:
    print(f\"  [{c.get('name')}]\")
    print(f\"    Pods: {c.get('podCount', 0)}\")
    print(f\"    ServiceAccounts: {c.get('serviceAccountCount', 0)}\")
    print(f\"    Roles: {c.get('roleCount', 0)}\")
    print(f\"    ClusterRoles: {c.get('clusterRoleCount', 0)}\")
" 2>/dev/null || echo "$CLUSTERS_STATS"
echo ""

# Comparison
echo "[4] Comparison (Database vs API):"
echo "-----------------------------------"
echo "Metric                    | Database | API      | Match"
echo "--------------------------|----------|----------|-------"

# Clusters
if [ "$DB_CLUSTERS" = "$API_CLUSTER_COUNT" ]; then
    CLUSTER_MATCH="✅"
else
    CLUSTER_MATCH="❌"
fi
printf "Clusters                  | %8s | %8s | %s\n" "$DB_CLUSTERS" "$API_CLUSTER_COUNT" "$CLUSTER_MATCH"

# Pods
if [ "$DB_PODS" = "$API_TOTAL_PODS" ]; then
    POD_MATCH="✅"
else
    POD_MATCH="❌"
fi
printf "Pods                      | %8s | %8s | %s\n" "$DB_PODS" "$API_TOTAL_PODS" "$POD_MATCH"

# ServiceAccounts
if [ "$DB_SAS" = "$API_TOTAL_SAS" ]; then
    SA_MATCH="✅"
else
    SA_MATCH="❌"
fi
printf "ServiceAccounts           | %8s | %8s | %s\n" "$DB_SAS" "$API_TOTAL_SAS" "$SA_MATCH"

# Insights Total
if [ "$DB_INSIGHTS" = "$API_INSIGHTS_TOTAL" ]; then
    INSIGHTS_MATCH="✅"
else
    INSIGHTS_MATCH="❌"
fi
printf "Insights Total            | %8s | %8s | %s\n" "$DB_INSIGHTS" "$API_INSIGHTS_TOTAL" "$INSIGHTS_MATCH"

# Insights Critical
if [ "$DB_CRITICAL" = "$API_INSIGHTS_CRITICAL" ]; then
    CRITICAL_MATCH="✅"
else
    CRITICAL_MATCH="❌"
fi
printf "Insights Critical         | %8s | %8s | %s\n" "$DB_CRITICAL" "$API_INSIGHTS_CRITICAL" "$CRITICAL_MATCH"

# Insights High
if [ "$DB_HIGH" = "$API_INSIGHTS_HIGH" ]; then
    HIGH_MATCH="✅"
else
    HIGH_MATCH="❌"
fi
printf "Insights High             | %8s | %8s | %s\n" "$DB_HIGH" "$API_INSIGHTS_HIGH" "$HIGH_MATCH"

# Insights Low
if [ "$DB_LOW" = "$API_INSIGHTS_LOW" ]; then
    LOW_MATCH="✅"
else
    LOW_MATCH="❌"
fi
printf "Insights Low              | %8s | %8s | %s\n" "$DB_LOW" "$API_INSIGHTS_LOW" "$LOW_MATCH"

echo ""

# Expected Dashboard Display
echo "[5] Expected Dashboard Display:"
echo "-----------------------------------"
echo "Dashboard Screen:"
echo "  - Clusters: $API_CLUSTER_COUNT"
echo "  - Insights: $API_INSIGHTS_TOTAL"
echo "  - Pods: $API_TOTAL_PODS"
echo "  - Critical Issues: $API_INSIGHTS_CRITICAL"
echo ""
echo "Insights by Severity:"
echo "  - Critical: $API_INSIGHTS_CRITICAL"
echo "  - High: $API_INSIGHTS_HIGH"
echo "  - Low: $API_INSIGHTS_LOW"
echo ""

# Summary
echo "=========================================="
echo "Summary"
echo "=========================================="
echo ""
echo "✅ Database (Source of Truth):"
echo "   Clusters: $DB_CLUSTERS"
echo "   Pods: $DB_PODS"
echo "   ServiceAccounts: $DB_SAS"
echo "   Insights: $DB_INSIGHTS"
echo ""
echo "✅ API (Dashboard Data Source):"
echo "   Clusters: $API_CLUSTER_COUNT"
echo "   Pods: $API_TOTAL_PODS"
echo "   ServiceAccounts: $API_TOTAL_SAS"
echo "   Insights: $API_INSIGHTS_TOTAL"
echo ""
echo "🌐 Dashboard Should Display:"
echo "   Clusters: $API_CLUSTER_COUNT"
echo "   Pods: $API_TOTAL_PODS"
echo "   Insights: $API_INSIGHTS_TOTAL"
echo "   Critical: $API_INSIGHTS_CRITICAL"
echo ""
echo "📝 Next: Open http://localhost:3000 and verify display"
echo ""

