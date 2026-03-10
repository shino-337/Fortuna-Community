#!/usr/bin/env bash
# =============================================================================
# Clean host: old images (fortuna + dangling), build cache, temp junk.
# Does NOT touch CVE data (cve-data/, package_vulnerabilities in DB).
# Run before full-clean-database-rebuild-deploy.sh or standalone.
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CONTAINERD_NS="${CONTAINERD_NAMESPACE:-k8s.io}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "=========================================="
echo "Host resources & clean (no CVE data touch)"
echo "=========================================="
echo ""

echo "--- Disk usage ---"
df -h / /tmp 2>/dev/null || df -h .
echo ""

echo "--- Fortuna images (before) ---"
nerdctl --namespace "$CONTAINERD_NS" images 2>/dev/null | grep -E "fortuna|REPOSITORY" || true
echo ""

echo "--- Removing ALL fortuna images from containerd (namespace=$CONTAINERD_NS) ---"
nerdctl --namespace "$CONTAINERD_NS" images 2>/dev/null | grep fortuna | awk '{print $3}' | xargs -r nerdctl --namespace "$CONTAINERD_NS" rmi --force 2>/dev/null || true
echo -e "${GREEN}[OK]${NC} Fortuna images removed"
echo ""

echo "--- System prune (dangling, unused) ---"
nerdctl --namespace "$CONTAINERD_NS" system prune -f 2>/dev/null || true
nerdctl builder prune --namespace "$CONTAINERD_NS" -a -f 2>/dev/null || true
echo -e "${GREEN}[OK]${NC} Prune done"
echo ""

echo "--- Temp junk (/tmp/fortuna-images, /var/tmp/fortuna-images) ---"
rm -rf /tmp/fortuna-images 2>/dev/null && echo "  Removed /tmp/fortuna-images" || true
rm -rf /var/tmp/fortuna-images 2>/dev/null && echo "  Removed /var/tmp/fortuna-images" || true
echo ""

echo "--- Disk usage (after) ---"
df -h / /tmp 2>/dev/null || df -h .
echo ""
echo -e "${GREEN}Done. CVE data (cve-data/, DB) not modified.${NC}"
