#!/bin/bash

# Script to list all insights from database

set -e

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║              INSIGHTS LISTING REPORT                          ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Get PostgreSQL pod
POSTGRES_POD=$(kubectl get pods -n ksam -l app=postgres -o jsonpath='{.items[0].metadata.name}')

if [ -z "$POSTGRES_POD" ]; then
    echo "❌ Error: PostgreSQL pod not found"
    exit 1
fi

echo "📊 Querying insights from database..."
echo ""

# Total count
TOTAL=$(kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM insights;")
echo "📈 Total Insights: $TOTAL"
echo ""

# Breakdown by type and severity
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📋 Breakdown by Type and Severity:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -c "
SELECT 
    type,
    severity,
    COUNT(*) as count,
    MIN(created_at) as first_created,
    MAX(updated_at) as last_updated
FROM insights 
GROUP BY type, severity 
ORDER BY 
    CASE severity 
        WHEN 'Critical' THEN 1 
        WHEN 'High' THEN 2 
        WHEN 'Medium' THEN 3 
        WHEN 'Low' THEN 4 
        ELSE 5 
    END,
    count DESC;
"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔴 Critical Insights (Top 10):"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -c "
SELECT 
    id,
    type,
    LEFT(description, 100) as description,
    created_at
FROM insights 
WHERE severity = 'Critical'
ORDER BY created_at DESC
LIMIT 10;
"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🟠 High Severity Insights (Top 10):"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -c "
SELECT 
    id,
    type,
    LEFT(description, 100) as description,
    created_at
FROM insights 
WHERE severity = 'High'
ORDER BY created_at DESC
LIMIT 10;
"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📊 Recent Insights (Last 10):"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
kubectl exec -n ksam $POSTGRES_POD -- psql -U postgres -d ksam -c "
SELECT 
    id,
    type,
    severity,
    LEFT(description, 80) as description,
    created_at,
    updated_at
FROM insights 
ORDER BY updated_at DESC
LIMIT 10;
"

echo ""
echo "✅ Insights listing completed"

