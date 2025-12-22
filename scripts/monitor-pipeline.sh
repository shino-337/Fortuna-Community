#!/bin/bash

# Real-time monitoring script for KSAM pipeline
# Shows logs from all workers as they process events

NAMESPACE="${1:-ksam}"
POD_FILTER="${2:-}"

echo "========================================"
echo "KSAM Pipeline Monitor"
echo "========================================"
echo "Namespace: $NAMESPACE"
echo "Filter: ${POD_FILTER:-all pods}"
echo ""
echo "Monitoring workers:"
echo "  - NormalizerWorker (raw → normalized)"
echo "  - SBOMWorker (normalized → SBOM)"
echo "  - CVEMatcherWorker (SBOM → CVE matches)"
echo "  - InsightManager (CVE matches → insights)"
echo ""
echo "Press Ctrl+C to stop"
echo "========================================"
echo ""

# Build grep pattern
if [ -n "$POD_FILTER" ]; then
  PATTERN="NormalizerWorker.*$POD_FILTER|SBOMWorker.*$POD_FILTER|CVEMatcher.*$POD_FILTER|InsightManager.*$POD_FILTER"
else
  PATTERN="NormalizerWorker|SBOMWorker|CVEMatcher|Insight.*Created"
fi

# Follow logs with color highlighting
kubectl logs -n "$NAMESPACE" -l app=ksam-core -f --tail=50 2>&1 | \
  grep -E --line-buffered "$PATTERN" | \
  sed -u \
    -e 's/\[NormalizerWorker\]/\x1b[36m[NormalizerWorker]\x1b[0m/g' \
    -e 's/\[SBOMWorker\]/\x1b[32m[SBOMWorker]\x1b[0m/g' \
    -e 's/\[CVEMatcherWorker\]/\x1b[33m[CVEMatcherWorker]\x1b[0m/g' \
    -e 's/✅/\x1b[32m✅\x1b[0m/g' \
    -e 's/❌/\x1b[31m❌\x1b[0m/g' \
    -e 's/⚠️ /\x1b[33m⚠️ \x1b[0m/g'
