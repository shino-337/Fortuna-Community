#!/bin/bash

# ============================================================================
# Remove Tracked Files That Should Be Ignored
# ============================================================================
# Removes files from git tracking that are in .gitignore
# ============================================================================

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "=========================================="
echo "Remove Tracked Files That Should Be Ignored"
echo "=========================================="
echo ""

# Check if in git repo
if ! git rev-parse --git-dir >/dev/null 2>&1; then
    echo -e "${RED}❌${NC} Not a git repository"
    exit 1
fi

# Check .gitignore exists
if [ ! -f ".gitignore" ]; then
    echo -e "${RED}❌${NC} .gitignore not found"
    exit 1
fi

# Files to check
FILES_TO_CHECK=(
    ".cursorignore"
    ".claude/"
    ".vscode/"
    ".idea/"
)

echo -e "${BLUE}Checking for tracked files that should be ignored...${NC}"
echo ""

FOUND_FILES=()

for file in "${FILES_TO_CHECK[@]}"; do
    if git ls-files --error-unmatch "$file" >/dev/null 2>&1 || git ls-files | grep -q "^${file}"; then
        if grep -q "^${file}" .gitignore 2>/dev/null || grep -q "^${file%/}" .gitignore 2>/dev/null; then
            FOUND_FILES+=("$file")
            echo -e "${YELLOW}Found:${NC} $file (tracked but should be ignored)"
        fi
    fi
done

if [ ${#FOUND_FILES[@]} -eq 0 ]; then
    echo -e "${GREEN}✅${NC} No tracked files need to be removed"
    exit 0
fi

echo ""
read -p "Remove these files from git tracking? (y/N): " confirm
if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
    echo "Aborted."
    exit 0
fi

echo ""
echo -e "${BLUE}Removing from git tracking...${NC}"

for file in "${FOUND_FILES[@]}"; do
    if [ -d "$file" ]; then
        git rm -r --cached "$file" 2>/dev/null && echo -e "${GREEN}✅${NC} Removed $file/" || echo -e "${YELLOW}⚠️${NC}  Could not remove $file/"
    else
        git rm --cached "$file" 2>/dev/null && echo -e "${GREEN}✅${NC} Removed $file" || echo -e "${YELLOW}⚠️${NC}  Could not remove $file"
    fi
done

echo ""
echo -e "${GREEN}✅${NC} Files removed from git tracking (kept locally)"
echo ""
echo "Next steps:"
echo "  git commit -m \"chore: remove tracked files that should be ignored\""
echo "  git push origin \$(git branch --show-current)"
echo ""


