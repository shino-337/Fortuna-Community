# MVP2 Phase 2: Policy Engine - Implementation Plan

**Date**: 2025-12-08  
**Status**: 🚀 **STARTING**  
**Previous**: Phase 1.1-1.3 (Risk Analysis) ✅ Complete

---

## 📊 Overview

### Objective
Build a flexible, YAML-based policy engine that allows organizations to define, evaluate, and enforce custom security policies.

### Why Policy Engine?
- **Custom Requirements**: Every organization has unique security needs
- **Compliance**: Required for CIS, NIST, PCI-DSS frameworks
- **Automation**: Enable automated remediation and enforcement
- **Audit**: Track policy violations and compliance

---

## 🎯 Phase 2.1: Policy Definition Language

### Goals
1. Design YAML-based policy schema
2. Implement policy parser and validator
3. Create database schema for policies
4. Build API endpoints for policy management

### Database Schema

```sql
-- Policies table
CREATE TABLE policies (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    version INTEGER NOT NULL DEFAULT 1,
    enabled BOOLEAN DEFAULT true,
    
    -- Policy definition (YAML stored as JSONB)
    definition JSONB NOT NULL,
    
    -- Scope
    scope_clusters TEXT[],
    scope_namespaces TEXT[],
    scope_resource_types TEXT[],
    
    -- Metadata
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_policies_enabled ON policies(enabled) WHERE deleted_at IS NULL;
CREATE INDEX idx_policies_scope_clusters ON policies USING GIN(scope_clusters);
CREATE INDEX idx_policies_scope_namespaces ON policies USING GIN(scope_namespaces);
```

```sql
-- Policy violations table
CREATE TABLE policy_violations (
    id SERIAL PRIMARY KEY,
    policy_id INTEGER NOT NULL REFERENCES policies(id),
    policy_name VARCHAR(255) NOT NULL,
    rule_id VARCHAR(255) NOT NULL,
    rule_name VARCHAR(255) NOT NULL,
    
    -- Resource info
    resource_type VARCHAR(50) NOT NULL,
    resource_uid VARCHAR(255) NOT NULL,
    resource_name VARCHAR(255),
    namespace VARCHAR(255),
    cluster_id VARCHAR(255) NOT NULL,
    
    -- Violation details
    severity VARCHAR(20) NOT NULL,
    action VARCHAR(50) NOT NULL, -- alert, block, remediate
    status VARCHAR(50) DEFAULT 'active', -- active, resolved, dismissed
    message TEXT,
    
    -- Enforcement
    enforced_at TIMESTAMP WITH TIME ZONE,
    enforcement_result TEXT,
    
    -- Metadata
    detected_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    resolved_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX idx_violations_policy ON policy_violations(policy_id);
CREATE INDEX idx_violations_resource ON policy_violations(resource_type, resource_uid);
CREATE INDEX idx_violations_status ON policy_violations(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_violations_cluster ON policy_violations(cluster_id);
```

### YAML Policy Schema

```yaml
apiVersion: ksam.io/v1
kind: SecurityPolicy
metadata:
  name: no-privileged-containers
  description: Prevent privileged containers in production
  version: 1
  enabled: true
spec:
  scope:
    clusters:
      - prod-cluster-*
    namespaces:
      - production
      - default
    resourceTypes:
      - Pod
  
  rules:
    - id: privileged-check
      name: Check for privileged containers
      description: Containers must not run in privileged mode
      severity: critical
      
      conditions:
        - type: field
          field: spec.containers[].securityContext.privileged
          operator: equals
          value: true
      
      action: block  # alert, block, remediate, quarantine
      
      remediation:
        type: auto
        script: |
          kubectl patch pod ${POD_NAME} -n ${NAMESPACE} \
            -p '{"spec":{"containers":[{"securityContext":{"privileged":false}}]}}'
      
      exceptions:
        - namespace: kube-system
          reason: System pods may need privileged access
```

### Implementation Steps

