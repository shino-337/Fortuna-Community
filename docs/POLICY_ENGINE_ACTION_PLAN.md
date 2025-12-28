# Policy Engine Action Plan - Detailed Implementation

**Date**: $(date)  
**Based on**: Policy Engine Analysis Report (67 pages)  
**Status**: Ready for Implementation

---

## Executive Summary

The policy engine analysis identified **7 critical issues** requiring immediate attention. This action plan provides a **concrete, prioritized roadmap** to address all issues over **7 sprints (356 hours)**.

**Current Grade**: A- (Excellent with Room for Enhancement)  
**Target Grade**: A+ (Production-Ready Excellence)

---

## Critical Issues Summary

| # | Issue | Priority | Impact | Effort |
|---|-------|----------|--------|--------|
| 1 | CEL Logic Negation (Line 353) | High | Medium | 8h |
| 2 | Violation Sampling (50% medium+warn) | High | High | 12h |
| 3 | No CEL Validation | **Critical** | **High** | 16h |
| 4 | Missing Integrations | High | High | 104h |
| 5 | No Default Templates | Medium | Medium | 12h |
| 6 | Missing RBAC | Medium | Medium | 40h |
| 7 | Limited Audit Trail | Medium | Low | 20h |

---

## Sprint 1: Critical Fixes (48 hours) - Week 1-2

### 🎯 Goal: Fix immediate issues preventing proper operation

---

### Task 1.1: CEL Validation (16 hours) - **CRITICAL**

**Problem**: Templates saved without validating CEL compiles → runtime errors

**Files to Modify**:
- `core/internal/api/policy/policy_handlers.go` (CreateTemplate, UpdateTemplate)
- `core/pkg/policy/validator.go` (NEW)

**Implementation Steps**:

1. **Create CEL Validator** (4 hours)
   ```go
   // core/pkg/policy/validator.go
   package policy
   
   type CELValidator struct {
       celEnv *cel.Env
   }
   
   func (v *CELValidator) ValidateExpression(expr string) error {
       // Compile CEL expression
       // Return clear error messages
   }
   ```

2. **Add Validation to CreateTemplate** (4 hours)
   - Validate CEL before saving
   - Return HTTP 400 with clear error message
   - Test with invalid CEL expressions

3. **Add Validation to UpdateTemplate** (4 hours)
   - Same validation logic
   - Prevent updates that break existing instances

4. **Add Validation Endpoint** (2 hours)
   - `POST /api/v1/policies/templates/validate`
   - Allow testing CEL without saving
   - Useful for UI/CLI tools

5. **Unit Tests** (2 hours)
   - Test valid CEL expressions
   - Test invalid CEL expressions
   - Test edge cases

**Success Criteria**:
- ✅ No template saved with invalid CEL
- ✅ Clear error messages for users
- ✅ Validation endpoint functional

---

### Task 1.2: Fix CEL Logic Negation (8 hours)

**Problem**: Line 353 logic is confusing (but correct) - needs documentation

**Files to Modify**:
- `core/pkg/policy/evaluator.go` (line 353)

**Current Code**:
```go
// Line 353: return !result, nil
```

**Analysis**: Logic is CORRECT but confusing:
- CEL returns `true` for compliant resources
- Negation converts to violation detection
- Need better documentation

**Implementation Steps**:

1. **Add Comprehensive Comments** (2 hours)
   - Document the logic clearly
   - Add examples
   - Explain why negation is needed

2. **Add Test Cases** (4 hours)
   - Test compliant resources (CEL returns true → no violation)
   - Test non-compliant resources (CEL returns false → violation)
   - Test edge cases

3. **Consider Refactoring** (2 hours)
   - Evaluate if clearer naming helps
   - Consider helper function with clear name
   - Keep logic if tests pass

**Success Criteria**:
- ✅ Logic clearly documented
- ✅ All test cases pass
- ✅ No regression in functionality

---

### Task 1.3: Fix Violation Sampling (12 hours)

**Problem**: 50% sampling for medium+warn may hide critical issues

**Files to Modify**:
- `core/pkg/policy/violation.go` (shouldRecordViolation)
- `core/pkg/models/policy_instance.go` (add sampling config)

