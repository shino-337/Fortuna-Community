#!/bin/bash

# Database Query Commands for E2E Verification
# These commands can be run directly to verify the processing flow

set -e

NAMESPACE="${NAMESPACE:-fortuna}"
TEST_POD_NAME="${TEST_POD_NAME:-test-pod-e2e-1766897554}"
POD_UID="${POD_UID:-9caa6290-5471-46de-9a0b-a43ba7937d9d}"

POSTGRES_POD=$(kubectl get pods -n $NAMESPACE -l app=postgres -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

if [ -z "$POSTGRES_POD" ]; then
    echo "❌ PostgreSQL pod not found"
    exit 1
fi

echo "=========================================="
echo "Database Query Commands"
echo "=========================================="
echo ""
echo "PostgreSQL Pod: $POSTGRES_POD"
echo "Test Pod Name: $TEST_POD_NAME"
echo "Pod UID: $POD_UID"
echo ""

echo "=========================================="
echo "1. Check Pod in Database"
echo "=========================================="
echo ""
echo "kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c \\"
echo "  \"SELECT uid, name, namespace, created_at FROM pods WHERE uid = '$POD_UID' OR name = '$TEST_POD_NAME' LIMIT 1;\""
echo ""
kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT uid, name, namespace, created_at FROM pods WHERE uid = '$POD_UID' OR name = '$TEST_POD_NAME' LIMIT 1;" 2>/dev/null || echo "No pod found"
echo ""

echo "=========================================="
echo "2. Query SBOM for Test Pod"
echo "=========================================="
echo ""
echo "kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c \\"
echo "  \"SELECT id, pod_uid, pod_name, namespace, image_name, image_digest, created_at FROM sboms WHERE pod_uid = '$POD_UID' OR pod_name = '$TEST_POD_NAME' ORDER BY created_at DESC LIMIT 5;\""
echo ""
kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT id, pod_uid, pod_name, namespace, image_name, image_digest, created_at FROM sboms WHERE pod_uid = '$POD_UID' OR pod_name = '$TEST_POD_NAME' ORDER BY created_at DESC LIMIT 5;" 2>/dev/null || echo "No SBOM found"
echo ""

# Get SBOM ID if exists
SBOM_ID=$(kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -t -A -c "SELECT id FROM sboms WHERE pod_uid = '$POD_UID' OR pod_name = '$TEST_POD_NAME' ORDER BY created_at DESC LIMIT 1;" 2>/dev/null | tr -d ' ')

if [ -n "$SBOM_ID" ] && [ "$SBOM_ID" != "" ]; then
    echo "SBOM ID found: $SBOM_ID"
    echo ""
    
    echo "=========================================="
    echo "3. Query SBOM Components"
    echo "=========================================="
    echo ""
    echo "kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c \\"
    echo "  \"SELECT COUNT(*) as total_components, COUNT(DISTINCT name) as unique_packages FROM sbom_components WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL;\""
    echo ""
    kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT COUNT(*) as total_components, COUNT(DISTINCT name) as unique_packages FROM sbom_components WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL;" 2>/dev/null
    echo ""
    
    echo "Sample components:"
    echo "kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c \\"
    echo "  \"SELECT name, version, package_type FROM sbom_components WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL LIMIT 10;\""
    echo ""
    kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT name, version, package_type FROM sbom_components WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL LIMIT 10;" 2>/dev/null
    echo ""
    
    echo "=========================================="
    echo "4. Query CVE Matches"
    echo "=========================================="
    echo ""
    echo "kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c \\"
    echo "  \"SELECT COUNT(*) as total_matches, COUNT(DISTINCT cve_id) as unique_cves FROM cve_matches WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL;\""
    echo ""
    kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT COUNT(*) as total_matches, COUNT(DISTINCT cve_id) as unique_cves FROM cve_matches WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL;" 2>/dev/null
    echo ""
    
    echo "Sample CVE matches:"
    echo "kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c \\"
    echo "  \"SELECT cve_id, package_name, package_version, cvss_score, matched_by FROM cve_matches WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL LIMIT 10;\""
    echo ""
    kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT cve_id, package_name, package_version, cvss_score, matched_by FROM cve_matches WHERE sbom_id = $SBOM_ID AND deleted_at IS NULL LIMIT 10;" 2>/dev/null
    echo ""
else
    echo "⚠️  SBOM not found yet. Skipping components and CVE queries."
    echo ""
fi

echo "=========================================="
echo "5. Query Insights"
echo "=========================================="
echo ""
echo "kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c \\"
echo "  \"SELECT COUNT(*) as total_insights, COUNT(DISTINCT cve_id) as unique_cves, string_agg(DISTINCT insight_type, ', ') as types FROM insights WHERE resource_uid = '$POD_UID' AND deleted_at IS NULL;\""
echo ""
kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT COUNT(*) as total_insights, COUNT(DISTINCT cve_id) as unique_cves, string_agg(DISTINCT insight_type, ', ') as types FROM insights WHERE resource_uid = '$POD_UID' AND deleted_at IS NULL;" 2>/dev/null
echo ""

echo "Sample insights:"
echo "kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c \\"
echo "  \"SELECT id, resource_uid, resource_type, resource_name, insight_type, severity, cvss, created_at FROM insights WHERE resource_uid = '$POD_UID' AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 10;\""
echo ""
kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT id, resource_uid, resource_type, resource_name, insight_type, severity, cvss, created_at FROM insights WHERE resource_uid = '$POD_UID' AND deleted_at IS NULL ORDER BY created_at DESC LIMIT 10;" 2>/dev/null
echo ""

echo "=========================================="
echo "6. Processing Timeline"
echo "=========================================="
echo ""

if [ -n "$SBOM_ID" ] && [ "$SBOM_ID" != "" ]; then
    echo "SBOM Creation Time:"
    kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT created_at as sbom_created FROM sboms WHERE id = $SBOM_ID;" 2>/dev/null
    echo ""
    
    echo "First CVE Match Time:"
    kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT MIN(created_at) as first_cve_match FROM cve_matches WHERE sbom_id = $SBOM_ID;" 2>/dev/null
    echo ""
fi

echo "First Insight Time:"
kubectl exec -n $NAMESPACE $POSTGRES_POD -- psql -U postgres -d ksam -c "SELECT MIN(created_at) as first_insight FROM insights WHERE resource_uid = '$POD_UID';" 2>/dev/null
echo ""