1. **Database Migration** (2 hours)
   - Create `policies` table
   - Create `policy_violations` table
   - Add indexes

2. **Policy Models** (2 hours)
   - `pkg/models/policy.go` - Policy struct
   - `pkg/models/policy_violation.go` - Violation struct

3. **YAML Parser** (4 hours)
   - `pkg/policy/parser.go` - YAML to Policy struct
   - `pkg/policy/validator.go` - Validate policy schema
   - Support for CEL expressions

4. **Policy Service** (4 hours)
   - `pkg/policy/service.go` - CRUD operations
   - Load policies from database
   - Cache policies for performance

5. **API Endpoints** (4 hours)
   - `POST /api/v1/policies` - Create policy
   - `GET /api/v1/policies` - List policies
   - `GET /api/v1/policies/:id` - Get policy
   - `PUT /api/v1/policies/:id` - Update policy
   - `DELETE /api/v1/policies/:id` - Delete policy
   - `POST /api/v1/policies/:id/validate` - Validate policy
   - `GET /api/v1/policies/:id/violations` - Get violations

**Total Effort**: 16 hours (2 days)

---

## 🎯 Phase 2.2: Policy Evaluation Engine

### Goals
1. Real-time policy evaluation
2. Integration with resource processing
3. Violation detection and tracking
4. CEL expression evaluation

### Implementation Steps

1. **Evaluator Core** (6 hours)
   - `pkg/policy/evaluator.go` - Main evaluator
   - Match resources to policies (scope)
   - Evaluate rules against resources
   - Generate violations

2. **CEL Integration** (4 hours)
   - Integrate CEL library
   - Expression evaluation
   - Resource context injection

3. **Integration Points** (4 hours)
   - Hook into CorrelatorWorker
   - Evaluate on resource creation/update
   - Async evaluation for batch processing

4. **Violation Manager** (4 hours)
   - Create/update violations
   - Deduplication logic
   - Auto-resolution when fixed

**Total Effort**: 18 hours (2.5 days)

---

## 🎯 Phase 2.3: Policy Enforcement

### Goals
1. Implement enforcement actions
2. Admission webhook for blocking
3. Automated remediation
4. Audit logging

### Enforcement Actions

1. **Alert** (1 hour)
   - Create violation record
   - Send notification
   - Log to audit

2. **Block** (8 hours)
   - Kubernetes admission webhook
   - Validate resource before creation
   - Return rejection with reason

3. **Remediate** (6 hours)
   - Auto-fix common issues
   - Patch resources
   - Rollback support

4. **Quarantine** (4 hours)
   - Label resources
   - Network policy isolation
   - Resource limits

**Total Effort**: 19 hours (2.5 days)

---

## 📋 Implementation Order

### Week 1: Phase 2.1 (Policy Definition)
- Day 1-2: Database schema + Models
- Day 3-4: YAML parser + Validator
- Day 5: API endpoints + Testing

### Week 2: Phase 2.2 (Evaluation Engine)
- Day 1-2: Evaluator core
- Day 3: CEL integration
- Day 4-5: Integration + Violation tracking

### Week 3: Phase 2.3 (Enforcement)
- Day 1-2: Alert + Remediate actions
- Day 3-4: Admission webhook (block)
- Day 5: Testing + Documentation

---

## ✅ Success Criteria

1. ✅ Can create policies via YAML
2. ✅ Policies are validated on creation
3. ✅ Resources are evaluated against policies
4. ✅ Violations are detected and tracked
5. ✅ Enforcement actions work (alert, block, remediate)
6. ✅ API endpoints functional
7. ✅ E2E tests pass

---

## 📝 Next Steps

1. **Start with Phase 2.1.1**: Database schema migration
2. **Create models**: Policy and PolicyViolation structs
3. **Implement parser**: YAML to Policy conversion
4. **Build APIs**: Policy CRUD endpoints

---

**Status**: Ready to start Phase 2.1  
**Estimated Completion**: 3 weeks  
**Priority**: HIGH (Strategic feature)

