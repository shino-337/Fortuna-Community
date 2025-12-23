#!/bin/bash

# Manually run Migration018
# This script connects to the database and runs Migration018 SQL directly

set -e

echo "=========================================="
echo "MANUAL MIGRATION 018 EXECUTION"
echo "=========================================="
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Get pod name
POD_NAME=$(kubectl get pod -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -z "$POD_NAME" ]; then
    echo -e "${RED}❌ Pod not found${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Pod: $POD_NAME${NC}"
echo ""

# Check if columns already exist
echo -e "${YELLOW}Step 1: Checking if columns already exist...${NC}"
EXISTS_CHECK=$(kubectl exec -n ksam $POD_NAME -- sh -c "PGPASSWORD=ksam psql -U ksam -d ksam -tAc \"SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'risk_scores' AND column_name IN ('exploitability_score', 'business_impact_score', 'scorer_version');\"" 2>&1)
if [ "$EXISTS_CHECK" = "3" ]; then
    echo -e "${GREEN}✅ Columns already exist${NC}"
    exit 0
elif [ "$EXISTS_CHECK" = "0" ]; then
    echo -e "${YELLOW}⚠️  Columns do not exist, proceeding with migration...${NC}"
else
    echo -e "${YELLOW}⚠️  Partial columns exist (count: $EXISTS_CHECK)${NC}"
fi
echo ""

# Run migration SQL
echo -e "${YELLOW}Step 2: Running Migration018 SQL...${NC}"
MIGRATION_SQL="
-- Migration 004: Add V2 scoring columns to risk_scores table
-- MVP2 Phase 1.2: Risk Scoring V2 Enhancement
-- Date: 2025-12-11

-- Add V2 scoring columns
ALTER TABLE risk_scores 
ADD COLUMN IF NOT EXISTS exploitability_score DECIMAL(5,2) DEFAULT 0.0,
ADD COLUMN IF NOT EXISTS business_impact_score DECIMAL(5,2) DEFAULT 0.0,
ADD COLUMN IF NOT EXISTS scorer_version VARCHAR(10) DEFAULT 'v1';

-- Add comments
COMMENT ON COLUMN risk_scores.exploitability_score IS 'Exploitability score (0-30) for V2 scoring';
COMMENT ON COLUMN risk_scores.business_impact_score IS 'Business impact score (0-30) for V2 scoring';
COMMENT ON COLUMN risk_scores.scorer_version IS 'Scorer version: v1 (old) or v2 (new)';
"

RESULT=$(kubectl exec -n ksam $POD_NAME -- sh -c "PGPASSWORD=ksam psql -U ksam -d ksam -c \"$MIGRATION_SQL\"" 2>&1)
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Migration018 executed successfully${NC}"
    echo "$RESULT"
else
    echo -e "${RED}❌ Migration018 failed${NC}"
    echo "$RESULT"
    exit 1
fi
echo ""

# Verify columns
echo -e "${YELLOW}Step 3: Verifying columns...${NC}"
VERIFY=$(kubectl exec -n ksam $POD_NAME -- sh -c "PGPASSWORD=ksam psql -U ksam -d ksam -tAc \"SELECT column_name FROM information_schema.columns WHERE table_name = 'risk_scores' AND column_name IN ('exploitability_score', 'business_impact_score', 'scorer_version') ORDER BY column_name;\"" 2>&1)
if echo "$VERIFY" | grep -q "exploitability_score"; then
    echo -e "${GREEN}✅ exploitability_score column exists${NC}"
else
    echo -e "${RED}❌ exploitability_score column NOT found${NC}"
fi
if echo "$VERIFY" | grep -q "business_impact_score"; then
    echo -e "${GREEN}✅ business_impact_score column exists${NC}"
else
    echo -e "${RED}❌ business_impact_score column NOT found${NC}"
fi
if echo "$VERIFY" | grep -q "scorer_version"; then
    echo -e "${GREEN}✅ scorer_version column exists${NC}"
else
    echo -e "${RED}❌ scorer_version column NOT found${NC}"
fi
echo ""

echo "=========================================="
echo -e "${GREEN}✅ MIGRATION 018 COMPLETE${NC}"
echo "=========================================="


