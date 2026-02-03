#!/bin/bash

# ============================================================================
# Sync Git Repository with Local State
# ============================================================================
# Removes deleted files from git and syncs with local state
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "=========================================="
echo "Sync Git Repository with Local State"
echo "=========================================="
echo ""

# Check if in git repo
if ! git rev-parse --git-dir >/dev/null 2>&1; then
    echo -e "${RED}❌${NC} Not a git repository"
    exit 1
fi

# Check current branch
CURRENT_BRANCH=$(git branch --show-current 2>/dev/null || echo "unknown")
echo -e "${BLUE}Current branch:${NC} $CURRENT_BRANCH"
echo ""

# Step 1: Find deleted files
echo -e "${BLUE}Step 1: Finding deleted files...${NC}"
DELETED_FILES=$(git ls-files --deleted 2>/dev/null || echo "")

if [ -z "$DELETED_FILES" ]; then
    echo -e "${GREEN}✅${NC} No deleted files found in git"
else
    echo -e "${YELLOW}Found deleted files:${NC}"
    echo "$DELETED_FILES" | while read -r file; do
        if [ -n "$file" ]; then
            echo "  - $file"
        fi
    done
    echo ""
fi

# Step 2: Find untracked files that should be ignored
echo -e "${BLUE}Step 2: Checking for files that should be ignored...${NC}"
if [ -f ".gitignore" ]; then
    IGNORED_FILES=$(git status --porcelain --ignored 2>/dev/null | grep "^!!" | awk '{print $2}' || echo "")
    if [ -n "$IGNORED_FILES" ]; then
        echo -e "${YELLOW}Files that should be ignored:${NC}"
        echo "$IGNORED_FILES" | while read -r file; do
            if [ -n "$file" ]; then
                echo "  - $file"
            fi
        done
    else
        echo -e "${GREEN}✅${NC} No ignored files to clean"
    fi
else
    echo -e "${YELLOW}⚠️${NC}  .gitignore not found"
fi
echo ""

# Step 3: Remove deleted files from git
if [ -n "$DELETED_FILES" ]; then
    echo -e "${BLUE}Step 3: Removing deleted files from git...${NC}"
    echo ""
    echo -e "${YELLOW}Files to be removed from git:${NC}"
    echo "$DELETED_FILES" | while read -r file; do
        if [ -n "$file" ]; then
            echo "  - $file"
        fi
    done
    echo ""
    
    read -p "Remove these files from git? (y/N): " confirm
    if [ "$confirm" = "y" ] || [ "$confirm" = "Y" ]; then
        echo "$DELETED_FILES" | while read -r file; do
            if [ -n "$file" ]; then
                git rm "$file" 2>/dev/null || true
            fi
        done
        echo -e "${GREEN}✅${NC} Deleted files removed from git"
    else
        echo -e "${YELLOW}⚠️${NC}  Skipped removing deleted files"
    fi
    echo ""
fi

# Step 4: Add .gitignore if it exists
if [ -f ".gitignore" ]; then
    echo -e "${BLUE}Step 4: Adding .gitignore...${NC}"
    if git add .gitignore; then
        echo -e "${GREEN}✅${NC} .gitignore added"
    else
        echo -e "${YELLOW}⚠️${NC}  .gitignore already tracked or no changes"
    fi
    echo ""
fi

# Step 5: Remove tracked files that should be ignored
echo -e "${BLUE}Step 5: Checking for tracked files that should be ignored...${NC}"
TRACKED_TO_IGNORE=""

# Check .cursorignore
if git ls-files --error-unmatch .cursorignore >/dev/null 2>&1; then
    if grep -q "^\.cursorignore$" .gitignore 2>/dev/null; then
        TRACKED_TO_IGNORE="$TRACKED_TO_IGNORE .cursorignore"
    fi
fi

# Check .claude/
if git ls-files --error-unmatch .claude/ >/dev/null 2>&1 || git ls-files | grep -q "^\.claude/"; then
    if grep -q "^\.claude/" .gitignore 2>/dev/null; then
        TRACKED_TO_IGNORE="$TRACKED_TO_IGNORE .claude/"
    fi
fi

if [ -n "$TRACKED_TO_IGNORE" ]; then
    echo -e "${YELLOW}Found tracked files that should be ignored:${NC}"
    for file in $TRACKED_TO_IGNORE; do
        echo "  - $file"
    done
    echo ""
    
    read -p "Remove these files from git tracking? (y/N): " confirm
    if [ "$confirm" = "y" ] || [ "$confirm" = "Y" ]; then
        for file in $TRACKED_TO_IGNORE; do
            if [ -d "$file" ]; then
                git rm -r --cached "$file" 2>/dev/null || true
            else
                git rm --cached "$file" 2>/dev/null || true
            fi
            echo -e "${GREEN}✅${NC} Removed $file from git tracking"
        done
    else
        echo -e "${YELLOW}⚠️${NC}  Skipped removing tracked files"
    fi
    echo ""
else
    echo -e "${GREEN}✅${NC} No tracked files need to be ignored"
    echo ""
fi

# Step 6: Show status
echo -e "${BLUE}Step 6: Current git status...${NC}"
git status --short
echo ""

# Step 7: Summary and next steps
echo "=========================================="
echo "Summary"
echo "=========================================="
echo ""
echo "Changes made:"
echo "  - Deleted files removed from git"
echo "  - Tracked files removed from git (kept locally)"
echo "  - .gitignore updated"
echo ""
echo "Next steps:"
echo ""
echo "1. Review changes:"
echo "   git status"
echo ""
echo "2. Stage all changes:"
echo "   git add -A"
echo ""
echo "3. Commit changes:"
echo "   git commit -m \"chore: sync with local state, remove deleted files\""
echo ""
echo "4. Push to remote:"
echo "   git push origin $CURRENT_BRANCH"
echo ""
echo "⚠️  WARNING: This will delete files from remote repository!"
echo "   Make sure you have backups if needed."
echo ""


