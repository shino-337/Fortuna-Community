#!/bin/bash

# ============================================================================
# Verify SBOM Script
# ============================================================================
# Verifies SBOM extraction and storage in database
# ============================================================================

set -euo pipefail

POD_UID="${1:-}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-ksam}"
DB_USER="${DB_USER:-ksam}"

if [ -z "${POD_UID}" ]; then
    echo "Usage: $0 <pod_uid>"
    exit 1
fi

db_query() {
    local query=$1
    PGPASSWORD="${DB_PASSWORD:-ksam}" psql -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" -d "${DB_NAME}" -t -A -c "${query}" 2>/dev/null || echo ""
}

echo "Verifying SBOM for pod_uid: ${POD_UID}"

# Check SBOM exists
SBOM_COUNT=$(db_query "SELECT COUNT(*) FROM sboms WHERE pod_uid = '${POD_UID}' AND deleted_at IS NULL;")

if [ "${SBOM_COUNT}" -eq 0 ]; then
    echo "❌ No SBOM found for pod_uid: ${POD_UID}"
    exit 1
fi

echo "✅ SBOM found: ${SBOM_COUNT} record(s)"

# Get SBOM details
SBOM_ID=$(db_query "SELECT id FROM sboms WHERE pod_uid = '${POD_UID}' AND deleted_at IS NULL LIMIT 1;")
echo "SBOM ID: ${SBOM_ID}"

# Get SBOM components
COMPONENT_COUNT=$(db_query "SELECT COUNT(*) FROM sbom_components WHERE sbom_id = ${SBOM_ID} AND deleted_at IS NULL;")
echo "Components: ${COMPONENT_COUNT}"

# Display SBOM details
echo ""
echo "SBOM Details:"
db_query "SELECT id, pod_uid, container_name, image_digest, extracted_at FROM sboms WHERE id = ${SBOM_ID};" | while IFS='|' read -r id pod_uid container_name image_digest extracted_at; do
    echo "  ID: ${id}"
    echo "  Pod UID: ${pod_uid}"
    echo "  Container: ${container_name}"
    echo "  Image Digest: ${image_digest}"
    echo "  Extracted At: ${extracted_at}"
done

# Display sample components
echo ""
echo "Sample Components (first 10):"
db_query "SELECT component_name, component_version, purl FROM sbom_components WHERE sbom_id = ${SBOM_ID} AND deleted_at IS NULL LIMIT 10;" | while IFS='|' read -r name version purl; do
    echo "  - ${name}@${version} (${purl})"
done

echo ""
echo "✅ SBOM verification complete"

