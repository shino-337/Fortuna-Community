# Policy Engine Implementation Plan

**Date**: $(date)  
**Based on**: Policy Engine Analysis Report  
**Status**: Planning Phase

---

## Executive Summary

The policy engine analysis identified **7 critical issues** and **17 actionable items** across **7 priority sprints** (356 hours total effort). This document provides a concrete implementation plan to address all identified issues.

---

## Current State Analysis

### ✅ Strengths
1. **Performance**: Excellent design with in-memory caching (< 100ms target met)
2. **Scalability**: CEL-based evaluation supports complex policies
3. **Safety**: Production-ready race condition prevention
4. **Architecture**: Clean separation of concerns
5. **Flexibility**: Template-instance pattern allows reuse

### ❌ Critical Issues

#### 1. CEL Logic Negation (Line 353 in evaluator.go)
- **Issue**: Confusing negation logic that may cause incorrect evaluations
- **Priority**: High
- **Impact**: Potential false positives/negatives

#### 2. Violation Sampling (50% for medium+warn)
- **Issue**: May hide critical issues by sampling violations
- **Priority**: High
- **Impact**: Missing important policy violations

#### 3. No CEL Validation
- **Issue**: Templates saved without validating CEL compiles
- **Priority**: Critical
- **Impact**: Runtime errors, poor user experience

#### 4. Missing Integrations
- **Issue**: No connection to risk scoring, insights, compliance
- **Priority**: High
- **Impact**: Isolated system, limited value

#### 5. No Default Templates
- **Issue**: System ships empty (no security/compliance policies)
- **Priority**: Medium
- **Impact**: Poor out-of-box experience

#### 6. Missing RBAC
- **Issue**: No role-based access control for policy management
- **Priority**: Medium
- **Impact**: Security risk, no access control

#### 7. Limited Audit Trail
- **Issue**: Policy changes not comprehensively logged
- **Priority**: Medium
- **Impact**: Compliance issues, difficult troubleshooting

---

## Implementation Plan

### Sprint 1: Critical Fixes (48 hours)
**Goal**: Fix immediate issues preventing proper operation

#### Task 1.1: CEL Validation (16 hours)
- **Files**: `core/pkg/policy/template.go`, `core/internal/api/policy_handlers.go`
- **Actions**:
  1. Add CEL compilation validation in `CreateTemplate` and `UpdateTemplate`
  2. Add validation endpoint for testing CEL expressions
  3. Return clear error messages for invalid CEL
  4. Add unit tests for CEL validation

#### Task 1.2: Fix CEL Logic Negation (8 hours)
- **Files**: `core/pkg/policy/evaluator.go` (line 353)
- **Actions**:
  1. Review and document negation logic
  2. Add comments explaining the logic
  3. Add test cases to verify correctness
  4. Refactor if needed for clarity

#### Task 1.3: Fix Violation Sampling (12 hours)
- **Files**: `core/pkg/policy/evaluator.go`, `core/pkg/policy/violation.go`
- **Actions**:
  1. Review sampling logic (50% for medium+warn)
  2. Make sampling configurable per policy
  3. Add option to disable sampling for critical policies
  4. Add metrics for sampled vs. total violations

#### Task 1.4: Create Default Templates (12 hours)
- **Files**: `core/migrations/040_add_default_policy_templates.go`
- **Actions**:
  1. Create migration with default security templates
  2. Create default compliance templates
  3. Include templates for:
     - Pod security (no privileged containers)
     - Resource limits
     - Image security (no latest tags)
     - Network policies
  4. Mark templates as system-managed

---

### Sprint 2: Risk Scoring Integration (56 hours)
**Goal**: Connect policy engine to risk scoring system

#### Task 2.1: Policy-Risk Integration (24 hours)
- **Files**: `core/pkg/policy/evaluator.go`, `core/pkg/riskengine/engine.go`
- **Actions**:
  1. Add risk score calculation based on policy violations
  2. Update risk scores when violations occur/resolve
  3. Add policy violation severity to risk calculation
  4. Create risk-policy correlation metrics