**Current Behavior**:
- Low + audit: 10% sampling
- Medium + warn: 50% sampling
- High/Critical: 100% (always record)

**Issues**:
1. Sampling is hardcoded
2. No way to disable sampling per policy
3. May hide important violations

**Implementation Steps**:

1. **Add Sampling Configuration** (4 hours)
   ```go
   // Add to PolicyInstance model
   type PolicyInstance struct {
       // ... existing fields
       SamplingEnabled *bool   `json:"samplingEnabled"` // nil = use default
       SamplingRate    float64 `json:"samplingRate"`    // 0.0-1.0, 1.0 = 100%
   }
   ```

2. **Make Sampling Configurable** (4 hours)
   - Allow per-instance sampling config
   - Default to current behavior
   - Add option to disable sampling

3. **Add Metrics** (2 hours)
   - Track sampled vs. total violations
   - Prometheus metrics for monitoring
   - Dashboard showing sampling impact

4. **Update Documentation** (2 hours)
   - Document sampling behavior
   - Explain when to use sampling
   - Best practices guide

**Success Criteria**:
- ✅ Sampling configurable per policy
- ✅ Metrics available for monitoring
- ✅ Documentation updated

---

### Task 1.4: Create Default Templates (12 hours)

**Problem**: System ships empty - no security/compliance policies

**Files to Create**:
- `core/migrations/040_add_default_policy_templates.go`

**Default Templates to Include**:

1. **Pod Security** (4 templates)
   - No privileged containers
   - Run as non-root
   - Read-only root filesystem
   - Drop all capabilities

2. **Resource Limits** (2 templates)
   - CPU limits required
   - Memory limits required

3. **Image Security** (2 templates)
   - No latest tags
   - Image digest required

4. **Network Policies** (2 templates)
   - Network policy required
   - No host network

**Implementation Steps**:

1. **Create Migration** (6 hours)
   - SQL migration with INSERT statements
   - Mark templates as system-managed
   - Add proper CEL expressions

2. **Test Templates** (4 hours)
   - Verify CEL expressions compile
   - Test with sample resources
   - Ensure templates work correctly

3. **Documentation** (2 hours)
   - Document each template
   - Explain when to use
   - Provide examples

**Success Criteria**:
- ✅ 10+ default templates available
- ✅ All templates tested
- ✅ Templates marked as system-managed

---

## Sprint 2: Risk Scoring Integration (56 hours) - Week 3-4

### 🎯 Goal: Connect policy engine to risk scoring system

---

### Task 2.1: Policy-Risk Integration (24 hours)

**Files to Modify**:
- `core/pkg/policy/evaluator.go`
- `core/pkg/riskengine/engine.go`
- `core/pkg/models/risk_score.go`

**Implementation Steps**:

1. **Add Violation Weight to Risk Calculation** (8 hours)
   ```go
   // In riskengine/engine.go
   func (e *Engine) CalculateRiskScore(resource *Resource) {
       // Get policy violations
       violations := e.policyEvaluator.EvaluateFast(ctx, resource)
       
       // Add violation weight
       riskScore += violationWeight * len(violations)
   }
   ```

2. **Different Weights by Severity** (8 hours)
   - Critical: 0.3 weight
   - High: 0.2 weight
   - Medium: 0.1 weight
   - Low: 0.05 weight

3. **Time-Decay for Resolved Violations** (4 hours)
   - Resolved violations decay over time
   - Full weight for 24h, then decay

4. **Metrics and Dashboard** (4 hours)
   - Track policy-risk correlation
   - Dashboard showing impact

**Success Criteria**:
- ✅ Policy violations affect risk scores
- ✅ Different weights by severity
- ✅ Time-decay working

---

### Task 2.2: Risk-Based Policy Recommendations (16 hours)

**Files to Create**:
- `core/pkg/policy/recommender.go` (NEW)

**Implementation Steps**:

1. **Analyze Risk Patterns** (6 hours)
   - Identify high-risk resources
   - Find common risk factors
   - Correlate with policy violations

2. **Recommend Policies** (6 hours)
   - Suggest templates based on risk
   - Auto-enable policies for high-risk
   - Provide reasoning

