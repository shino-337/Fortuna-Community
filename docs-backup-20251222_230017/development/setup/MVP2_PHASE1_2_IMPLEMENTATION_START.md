# MVP2 Phase 1.2: Risk Prioritization - Implementation Start

**Date**: 2025-12-11  
**Status**: 🚀 **IMPLEMENTATION STARTING**

---

## Executive Summary

Bắt đầu implement Phase 1.2: Risk Prioritization. Phase này sẽ thêm khả năng ưu tiên hóa risks dựa trên business impact và exploitability.

---

## Objectives

1. Implement prioritization logic
2. Add business impact scoring
3. Add exploitability scoring
4. Create priority-based API endpoints
5. Update dashboard views

---

## Current State Analysis

### Existing Risk Score Model

Cần kiểm tra:
- RiskScore model structure
- Existing risk scoring algorithm
- API endpoints for risk scores
- Database schema

### What Needs to Be Added

1. **Priority Field**: Add priority level (P0-P3) to RiskScore
2. **Business Impact Calculator**: Calculate business impact score
3. **Exploitability Calculator**: Calculate exploitability score
4. **Priority Calculator**: Combine scores into priority
5. **API Enhancements**: Add priority filtering and sorting
6. **Database Migration**: Add priority column if needed

---

## Implementation Plan

### Step 1: Database Schema Update
- Add `priority` field to `risk_scores` table
- Add `business_impact_score` field
- Add `exploitability_score` field
- Create migration

### Step 2: Model Updates
- Update `RiskScore` model
- Add priority calculation methods

### Step 3: Priority Calculation Logic
- Implement business impact calculator
- Implement exploitability calculator
- Implement priority assignment (P0-P3)

### Step 4: API Enhancements
- Add priority filter to risk scores API
- Add priority sorting
- Add priority-based endpoints

### Step 5: Testing
- Unit tests for priority calculation
- Integration tests for API
- End-to-end tests

---

## Next Actions

1. Analyze current RiskScore model
2. Design priority calculation algorithm
3. Create database migration
4. Implement priority calculation
5. Update API endpoints
6. Write tests

---

**Status**: 🚀 **READY TO START**


