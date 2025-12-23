#!/bin/bash
#
# KSAM Documentation Reorganization Script
# =========================================
# Reorganizes 524+ documentation files into logical structure
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KSAM_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DOCS_DIR="$KSAM_ROOT/docs"
cd "$DOCS_DIR"

echo "=========================================="
echo "KSAM Documentation Reorganization"
echo "=========================================="
echo ""

# Backup current state
echo "[1/6] Creating backup..."
BACKUP_DIR="../docs_backup_$(date +%Y%m%d_%H%M%S)"
cp -r . "$BACKUP_DIR"
echo "✅ Backup created: $BACKUP_DIR"
echo ""

# Function to move file safely
move_doc() {
    local src="$1"
    local dest_dir="$2"
    local dest_name="${3:-$(basename "$src")}"

    if [ -f "$src" ]; then
        mkdir -p "$dest_dir"
        mv "$src" "$dest_dir/$dest_name"
        echo "  ✓ Moved: $src → $dest_dir/$dest_name"
    fi
}

echo "[2/6] Organizing CVE documentation..."
# CVE Documentation
move_doc "CVE_DATABASE_DESIGN_OSV.md" "03-components/cve-scanner" "osv-database-design.md"
move_doc "CVE_OSV_VS_TRIVY_COMPARISON.md" "03-components/cve-scanner" "trivy-comparison.md"
move_doc "CVE_QUICK_START_GUIDE.md" "03-components/cve-scanner" "quick-start-guide.md"
move_doc "CVE_DETECTION_E2E_TEST_RESULTS.md" "archive/mvp2/test-reports"
move_doc "TRIVY_USAGE_CLARIFICATION.md" "03-components/cve-scanner"
echo "✅ CVE docs organized"
echo ""

echo "[3/6] Organizing SBOM documentation..."
# SBOM Documentation (move from existing SBOM folder)
if [ -d "SBOM" ]; then
    mv SBOM/* 03-components/sbom/ 2>/dev/null || true
    rmdir SBOM 2>/dev/null || true
fi
move_doc "SBOM_DUPLICATE_KEY_FIX.md" "03-components/sbom"
move_doc "SBOM_FIX_SUMMARY.md" "03-components/sbom"
move_doc "SBOM_FIX_TEST_RESULTS.md" "03-components/sbom"
move_doc "LOCAL_FIRST_SBOM_EXTRACTION.md" "03-components/sbom"
echo "✅ SBOM docs organized"
echo ""

echo "[4/6] Organizing architecture documentation..."
# Architecture Documentation
move_doc "ARCHITECTURE_UPDATE_PLAN.md" "02-architecture/changelog" "update-plan.md"
move_doc "ARCHITECTURE_V2_CHANGELOG.md" "02-architecture/changelog" "v2-changelog.md"
# Keep ARCHITECTURE.md at root
echo "✅ Architecture docs organized"
echo ""

echo "[5/6] Organizing migration & deployment docs..."
# Migrations
move_doc "MIGRATION_021_EXECUTION_ISSUE_FIX.md" "06-development/migrations"
move_doc "MIGRATION_021_FIX_SUMMARY.md" "06-development/migrations"
move_doc "MIGRATION_022_CVE_COLUMNS_FIX.md" "06-development/migrations"
echo "✅ Migration docs organized"
echo ""

echo "[6/6] Organizing insights & verification docs..."
# Insights & Verification
move_doc "INSIGHTS_VERIFICATION_REPORT.md" "03-components/risk-engine"
move_doc "INSIGHTS_VERIFICATION_SUMMARY.md" "03-components/risk-engine"
echo "✅ Insights docs organized"
echo ""

echo "[7/6] Moving test reports to archive..."
# Move all test reports to archive
if [ -d "test_reports" ]; then
    mv test_reports/* archive/mvp1/test-reports/ 2>/dev/null || true
    rmdir test_reports 2>/dev/null || true
fi

# Move existing archive test reports
if [ -d "archive/tests" ]; then
    mv archive/tests/* archive/mvp1/test-reports/ 2>/dev/null || true
    rmdir archive/tests 2>/dev/null || true
fi
echo "✅ Test reports archived"
echo ""

echo "[8/6] Organizing guides and specs..."
# Move existing guides
if [ -d "guides/implementation" ]; then
    mv guides/implementation/* 06-development/setup/ 2>/dev/null || true
fi
if [ -d "guides/setup" ]; then
    mv guides/setup/* 01-getting-started/ 2>/dev/null || true
fi
if [ -d "guides/testing" ]; then
    mv guides/testing/* 06-development/testing/ 2>/dev/null || true
fi

# Move specs to features
if [ -d "specs" ]; then
    mv specs/* 07-features/ 2>/dev/null || true
    rmdir specs 2>/dev/null || true
fi

# Clean up empty guides folder
rm -rf guides 2>/dev/null || true
echo "✅ Guides and specs organized"
echo ""

echo "=========================================="
echo "✅ Reorganization Complete!"
echo "=========================================="
echo ""
echo "Summary:"
echo "  - Backup created: $BACKUP_DIR"
echo "  - New structure created in: $DOCS_DIR"
echo ""
echo "Next steps:"
echo "  1. Review the new structure"
echo "  2. Run: ./scripts/create_doc_navigation.sh"
echo "  3. Update ARCHITECTURE.md references"
echo ""