#### Task 2.2: Risk-Based Policy Recommendations (16 hours)
- **Files**: `core/pkg/policy/recommender.go` (new)
- **Actions**:
  1. Analyze risk scores to recommend policies
  2. Suggest policy templates based on risk patterns
  3. Auto-enable policies for high-risk resources

#### Task 2.3: Policy Violation Impact on Risk (16 hours)
- **Files**: `core/pkg/riskengine/engine.go`
- **Actions**:
  1. Weight policy violations in risk calculation
  2. Different weights for different violation severities
  3. Time-decay for resolved violations
  4. Dashboard showing policy-risk correlation

---

### Sprint 3: Insights Integration (48 hours)
**Goal**: Connect policy engine to insights system

#### Task 3.1: Policy Violation Insights (20 hours)
- **Files**: `core/pkg/policy/evaluator.go`, `core/pkg/riskengine/insight_manager.go`
- **Actions**:
  1. Create insights from policy violations
  2. Link violations to resources via insights
  3. Add insight types: POLICY_VIOLATION, COMPLIANCE_ISSUE
  4. Aggregate multiple violations into single insight

#### Task 3.2: Policy Compliance Dashboard (16 hours)
- **Files**: `core/internal/api/compliance_handlers.go` (new)
- **Actions**:
  1. Create compliance endpoint showing policy status
  2. Aggregate violations by policy, resource, namespace
  3. Compliance score calculation
  4. Historical compliance trends

#### Task 3.3: Policy-Insight Correlation (12 hours)
- **Files**: `core/pkg/policy/evaluator.go`
- **Actions**:
  1. Correlate policy violations with CVE insights
  2. Show which policies would have caught vulnerabilities
  3. Recommend policies based on insight patterns
  4. Auto-create policies from recurring insights

---

### Sprint 4: RBAC Implementation (40 hours)
**Goal**: Add role-based access control

#### Task 4.1: Policy RBAC Model (16 hours)
- **Files**: `core/pkg/policy/rbac.go` (new), `core/migrations/041_add_policy_rbac.sql`
- **Actions**:
  1. Define RBAC roles: admin, editor, viewer
  2. Create policy permissions table
  3. Add role assignments
  4. Migration to create RBAC tables

#### Task 4.2: RBAC Enforcement (16 hours)
- **Files**: `core/internal/api/policy_handlers.go`, `core/pkg/policy/rbac.go`
- **Actions**:
  1. Add RBAC checks to all policy endpoints
  2. Filter policies based on user permissions
  3. Add audit logging for permission denials
  4. Middleware for RBAC enforcement

#### Task 4.3: RBAC API (8 hours)
- **Files**: `core/internal/api/rbac_handlers.go` (new)
- **Actions**:
  1. Endpoints for role management
  2. Permission assignment endpoints
  3. User-role mapping endpoints
  4. Documentation

---

### Sprint 5: Testing Framework (48 hours)
**Goal**: Comprehensive testing infrastructure

#### Task 5.1: Unit Tests (20 hours)
- **Files**: `core/pkg/policy/*_test.go`
- **Actions**:
  1. Test CEL evaluation logic
  2. Test violation creation
  3. Test sampling logic
  4. Test RBAC enforcement
  5. Achieve >80% code coverage

#### Task 5.2: Integration Tests (16 hours)
- **Files**: `tests/integration/policy/`
- **Actions**:
  1. End-to-end policy evaluation tests
  2. Policy-template-instance flow tests
  3. Risk-policy integration tests
  4. Insights-policy integration tests

#### Task 5.3: Performance Tests (12 hours)
- **Files**: `tests/performance/policy/`
- **Actions**:
  1. Load testing for policy evaluation
  2. Concurrent evaluation tests
  3. Cache performance tests
  4. Memory usage profiling

---

### Sprint 6: Observability (48 hours)
**Goal**: Enhanced monitoring and debugging

