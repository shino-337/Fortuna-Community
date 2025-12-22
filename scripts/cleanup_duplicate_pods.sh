#!/bin/bash
# Cleanup duplicate pods in database (keep only the latest entry per UID)

set -e

NAMESPACE="ksam"
POSTGRES_POD=$(kubectl get pods -n ${NAMESPACE} -l app=postgres -o jsonpath='{.items[0].metadata.name}')

echo "=========================================="
echo "Cleaning Up Duplicate Pods"
echo "=========================================="
echo ""

echo "[1] Checking for duplicate pods..."
DUPLICATES=$(kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -t -c "
    SELECT COUNT(*) FROM (
        SELECT uid, COUNT(*) as cnt 
        FROM pods 
        GROUP BY uid 
        HAVING COUNT(*) > 1
    ) as dup;" | xargs)

if [ "$DUPLICATES" -eq 0 ]; then
    echo "✅ No duplicate pods found"
    exit 0
fi

echo "⚠️  Found $DUPLICATES UIDs with duplicates"
echo ""

echo "[2] Showing duplicate UIDs..."
kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -c "
    SELECT uid, COUNT(*) as count 
    FROM pods 
    GROUP BY uid 
    HAVING COUNT(*) > 1 
    ORDER BY count DESC 
    LIMIT 10;" 2>/dev/null

echo ""
echo "[3] Cleaning up duplicates (keeping latest entry per UID)..."
kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam <<'EOF'
-- Delete duplicate pods, keeping only the one with the latest created_at
DELETE FROM pods
WHERE id IN (
    SELECT id
    FROM (
        SELECT id,
               ROW_NUMBER() OVER (PARTITION BY cluster_id, uid ORDER BY created_at DESC) as rn
        FROM pods
    ) t
    WHERE rn > 1
);
EOF

echo ""
echo "[4] Verifying cleanup..."
REMAINING=$(kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -t -c "
    SELECT COUNT(*) FROM (
        SELECT uid, COUNT(*) as cnt 
        FROM pods 
        GROUP BY uid 
        HAVING COUNT(*) > 1
    ) as dup;" | xargs)

if [ "$REMAINING" -eq 0 ]; then
    echo "✅ All duplicates cleaned up"
else
    echo "⚠️  Still have $REMAINING duplicate UIDs"
fi

echo ""
echo "[5] Final pod count..."
TOTAL=$(kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -t -c "SELECT COUNT(*) FROM pods;" | xargs)
DISTINCT=$(kubectl exec -n ${NAMESPACE} "${POSTGRES_POD}" -- psql -U postgres -d ksam -t -c "SELECT COUNT(DISTINCT uid) FROM pods;" | xargs)
echo "  Total pods: $TOTAL"
echo "  Distinct UIDs: $DISTINCT"

echo ""
echo "=========================================="
echo "Cleanup Complete"
echo "=========================================="