3. **API Endpoint** (4 hours)
   - `GET /api/v1/policies/recommendations?resource_uid=...`
   - Return recommended policies
   - Include confidence scores

**Success Criteria**:
- ✅ Recommendations generated
- ✅ API endpoint functional
- ✅ Recommendations are relevant

---

### Task 2.3: Policy Violation Impact on Risk (16 hours)

**Files to Modify**:
- `core/pkg/riskengine/engine.go`

**Implementation Steps**:

1. **Weight Violations in Risk** (8 hours)
   - Integrate violations into risk calculation
   - Different weights by severity
   - Configurable weights

2. **Dashboard Integration** (4 hours)
   - Show policy violations in risk dashboard
   - Visualize correlation
   - Historical trends

3. **Alerts** (4 hours)
   - Alert when violations increase risk
   - Threshold-based alerts
   - Integration with notification system

**Success Criteria**:
- ✅ Violations affect risk scores
- ✅ Dashboard shows correlation
- ✅ Alerts working

---

## Sprint 3: Insights Integration (48 hours) - Week 5

### 🎯 Goal: Connect policy engine to insights system

---

### Task 3.1: Policy Violation Insights (20 hours)

**Files to Modify**:
- `core/pkg/policy/evaluator.go`
- `core/pkg/riskengine/insight_manager.go`

**Implementation Steps**:

1. **Create Insights from Violations** (8 hours)
   ```go
   // In evaluator.go
   func (e *Evaluator) EvaluateFast(...) {
       // ... existing evaluation
       
       // Create insights
       for _, violation := range violations {
           insight := &models.Insight{
               InsightType: "POLICY_VIOLATION",
               Severity: violation.Severity,
               ResourceUID: violation.ResourceUID,
               // ...
           }
           e.insightMgr.CreateOrUpdateInsight(insight)
       }
   }
   ```

2. **Link Violations to Resources** (4 hours)
   - Use resource UID
   - Link to existing insights
   - Aggregate multiple violations

3. **Insight Types** (4 hours)
   - POLICY_VIOLATION
   - COMPLIANCE_ISSUE
   - SECURITY_POLICY_VIOLATION

4. **Testing** (4 hours)
   - Test insight creation
   - Test aggregation
   - Test linking

**Success Criteria**:
- ✅ Insights created from violations
- ✅ Linked to resources
- ✅ Multiple types supported

---

### Task 3.2: Policy Compliance Dashboard (16 hours)

**Files to Create**:
- `core/internal/api/compliance_handlers.go` (NEW)

**Implementation Steps**:

1. **Compliance Endpoint** (6 hours)
   - `GET /api/v1/compliance`
   - Aggregate violations by policy/resource/namespace
   - Calculate compliance scores

2. **Compliance Score Calculation** (6 hours)
   - Score = (total - violations) / total * 100
   - Per-policy scores
   - Overall score

3. **Historical Trends** (4 hours)
   - Track compliance over time
   - Show trends
   - Export data

**Success Criteria**:
- ✅ Compliance endpoint functional
- ✅ Scores calculated correctly
- ✅ Historical data available

---

### Task 3.3: Policy-Insight Correlation (12 hours)

**Files to Modify**:
- `core/pkg/policy/evaluator.go`

**Implementation Steps**:

1. **Correlate with CVE Insights** (4 hours)
   - Find resources with both CVEs and violations
   - Show correlation
   - Identify patterns

2. **Recommend Policies from Insights** (4 hours)
   - Analyze recurring insights
   - Suggest policies to prevent
   - Auto-create policies

3. **Dashboard** (4 hours)
   - Show correlation
   - Visualize relationships
   - Export data

**Success Criteria**:
- ✅ Correlation identified
   - ✅ Recommendations generated
   - ✅ Dashboard functional

---

## Sprint 4: RBAC Implementation (40 hours) - Week 6

### 🎯 Goal: Add role-based access control

---

### Task 4.1: Policy RBAC Model (16 hours)

**Files to Create**:
- `core/pkg/policy/rbac.go` (NEW)
- `core/migrations/041_add_policy_rbac.sql` (NEW)

**Implementation Steps**:

1. **Define RBAC Roles** (4 hours)
   ```go
   type PolicyRole string
   const (
       PolicyAdmin  PolicyRole = "admin"  // Full access
       PolicyEditor PolicyRole = "editor" // Create/update
       PolicyViewer PolicyRole = "viewer" // Read-only
   )
   ```

2. **Create RBAC Tables** (6 hours)
   - `policy_permissions` table
   - `policy_role_assignments` table
   - Foreign keys

3. **Migration** (4 hours)
   - Create tables
   - Add default roles
   - Seed data

4. **Model Definitions** (2 hours)
   - GORM models
   - Relationships
   - Validation

**Success Criteria**:
- ✅ RBAC tables created
- ✅ Roles defined
- ✅ Models working

---

### Task 4.2: RBAC Enforcement (16 hours)

**Files to Modify**:
- `core/internal/api/policy/policy_handlers.go`
- `core/pkg/policy/rbac.go`

**Implementation Steps**:

1. **Add RBAC Checks** (8 hours)
   - Check permissions before operations
   - Filter policies based on permissions
   - Return 403 for denied access

2. **Middleware** (4 hours)
   - RBAC middleware
   - Extract user from context
   - Check permissions

3. **Audit Logging** (4 hours)
   - Log permission denials
   - Log policy changes
   - Track access

**Success Criteria**:
- ✅ RBAC enforced
- ✅ Permissions checked
- ✅ Audit logging working

---

### Task 4.3: RBAC API (8 hours)

**Files to Create**:
- `core/internal/api/rbac_handlers.go` (NEW)

**Implementation Steps**:

1. **Role Management** (3 hours)
   - `POST /api/v1/rbac/roles`
   - `GET /api/v1/rbac/roles`
   - `PUT /api/v1/rbac/roles/:id`

2. **Permission Assignment** (3 hours)
   - `POST /api/v1/rbac/assignments`
   - `GET /api/v1/rbac/assignments`
   - `DELETE /api/v1/rbac/assignments/:id`

3. **User-Role Mapping** (2 hours)
   - `GET /api/v1/rbac/users/:id/roles`
   - Assign roles to users
   - List user permissions

**Success Criteria**:
- ✅ All endpoints functional
- ✅ Documentation complete
- ✅ Tests passing

---

## Sprint 5: Testing Framework (48 hours) - Week 7

### 🎯 Goal: Comprehensive testing infrastructure

---

### Task 5.1: Unit Tests (20 hours)

**Files to Create/Modify**:
- `core/pkg/policy/*_test.go`

**Coverage Targets**:
- CEL evaluation: 90%
- Violation creation: 85%
- Sampling logic: 90%
- RBAC enforcement: 85%

**Implementation Steps**:

1. **CEL Evaluation Tests** (6 hours)
   - Valid expressions
   - Invalid expressions
   - Edge cases

2. **Violation Tests** (4 hours)
   - Creation
   - Sampling
   - Status updates

3. **RBAC Tests** (4 hours)
   - Permission checks
   - Role assignments
   - Access control

4. **Integration Tests** (6 hours)
   - End-to-end flows
   - Policy-template-instance
   - Risk-policy integration

**Success Criteria**:
- ✅ >80% code coverage
- ✅ All critical paths tested
- ✅ Tests passing

---

### Task 5.2: Integration Tests (16 hours)

**Files to Create**:
- `tests/integration/policy/`

**Test Scenarios**:

1. **Policy Evaluation Flow** (4 hours)
   - Create template
   - Create instance
   - Evaluate resource
   - Verify violation

2. **Risk Integration** (4 hours)
   - Create violation
   - Verify risk score updated
   - Check correlation

3. **Insights Integration** (4 hours)
   - Create violation
   - Verify insight created
   - Check linking

4. **RBAC Flow** (4 hours)
   - Assign role
   - Test access
   - Verify permissions

**Success Criteria**:
- ✅ All flows tested
- ✅ Tests passing
- ✅ Documentation complete

---

### Task 5.3: Performance Tests (12 hours)

**Files to Create**:
- `tests/performance/policy/`

**Test Scenarios**:

