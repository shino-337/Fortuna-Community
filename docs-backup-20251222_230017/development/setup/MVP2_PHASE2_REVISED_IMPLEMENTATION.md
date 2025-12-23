# MVP2 Phase 2: Policy Engine - Revised Implementation Plan

**Date**: 2025-12-08  
**Status**: 🚀 **STARTING**  
**Based On**: POLICY_ENGINE_CRITICAL_REVISIONS_PART1.md

---

## 📊 Key Architectural Changes

### ✅ Revised Design Principles

1. **Template vs Instance Pattern**
   - Templates: Built-in, immutable, contain CEL logic
   - Instances: User-configurable, only scope/action/severity
   - Benefits: Simplicity, consistency, safety

2. **CEL Program Caching**
   - Compile once, cache in memory
   - Optional: Cache in database for fast restart
   - Performance: 1000x faster evaluation

3. **Async Admission Webhook**
   - Fast path: <100ms (evaluation only)
   - Slow path: Async (violation storage, alerts, insights)
   - Benefits: No deployment blocking, better reliability

---

## 🎯 Phase 2.1: Policy Definition (Revised)

### Step 1: Database Schema

#### Table 1: `policy_templates` (Immutable, Built-in)

```sql
CREATE TABLE policy_templates (
    id SERIAL PRIMARY KEY,
    
    -- Template identity
    template_id VARCHAR(255) NOT NULL UNIQUE,
    version VARCHAR(20) NOT NULL,
    
    -- Metadata
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(50) NOT NULL,
    default_severity VARCHAR(20) NOT NULL,
    
    -- Immutable logic
    cel_expression TEXT NOT NULL,
    cel_program_cache BYTEA,
    
    -- Default configuration
    default_scope JSONB,
    default_action VARCHAR(20) DEFAULT 'alert',
    
    -- Remediation
    supports_remediation BOOLEAN DEFAULT false,
    remediation_template JSONB,
    
    -- Documentation
    rationale TEXT,
    references TEXT[],
    examples JSONB,
    
    -- Metadata
    created_by VARCHAR(255) DEFAULT 'system',
    is_system BOOLEAN DEFAULT true,
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    CONSTRAINT check_category CHECK (category IN ('security', 'compliance', 'operational', 'governance')),
    CONSTRAINT check_action CHECK (default_action IN ('alert', 'block', 'audit'))
);

CREATE UNIQUE INDEX idx_policy_templates_id_version ON policy_templates(template_id, version);
CREATE INDEX idx_policy_templates_category ON policy_templates(category);
```

#### Table 2: `policy_instances` (User-configurable)

```sql
CREATE TABLE policy_instances (
    id SERIAL PRIMARY KEY,
    
    -- Template reference
    template_id VARCHAR(255) NOT NULL,
    template_version VARCHAR(20) NOT NULL,
    
    -- Instance identity
    instance_name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    
    -- Status
    enabled BOOLEAN DEFAULT true,
    
    -- Scope (USER CONFIGURABLE)
    clusters TEXT[],
    namespaces TEXT[],
    resource_types TEXT[],
    label_selectors JSONB,
    
    -- Action override (USER CONFIGURABLE)
    action VARCHAR(20),
    severity VARCHAR(20),
    
    -- Message override
    custom_message TEXT,
    
    -- Remediation settings
    auto_remediate BOOLEAN DEFAULT false,
    remediation_dry_run BOOLEAN DEFAULT true,
    
    -- Exemptions
    exemptions JSONB,
    
    -- Metadata
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    FOREIGN KEY (template_id, template_version) 
        REFERENCES policy_templates(template_id, version),
    
    CONSTRAINT check_action CHECK (action IN ('alert', 'block', 'audit', 'remediate')),
    CONSTRAINT check_severity CHECK (severity IN ('low', 'medium', 'high', 'critical'))
);

CREATE INDEX idx_policy_instances_template ON policy_instances(template_id, template_version);
CREATE INDEX idx_policy_instances_enabled ON policy_instances(enabled) WHERE deleted_at IS NULL;
```

#### Table 3: `policy_violations` (Revised)

```sql
CREATE TABLE policy_violations (
    id SERIAL PRIMARY KEY,
    
    -- Policy reference
    instance_id INTEGER NOT NULL REFERENCES policy_instances(id),
    instance_name VARCHAR(255) NOT NULL,
    template_id VARCHAR(255) NOT NULL,
    template_name VARCHAR(255) NOT NULL,
    
    -- Resource info
    resource_type VARCHAR(50) NOT NULL,
    resource_uid VARCHAR(255) NOT NULL,
    resource_name VARCHAR(255),
    namespace VARCHAR(255),
    cluster_id VARCHAR(255) NOT NULL,
    
    -- Violation details
    severity VARCHAR(20) NOT NULL,
    action VARCHAR(50) NOT NULL,
    status VARCHAR(50) DEFAULT 'active',
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

CREATE INDEX idx_violations_instance ON policy_violations(instance_id);
CREATE INDEX idx_violations_resource ON policy_violations(resource_type, resource_uid);
CREATE INDEX idx_violations_status ON policy_violations(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_violations_cluster ON policy_violations(cluster_id);
```

---

## 📝 Implementation Steps

### Step 1: Database Migration (2 hours)
- Create migration file for policy_templates
- Create migration file for policy_instances  
- Create migration file for policy_violations
- Register migrations

### Step 2: Models (2 hours)
- `pkg/models/policy_template.go`
- `pkg/models/policy_instance.go`
- `pkg/models/policy_violation.go`

### Step 3: CEL Evaluator with Caching (4 hours)
- `pkg/policy/evaluator.go` - Main evaluator
- CEL program caching (in-memory)
- Template/instance loading
- Fast evaluation path

### Step 4: YAML Parser (3 hours)
- `pkg/policy/parser.go` - Template parser
- `pkg/policy/validator.go` - Schema validation
- Load templates from YAML files

### Step 5: Policy Service (3 hours)
- `pkg/policy/service.go` - CRUD operations
- Template management
- Instance management
- Cache invalidation

### Step 6: API Endpoints (4 hours)
- `POST /api/v1/policy-templates` - Create template (admin only)
- `GET /api/v1/policy-templates` - List templates
- `GET /api/v1/policy-templates/:id` - Get template
- `POST /api/v1/policy-instances` - Create instance
- `GET /api/v1/policy-instances` - List instances
- `PUT /api/v1/policy-instances/:id` - Update instance
- `DELETE /api/v1/policy-instances/:id` - Delete instance
- `GET /api/v1/policy-instances/:id/violations` - Get violations

**Total Effort**: 18 hours (2.5 days)

---

## 🚀 Starting Implementation

Let's begin with Step 1: Database Schema Migration.