#### Task 6.1: Policy Metrics (16 hours)
- **Files**: `core/pkg/policy/metrics.go` (new)
- **Actions**:
  1. Prometheus metrics for:
     - Policy evaluation count
     - Violation count by severity
     - Evaluation latency
     - Cache hit rate
  2. Export metrics endpoint

#### Task 6.2: Enhanced Logging (16 hours)
- **Files**: `core/pkg/policy/evaluator.go`
- **Actions**:
  1. Structured logging for policy evaluations
  2. Log policy decisions with context
  3. Log violation creation with details
  4. Configurable log levels

#### Task 6.3: Policy Debugging Tools (16 hours)
- **Files**: `core/internal/api/policy_debug_handlers.go` (new)
- **Actions**:
  1. Policy evaluation trace endpoint
  2. CEL expression testing endpoint
  3. Policy impact analysis endpoint
  4. Debug dashboard

---

### Sprint 7: Compliance & Documentation (68 hours)
**Goal**: Production readiness

#### Task 7.1: Audit Trail Enhancement (20 hours)
- **Files**: `core/pkg/policy/audit.go` (new), `core/migrations/042_add_policy_audit.sql`
- **Actions**:
  1. Comprehensive audit logging for:
     - Template creation/update/deletion
     - Instance creation/update/deletion
     - Policy evaluation results
     - Violation creation/resolution
  2. Audit log table
  3. Audit log query API

#### Task 7.2: Compliance Reporting (24 hours)
- **Files**: `core/internal/api/compliance_handlers.go`
- **Actions**:
  1. Compliance report generation
  2. Export to CSV/JSON
  3. Scheduled compliance reports
  4. Compliance score calculation

#### Task 7.3: Documentation (24 hours)
- **Files**: `docs/policy-engine/`
- **Actions**:
  1. Policy engine architecture documentation
  2. CEL expression guide
  3. Policy template creation guide
  4. API documentation
  5. Troubleshooting guide
  6. Best practices

---

## Implementation Priority

### Phase 1: Critical (Sprint 1) - Week 1-2
- CEL Validation
- CEL Logic Fix
- Violation Sampling Fix
- Default Templates

### Phase 2: Integration (Sprint 2-3) - Week 3-5
- Risk Scoring Integration
- Insights Integration

### Phase 3: Security & Quality (Sprint 4-5) - Week 6-7
- RBAC Implementation
- Testing Framework

### Phase 4: Production Ready (Sprint 6-7) - Week 8-9
- Observability
- Compliance & Documentation

---

## Success Criteria

### Sprint 1
- ✅ All CEL expressions validated before saving
- ✅ CEL logic clearly documented and tested
- ✅ Violation sampling configurable and transparent
- ✅ Default templates available on fresh install

### Sprint 2-3
- ✅ Policy violations affect risk scores
- ✅ Insights generated from policy violations
- ✅ Compliance dashboard functional

### Sprint 4-5
- ✅ RBAC enforced on all policy operations
- ✅ >80% test coverage
- ✅ Performance targets met

### Sprint 6-7
- ✅ Comprehensive metrics and logging
- ✅ Full audit trail
- ✅ Complete documentation

---

## Risk Mitigation

### Technical Risks
1. **CEL Performance**: Monitor evaluation latency, optimize if needed
2. **Database Load**: Index policy tables properly, consider caching
3. **Integration Complexity**: Start with simple integrations, iterate

### Process Risks
1. **Scope Creep**: Stick to sprint plan, defer non-critical items
2. **Testing Gaps**: Allocate sufficient time for testing
3. **Documentation**: Document as you go, not at the end

---

## Next Steps

1. **Review this plan** with stakeholders
2. **Prioritize sprints** based on business needs
3. **Assign resources** to each sprint
4. **Start Sprint 1** with CEL validation

---

**Plan Created**: $(date)  
**Estimated Total Effort**: 356 hours (7 weeks)  
**Status**: Ready for Review