1. **Load Testing** (4 hours)
   - 1000 concurrent evaluations
   - Measure latency
   - Check memory usage

2. **Cache Performance** (4 hours)
   - Cache hit rate
   - Cache invalidation
   - Memory usage

3. **Database Performance** (4 hours)
   - Violation recording
   - Query performance
   - Index usage

**Success Criteria**:
- ✅ Performance targets met
- ✅ No memory leaks
- ✅ Scalability verified

---

## Sprint 6: Observability (48 hours) - Week 8

### 🎯 Goal: Enhanced monitoring and debugging

---

### Task 6.1: Policy Metrics (16 hours)

**Files to Create**:
- `core/pkg/policy/metrics.go` (NEW)

**Metrics to Export**:

1. **Evaluation Metrics**
   - `policy_evaluations_total`
   - `policy_evaluation_duration_seconds`
   - `policy_evaluation_errors_total`

2. **Violation Metrics**
   - `policy_violations_total{severity,action}`
   - `policy_violations_sampled_total`
   - `policy_violations_resolved_total`

3. **Cache Metrics**
   - `policy_cache_hits_total`
   - `policy_cache_misses_total`
   - `policy_cache_size_bytes`

**Implementation Steps**:

1. **Define Metrics** (4 hours)
   - Prometheus metrics
   - Labels and dimensions
   - Documentation

2. **Instrument Code** (8 hours)
   - Add metrics to evaluator
   - Add metrics to violation service
   - Add metrics to cache

3. **Export Endpoint** (4 hours)
   - `/metrics` endpoint
   - Prometheus format
   - Testing

**Success Criteria**:
- ✅ All metrics exported
- ✅ Prometheus compatible
- ✅ Documentation complete

---

### Task 6.2: Enhanced Logging (16 hours)

**Files to Modify**:
- `core/pkg/policy/evaluator.go`
- `core/pkg/policy/violation.go`

**Logging Improvements**:

1. **Structured Logging** (6 hours)
   - JSON format
   - Consistent fields
   - Log levels

2. **Policy Decisions** (4 hours)
   - Log evaluation results
   - Include context
   - Trace IDs

3. **Violation Logging** (4 hours)
   - Log violation creation
   - Include details
   - Sampling decisions

4. **Configurable Levels** (2 hours)
   - Environment-based
   - Per-component
   - Dynamic adjustment

**Success Criteria**:
- ✅ Structured logging
- ✅ Trace IDs
- ✅ Configurable levels

---

### Task 6.3: Policy Debugging Tools (16 hours)

**Files to Create**:
- `core/internal/api/policy_debug_handlers.go` (NEW)

**Debug Endpoints**:

1. **Policy Evaluation Trace** (6 hours)
   - `POST /api/v1/policies/debug/evaluate`
   - Return detailed trace
   - Show CEL evaluation steps

2. **CEL Expression Testing** (4 hours)
   - `POST /api/v1/policies/debug/validate-cel`
   - Test expression
   - Return errors

3. **Policy Impact Analysis** (4 hours)
   - `GET /api/v1/policies/debug/impact?resource_uid=...`
   - Show which policies apply
   - Show evaluation results

4. **Debug Dashboard** (2 hours)
   - Web UI for debugging
   - Visualize policy flow
   - Test expressions

**Success Criteria**:
- ✅ All endpoints functional
- ✅ Useful for debugging
- ✅ Documentation complete

---

## Sprint 7: Compliance & Documentation (68 hours) - Week 9

### 🎯 Goal: Production readiness

---

### Task 7.1: Audit Trail Enhancement (20 hours)

**Files to Create**:
- `core/pkg/policy/audit.go` (NEW)
- `core/migrations/042_add_policy_audit.sql` (NEW)

**Audit Events**:

1. **Template Events**
   - Template created
   - Template updated
   - Template deleted

2. **Instance Events**
   - Instance created
   - Instance updated
   - Instance deleted
   - Instance enabled/disabled

3. **Violation Events**
   - Violation created
   - Violation resolved
   - Violation dismissed

4. **Evaluation Events**
   - Policy evaluated
   - Violation detected
   - Action taken

**Implementation Steps**:

