#!/bin/bash

# Script to monitor SBOM pipeline performance
# Tracks SBOM generation time, CVE matching time, and resource usage

set -e

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║          SBOM PIPELINE PERFORMANCE MONITORING                  ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Get Core pod name
CORE_POD=$(kubectl get pods -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$CORE_POD" ]; then
    echo "❌ Error: Core pod not found"
    exit 1
fi

echo "📊 Core pod: $CORE_POD"
echo ""

# Get database connection
DB_URL=$(kubectl exec -n ksam $CORE_POD -- env | grep DATABASE_URL | cut -d'=' -f2- || echo "")
DB_USER=$(echo $DB_URL | sed -n 's|postgres://\([^:]*\):.*|\1|p')
DB_PASS=$(echo $DB_URL | sed -n 's|postgres://[^:]*:\([^@]*\)@.*|\1|p')
DB_HOST=$(echo $DB_URL | sed -n 's|postgres://[^@]*@\([^:]*\):.*|\1|p')
DB_PORT=$(echo $DB_URL | sed -n 's|postgres://[^@]*@[^:]*:\([^/]*\)/.*|\1|p')
DB_NAME=$(echo $DB_URL | sed -n 's|postgres://[^@]*@[^/]*/\(.*\)|\1|p')

if [ -z "$DB_URL" ]; then
    echo "⚠️  Could not get database connection"
    exit 1
fi

echo "📊 SBOM Statistics:"
echo "════════════════════════════════════════════════════════════════"
kubectl exec -n ksam $CORE_POD -- sh -c "
    PGPASSWORD='$DB_PASS' psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c '
    SELECT 
        COUNT(*) as total_sboms,
        COUNT(DISTINCT image_digest) as unique_images,
        SUM(component_count) as total_components,
        AVG(component_count)::int as avg_components_per_sbom,
        MAX(generated_at) as last_generated
    FROM sboms
    WHERE deleted_at IS NULL;
    '
" 2>/dev/null || echo "   (Could not query)"

echo ""
echo "📊 CVE Matching Statistics:"
echo "════════════════════════════════════════════════════════════════"
kubectl exec -n ksam $CORE_POD -- sh -c "
    PGPASSWORD='$DB_PASS' psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c '
    SELECT 
        COUNT(*) as total_matches,
        COUNT(DISTINCT cve_id) as unique_cves,
        COUNT(DISTINCT sbom_id) as sboms_with_cves,
        COUNT(*) FILTER (WHERE severity = '\''CRITICAL'\'') as critical_count,
        COUNT(*) FILTER (WHERE severity = '\''HIGH'\'') as high_count,
        COUNT(*) FILTER (WHERE severity = '\''MEDIUM'\'') as medium_count,
        COUNT(*) FILTER (WHERE severity = '\''LOW'\'') as low_count
    FROM cve_matches
    WHERE deleted_at IS NULL;
    '
" 2>/dev/null || echo "   (Could not query)"

echo ""
echo "📊 SBOM Cache Efficiency:"
echo "════════════════════════════════════════════════════════════════"
kubectl exec -n ksam $CORE_POD -- sh -c "
    PGPASSWORD='$DB_PASS' psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c '
    SELECT 
        COUNT(*) FILTER (WHERE use_count = 1) as first_time_generations,
        COUNT(*) FILTER (WHERE use_count > 1) as cache_hits,
        ROUND(100.0 * COUNT(*) FILTER (WHERE use_count > 1) / NULLIF(COUNT(*), 0), 2) as cache_hit_percentage,
        AVG(use_count)::numeric(10,2) as avg_use_count,
        MAX(use_count) as max_use_count
    FROM sboms
    WHERE deleted_at IS NULL;
    '
" 2>/dev/null || echo "   (Could not query)"

echo ""
echo "📊 Resource Usage (Core Pod):"
echo "════════════════════════════════════════════════════════════════"
kubectl top pod $CORE_POD -n ksam 2>/dev/null || echo "   (Metrics not available - install metrics-server)"

echo ""
echo "📊 Recent SBOM Activity (Last 10):"
echo "════════════════════════════════════════════════════════════════"
kubectl exec -n ksam $CORE_POD -- sh -c "
    PGPASSWORD='$DB_PASS' psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c '
    SELECT 
        image_name || '\'':'\'' || image_tag as image,
        component_count,
        os_packages,
        language_packages,
        use_count,
        generated_at,
        last_used_at
    FROM sboms
    WHERE deleted_at IS NULL
    ORDER BY generated_at DESC
    LIMIT 10;
    '
" 2>/dev/null || echo "   (Could not query)"

echo ""
echo "📊 Top Vulnerable Images:"
echo "════════════════════════════════════════════════════════════════"
kubectl exec -n ksam $CORE_POD -- sh -c "
    PGPASSWORD='$DB_PASS' psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c '
    SELECT 
        s.image_name || '\'':'\'' || s.image_tag as image,
        COUNT(cm.id) as total_cves,
        COUNT(cm.id) FILTER (WHERE cm.severity = '\''CRITICAL'\'') as critical,
        COUNT(cm.id) FILTER (WHERE cm.severity = '\''HIGH'\'') as high,
        MAX(cm.cvss_score) as max_cvss
    FROM sboms s
    LEFT JOIN cve_matches cm ON s.id = cm.sbom_id AND cm.deleted_at IS NULL
    WHERE s.deleted_at IS NULL
    GROUP BY s.id, s.image_name, s.image_tag
    HAVING COUNT(cm.id) > 0
    ORDER BY 
        COUNT(cm.id) FILTER (WHERE cm.severity IN ('\''CRITICAL'\'', '\''HIGH'\'')) DESC,
        COUNT(cm.id) DESC
    LIMIT 10;
    '
" 2>/dev/null || echo "   (Could not query)"

echo ""
echo "✅ Monitoring complete"
echo ""
echo "💡 Tips:"
echo "   - High cache hit percentage = efficient SBOM reuse"
echo "   - Monitor use_count to see which images are scanned most"
echo "   - Check last_used_at to identify stale SBOMs"


