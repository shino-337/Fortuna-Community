#!/bin/bash
# Script to clean up repository - remove excluded files from git tracking

set -e

echo "🧹 Cleaning up repository..."

# Remove test results
echo "Removing test results..."
git rm --cached -r tests/results/ tests/e2e/results/ docs/test-results/*.md 2>/dev/null || true

# Remove debug scripts
echo "Removing debug scripts from tracking..."
git rm --cached scripts/fix-*.sh scripts/apply-*.sh scripts/clean-*.sh 2>/dev/null || true
git rm --cached scripts/quick-*.sh scripts/validate-*.sh 2>/dev/null || true
git rm --cached scripts/migrate-*.sh scripts/cleanup-*.sh 2>/dev/null || true
git rm --cached scripts/import-*.sh scripts/copy-*.sh 2>/dev/null || true
git rm --cached scripts/deploy-fortuna-robust.sh 2>/dev/null || true
git rm --cached scripts/load-cve-database.sh 2>/dev/null || true
git rm --cached scripts/build-and-import-containerd.sh 2>/dev/null || true
git rm --cached scripts/build-with-containerd.sh 2>/dev/null || true
git rm --cached scripts/apply-core-master-only.sh 2>/dev/null || true
git rm --cached scripts/apply-nats-single-replica.sh 2>/dev/null || true
git rm --cached scripts/fix_dns_isues.sh 2>/dev/null || true
git rm --cached scripts/fix_nats_storage.sh 2>/dev/null || true
git rm --cached scripts/create-mtls-secrets.sh 2>/dev/null || true

# Remove archive
echo "Removing archive directory..."
git rm --cached -r scripts/archive/ 2>/dev/null || true

# Remove temporary docs
echo "Removing temporary documentation..."
git rm --cached CLEANUP_SUMMARY.md HELM_CHART_UPDATE.md PROJECT_CLEANUP_ANALYSIS.md 2>/dev/null || true

# Remove docker-compose files
echo "Removing docker-compose files..."
git rm --cached docker-compose.prod.yml docker-compose.override.yml.example 2>/dev/null || true

echo "✅ Cleanup complete!"
echo ""
echo "Files are now untracked but remain on disk (local only)"
echo "Run 'git status' to see changes"