1. **Create Audit Table** (6 hours)
   - `policy_audit_logs` table
   - Event types
   - User tracking

2. **Audit Service** (8 hours)
   - Log all events
   - Include context
   - User attribution

3. **Query API** (4 hours)
   - `GET /api/v1/policies/audit`
   - Filter by event type
   - Time range queries

4. **Retention Policy** (2 hours)
   - Configurable retention
   - Archive old logs
   - Compliance requirements

**Success Criteria**:
- ✅ All events logged
- ✅ Query API functional
- ✅ Retention working

---

### Task 7.2: Compliance Reporting (24 hours)

**Files to Create**:
- `core/internal/api/compliance_handlers.go` (extend)

**Report Types**:

1. **Policy Compliance Report** (8 hours)
   - Per-policy compliance
   - Resource compliance
   - Namespace compliance
   - Export CSV/JSON

2. **Violation Report** (6 hours)
   - Active violations
   - Resolved violations
   - Trends over time
   - Export formats

3. **Scheduled Reports** (6 hours)
   - Daily/weekly/monthly
   - Email delivery
   - Configurable recipients

4. **Compliance Score** (4 hours)
   - Calculate scores
   - Historical trends
   - Benchmarking

**Success Criteria**:
- ✅ All report types functional
- ✅ Export working
- ✅ Scheduling working

---

### Task 7.3: Documentation (24 hours)

**Files to Create**:
- `docs/policy-engine/`

**Documentation Sections**:

1. **Architecture** (4 hours)
   - System overview
   - Component diagrams
   - Data flow

2. **CEL Expression Guide** (6 hours)
   - CEL syntax
   - Common patterns
   - Examples
   - Best practices

3. **Policy Template Guide** (4 hours)
   - Creating templates
   - CEL expressions
   - Testing
   - Versioning

4. **API Documentation** (4 hours)
   - OpenAPI spec
   - Endpoint documentation
   - Examples
   - Error codes

5. **Troubleshooting** (3 hours)
   - Common issues
   - Debugging guide
   - Performance tuning

6. **Best Practices** (3 hours)
   - Policy design
   - Performance
   - Security
   - Compliance

**Success Criteria**:
- ✅ All sections complete
- ✅ Examples working
- ✅ Reviewed and approved

---

## Implementation Timeline

```
Week 1-2:  Sprint 1 - Critical Fixes (48h)
Week 3-4:  Sprint 2 - Risk Integration (56h)
Week 5:    Sprint 3 - Insights Integration (48h)
Week 6:    Sprint 4 - RBAC (40h)
Week 7:    Sprint 5 - Testing (48h)
Week 8:    Sprint 6 - Observability (48h)
Week 9:    Sprint 7 - Compliance & Docs (68h)
```

**Total**: 356 hours over 9 weeks

---

## Success Metrics

### Sprint 1
- ✅ 100% of templates validated before saving
- ✅ CEL logic documented and tested
- ✅ Sampling configurable per policy
- ✅ 10+ default templates available

### Sprint 2-3
- ✅ Policy violations affect risk scores
- ✅ Insights generated from violations
- ✅ Compliance dashboard functional

### Sprint 4-5
- ✅ RBAC enforced on all operations
- ✅ >80% test coverage
- ✅ Performance targets met (<100ms)

### Sprint 6-7
- ✅ Comprehensive metrics and logging
- ✅ Full audit trail
- ✅ Complete documentation

---

## Risk Mitigation

### Technical Risks
1. **CEL Performance**: Monitor latency, optimize if needed
2. **Database Load**: Proper indexing, consider caching
3. **Integration Complexity**: Start simple, iterate

### Process Risks
1. **Scope Creep**: Stick to plan, defer non-critical
2. **Testing Gaps**: Allocate sufficient time
3. **Documentation**: Document as you go

---

## Next Steps

1. ✅ **Review this plan** with stakeholders
2. ⏳ **Prioritize sprints** based on business needs
3. ⏳ **Assign resources** to each sprint
4. ⏳ **Start Sprint 1** with CEL validation

---

**Plan Created**: $(date)  
**Estimated Total Effort**: 356 hours (9 weeks)  
**Status**: Ready for Review and Approval

