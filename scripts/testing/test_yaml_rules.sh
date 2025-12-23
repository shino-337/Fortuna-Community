#!/bin/bash

# Script to test YAML rules functionality

set -e

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║           YAML RULES TESTING SCRIPT                           ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# Check if rules directory exists
RULES_DIR="${KSAM_RULES_DIR:-./core/rules}"
if [ ! -d "$RULES_DIR" ]; then
    echo "❌ Rules directory not found: $RULES_DIR"
    echo "   Set KSAM_RULES_DIR environment variable or ensure ./core/rules exists"
    exit 1
fi

echo "📁 Rules directory: $RULES_DIR"
echo ""

# Count YAML files
YAML_COUNT=$(find "$RULES_DIR" -name "*.yaml" -o -name "*.yml" | wc -l | tr -d ' ')
echo "📊 Found $YAML_COUNT YAML rule files"
echo ""

# List all rules
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📋 YAML Rules:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
for file in "$RULES_DIR"/*.yaml "$RULES_DIR"/*.yml; do
    if [ -f "$file" ]; then
        RULE_ID=$(grep -E "^id:" "$file" | head -1 | sed 's/id: *//' | tr -d ' "')
        RULE_NAME=$(grep -E "^name:" "$file" | head -1 | sed 's/name: *//' | tr -d '"')
        RULE_SEVERITY=$(grep -E "^severity:" "$file" | head -1 | sed 's/severity: *//' | tr -d ' "')
        echo "  • $RULE_ID ($RULE_SEVERITY): $RULE_NAME"
    fi
done
echo ""

# Validate YAML syntax
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Validating YAML syntax..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
ERRORS=0
for file in "$RULES_DIR"/*.yaml "$RULES_DIR"/*.yml; do
    if [ -f "$file" ]; then
        if ! python3 -c "import yaml; yaml.safe_load(open('$file'))" 2>/dev/null; then
            echo "❌ Invalid YAML: $(basename $file)"
            ERRORS=$((ERRORS + 1))
        else
            echo "  ✓ $(basename $file)"
        fi
    fi
done

if [ $ERRORS -gt 0 ]; then
    echo ""
    echo "❌ Found $ERRORS YAML syntax errors"
    exit 1
fi

echo ""
echo "✅ All YAML files are valid"
echo ""

# Check required fields
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🔍 Checking required fields..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
REQUIRED_FIELDS=("id" "name" "category" "severity" "description" "enabled" "conditions" "aggregation" "base_score")
MISSING=0

for file in "$RULES_DIR"/*.yaml "$RULES_DIR"/*.yml; do
    if [ -f "$file" ]; then
        FILENAME=$(basename "$file")
        MISSING_FIELDS=""
        for field in "${REQUIRED_FIELDS[@]}"; do
            if ! grep -qE "^${field}:" "$file"; then
                MISSING_FIELDS="$MISSING_FIELDS $field"
            fi
        done
        if [ -n "$MISSING_FIELDS" ]; then
            echo "❌ $FILENAME missing:$MISSING_FIELDS"
            MISSING=$((MISSING + 1))
        else
            echo "  ✓ $FILENAME"
        fi
    fi
done

if [ $MISSING -gt 0 ]; then
    echo ""
    echo "❌ Found $MISSING files with missing required fields"
    exit 1
fi

echo ""
echo "✅ All required fields present"
echo ""

# Summary
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "📊 Summary:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Total rules: $YAML_COUNT"
echo "  Valid YAML: ✅"
echo "  Required fields: ✅"
echo ""
echo "✅ YAML rules validation completed successfully"
echo ""
echo "💡 To use these rules, set environment variable:"
echo "   export KSAM_RULES_DIR=\"$RULES_DIR\""

