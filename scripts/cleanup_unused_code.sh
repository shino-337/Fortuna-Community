#!/bin/bash

# Script to clean up unused code and directories
# Run with caution - review changes before committing

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

echo "=========================================="
echo "🧹 KSAM Project Cleanup Script"
echo "=========================================="
echo ""

# Backup flag
BACKUP_DIR="backup_$(date +%Y%m%d_%H%M%S)"
CREATE_BACKUP=true

if [ "$1" == "--no-backup" ]; then
    CREATE_BACKUP=false
    echo "⚠️  Running without backup (--no-backup flag)"
else
    echo "📦 Creating backup in: $BACKUP_DIR"
    mkdir -p "$BACKUP_DIR"
fi

echo ""

# Function to safely remove with backup
safe_remove() {
    local target="$1"
    local description="$2"
    
    if [ ! -e "$target" ]; then
        echo "   ⚠️  $description not found, skipping"
        return
    fi
    
    if [ "$CREATE_BACKUP" = true ]; then
        echo "   📦 Backing up: $target"
        mkdir -p "$BACKUP_DIR/$(dirname "$target")"
        cp -r "$target" "$BACKUP_DIR/$target" 2>/dev/null || true
    fi
    
    echo "   🗑️  Removing: $target"
    rm -rf "$target"
}

# 1. Remove k8sfortuna directory
echo "1️⃣  Removing k8sfortuna directory (old project)..."
if [ -d "k8sfortuna" ]; then
    safe_remove "k8sfortuna" "k8sfortuna directory"
    echo "   ✅ Removed k8sfortuna directory"
else
    echo "   ℹ️  k8sfortuna directory not found"
fi
echo ""

# 2. Remove unused controller package (if not implementing)
echo "2️⃣  Checking controller package..."
if [ -d "KSAM/core/internal/controller" ]; then
    echo "   ⚠️  Controller package found with TODO stubs"
    echo "   💡 To remove: uncomment the following lines"
    echo "   # safe_remove 'KSAM/core/internal/controller' 'controller package'"
    # Uncomment to actually remove:
    # safe_remove "KSAM/core/internal/controller" "controller package"
else
    echo "   ℹ️  Controller package not found"
fi
echo ""

# 3. Remove unused policy package (if not implementing)
echo "3️⃣  Checking policy package..."
if [ -d "KSAM/core/internal/policy" ]; then
    echo "   ⚠️  Policy package found with TODO stubs"
    echo "   💡 To remove: uncomment the following lines"
    echo "   # safe_remove 'KSAM/core/internal/policy' 'policy package'"
    # Uncomment to actually remove:
    # safe_remove "KSAM/core/internal/policy" "policy package"
else
    echo "   ℹ️  Policy package not found"
fi
echo ""

# 4. Clean up test results (keep last 30 days)
echo "4️⃣  Cleaning up test results..."
if [ -d "KSAM/test_results" ]; then
    echo "   📊 Test results directory size: $(du -sh KSAM/test_results | cut -f1)"
    echo "   💡 To clean old results, run:"
    echo "   find KSAM/test_results -type f -mtime +30 -delete"
    # Uncomment to actually clean:
    # find KSAM/test_results -type f -mtime +30 -delete
    # echo "   ✅ Cleaned test results older than 30 days"
else
    echo "   ℹ️  Test results directory not found"
fi
echo ""

# 5. Summary
echo "=========================================="
echo "📊 Cleanup Summary"
echo "=========================================="
echo ""

if [ "$CREATE_BACKUP" = true ]; then
    echo "✅ Backup created in: $BACKUP_DIR"
    echo "   To restore: cp -r $BACKUP_DIR/* ."
    echo ""
fi

echo "📝 Next Steps:"
echo "   1. Review changes: git status"
echo "   2. Test the application: make test"
echo "   3. Commit changes: git add . && git commit -m 'Cleanup: Remove unused code'"
echo ""
echo "⚠️  Note: Controller and Policy packages are kept by default"
echo "   Remove them manually if not needed for Phase 3+"
echo ""
echo "=========================================="
echo "✅ Cleanup Complete"
echo "=========================================="

