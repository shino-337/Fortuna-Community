# Policy Engine Component

The Policy Engine evaluates security policies using CEL (Common Expression Language) and YAML-based rules to detect misconfigurations and violations.

---

## Overview

**Status**: ✅ Production Ready (MVP2)
**Rule Format**: YAML with CEL expressions
**Evaluation**: Real-time on resource sync
**Enforcement**: Optional admission webhook blocking

---

## Architecture

```
Resource Synced (ServiceAccount/Pod)
    ↓
Policy Engine
    ├─▶ Load Policy Templates (from database)
    ├─▶ Evaluate CEL Expressions
    │    ├─▶ object.metadata.name
    │    ├─▶ object.metadata.namespace
    │    └─▶ Custom functions (hasLabel, etc.)
    └─▶ Create Policy Violations
        ↓
Risk Engine
    └─▶ Create Insights (if severity threshold met)
```

---

## Quick Start

### View Active Policies

```bash
# List policy templates
kubectl exec -n ksam postgres-* -- psql -U postgres -d ksam -c \
  "SELECT name, severity, category, enabled
   FROM policy_templates
   WHERE deleted_at IS NULL;"
```

### Check Policy Violations

```bash
# List violations
kubectl exec -n ksam postgres-* -- psql -U postgres -d ksam -c \
  "SELECT pv.resource_name, pt.name AS policy_name, pv.severity, pv.message
   FROM policy_violations pv
   JOIN policy_templates pt ON pv.policy_id = pt.id
   WHERE pv.deleted_at IS NULL
   LIMIT 20;"
```

### Test Policy Evaluation

```bash
# Deploy a pod that violates policy
kubectl run privileged-pod --image=nginx --privileged=true

# Wait for evaluation
sleep 5

# Check for violation
kubectl exec -n ksam postgres-* -- psql -U postgres -d ksam -c \
  "SELECT * FROM policy_violations WHERE resource_name = 'privileged-pod';"
```

---

## Policy Format (YAML)

### Example: Privileged Container Detection

**File**: `core/rules/privileged-container.yaml`

```yaml
apiVersion: policy.ksam.io/v1
kind: PolicyTemplate
metadata:
  name: privileged-container-detected
  description: Detects pods running privileged containers
spec:
  severity: CRITICAL
  category: Security
  resourceTypes:
    - Pod

  # CEL expression (evaluated against Pod object)
  expression: |
    object.spec.containers.exists(c,
      has(c.securityContext) &&
      has(c.securityContext.privileged) &&
      c.securityContext.privileged == true
    )

  # Violation message template
  message: "Pod '{{.object.metadata.name}}' contains privileged container"

  # Remediation steps
  remediation: |
    1. Remove 'privileged: true' from container securityContext
    2. Use specific capabilities instead
    3. Example:
       securityContext:
         capabilities:
           add: ["NET_ADMIN"]

  # Additional metadata
  references:
    - https://kubernetes.io/docs/concepts/security/pod-security-standards/

  tags:
    - cis-benchmark
    - privilege-escalation
    - pod-security
```

### Example: Cluster-Admin Binding

**File**: `core/rules/cluster-admin-binding.yaml`

```yaml
apiVersion: policy.ksam.io/v1
kind: PolicyTemplate
metadata:
  name: cluster-admin-binding-detected
  description: Detects ServiceAccounts with cluster-admin role
spec:
  severity: CRITICAL
  category: RBAC
  resourceTypes:
    - ServiceAccount

  expression: |
    object.bindings.exists(b,
      b.role_name == "cluster-admin"
    )

  message: "ServiceAccount '{{.object.metadata.namespace}}/{{.object.metadata.name}}' has cluster-admin role"

  remediation: |
    1. Review if cluster-admin access is necessary
    2. Use least-privilege principle
    3. Create custom role with only required permissions
    4. Example:
       apiVersion: rbac.authorization.k8s.io/v1
       kind: Role
       metadata:
         name: specific-permissions
       rules:
       - apiGroups: [""]
         resources: ["pods"]
         verbs: ["get", "list"]
```

---

## CEL Expression Guide

### Available Object Fields

#### For Pods:
```cel
object.metadata.name           // Pod name
object.metadata.namespace      // Namespace
object.metadata.labels         // Map of labels
object.spec.containers         // Array of containers
object.spec.serviceAccountName // ServiceAccount used
```

#### For ServiceAccounts:
```cel
object.metadata.name           // SA name
object.metadata.namespace      // Namespace
object.bindings                // Array of RoleBindings
object.pods                    // Array of Pods using this SA
```

### CEL Functions

**Built-in**:
```cel
// String operations
object.metadata.name.startsWith("test-")
object.metadata.name.contains("admin")
object.metadata.name.matches("^prod-.*")

// Array operations
object.spec.containers.exists(c, c.image == "nginx:latest")
object.spec.containers.all(c, has(c.resources.limits))
object.spec.containers.size() > 5

// Map operations
has(object.metadata.labels["app"])
object.metadata.labels["env"] == "production"
```

**Custom Functions** (planned):
```cel
hasLabel("app", "web")
hasAnnotation("security", "high")
hasCapability("SYS_ADMIN")
inNamespace(["kube-system", "kube-public"])
```

---

## Database Schema

### policy_templates Table

```sql
CREATE TABLE policy_templates (
    id UUID PRIMARY KEY,
    name VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    severity VARCHAR(20) NOT NULL,  -- CRITICAL, HIGH, MEDIUM, LOW
    category VARCHAR(100),           -- Security, RBAC, Network, etc.
    resource_types TEXT[],           -- ["Pod", "ServiceAccount"]
    expression TEXT NOT NULL,        -- CEL expression
    message_template TEXT,
    remediation TEXT,
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP,
    deleted_at TIMESTAMP
);
```

### policy_violations Table

```sql
CREATE TABLE policy_violations (
    id UUID PRIMARY KEY,
    policy_id UUID REFERENCES policy_templates(id),
    resource_type VARCHAR(50),
    resource_name VARCHAR(255),
    resource_namespace VARCHAR(255),
    resource_uid VARCHAR(100),
    severity VARCHAR(20),
    message TEXT,
    detected_at TIMESTAMP,
    resolved_at TIMESTAMP,
    deleted_at TIMESTAMP,
    UNIQUE(policy_id, resource_uid)  -- One violation per resource per policy
);
```

---

## Configuration

### Environment Variables

```yaml
# Policy Engine
POLICY_ENGINE_ENABLED: "true"
POLICY_EVALUATION_ON_SYNC: "true"
POLICY_EVALUATION_INTERVAL: "300s"  # Re-evaluate every 5 min

# Policy Loading
POLICY_AUTO_LOAD: "true"
POLICY_RULES_DIR: "/app/rules"  # YAML files directory

# Enforcement
POLICY_ENFORCEMENT_ENABLED: "false"  # Webhook blocking
POLICY_ENFORCEMENT_MODE: "warn"      # warn, deny
```

---

## Built-in Policies

KSAM ships with built-in policies covering:

### Security Policies
- Privileged containers
- Host network usage
- Host PID/IPC usage
- HostPath volumes
- Writable root filesystem
- Missing security context

### RBAC Policies
- Cluster-admin bindings
- Wildcard RBAC permissions
- ServiceAccount with secrets access
- Excessive pod permissions

### Network Policies
- Missing network policies
- Allow-all ingress/egress
- External traffic exposure

### Resource Policies
- Missing resource limits
- Missing resource requests
- Large resource requests

### Compliance Policies
- CIS Benchmark violations
- PCI-DSS requirements
- HIPAA requirements

**Location**: `core/rules/*.yaml` (20+ policies)

---

## API Endpoints

### Policy Templates

```bash
# List policy templates
curl http://localhost:8080/api/v1/policies

# Get specific template
curl http://localhost:8080/api/v1/policies/{policy_id}

# Create custom policy
curl -X POST http://localhost:8080/api/v1/policies \
  -H "Content-Type: application/yaml" \
  --data-binary @custom-policy.yaml

# Update policy
curl -X PUT http://localhost:8080/api/v1/policies/{policy_id} \
  -H "Content-Type: application/yaml" \
  --data-binary @updated-policy.yaml

# Disable policy
curl -X PATCH http://localhost:8080/api/v1/policies/{policy_id} \
  -d '{"enabled": false}'
```

### Policy Violations

```bash
# List violations
curl http://localhost:8080/api/v1/violations?severity=CRITICAL

# Get violations for resource
curl http://localhost:8080/api/v1/violations/by-resource/{namespace}/{name}

# Resolve violation
curl -X POST http://localhost:8080/api/v1/violations/{violation_id}/resolve
```

---

## Admission Webhook Integration

### Enable Enforcement

```yaml
# core/internal/webhook/config.yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: ksam-policy-enforcer
webhooks:
- name: validate.pods.ksam.io
  rules:
  - apiGroups: [""]
    apiVersions: ["v1"]
    operations: ["CREATE", "UPDATE"]
    resources: ["pods"]
  clientConfig:
    service:
      name: ksam-core
      namespace: ksam
      path: /validate-pods
  admissionReviewVersions: ["v1"]
  sideEffects: None
  failurePolicy: Fail  # Block on policy violations
```

### Webhook Behavior

**Enforcement Modes**:
- **warn**: Allow creation, log violation
- **deny**: Block creation, return error message

**Example Denial**:
```bash
kubectl run privileged-pod --image=nginx --privileged=true

# Error:
Error from server: admission webhook "validate.pods.ksam.io" denied the request:
Policy violation: privileged-container-detected
Severity: CRITICAL
Message: Pod 'privileged-pod' contains privileged container
Remediation: Remove 'privileged: true' from container securityContext
```

---

## Custom Policy Development

### Step 1: Write YAML Rule

```yaml
apiVersion: policy.ksam.io/v1
kind: PolicyTemplate
metadata:
  name: custom-label-required
  description: Ensure pods have required labels
spec:
  severity: MEDIUM
  category: Governance
  resourceTypes:
    - Pod

  expression: |
    has(object.metadata.labels) &&
    has(object.metadata.labels["team"]) &&
    has(object.metadata.labels["environment"])

  message: "Pod missing required labels: team, environment"

  remediation: |
    Add the following labels to your pod:
    metadata:
      labels:
        team: <team-name>
        environment: <env-name>
```

### Step 2: Test Locally

```bash
# Load policy
curl -X POST http://localhost:8080/api/v1/policies \
  -H "Content-Type: application/yaml" \
  --data-binary @custom-label-required.yaml

# Test with compliant pod
kubectl run good-pod --image=nginx \
  --labels="team=platform,environment=prod"
# Result: No violation ✅

# Test with non-compliant pod
kubectl run bad-pod --image=nginx
# Result: Violation created ❌
```

### Step 3: Deploy to Production

```bash
# Add to built-in rules
cp custom-label-required.yaml core/rules/

# Rebuild and deploy
docker build -t ksam/core:v1.1.0 core/
kubectl set image deployment/ksam-core -n ksam core=ksam/core:v1.1.0
```

---

## Troubleshooting

### Policy Not Evaluating

**Check Policy Loaded**:
```sql
SELECT * FROM policy_templates WHERE name = 'your-policy-name';
```

**Check Enabled**:
```sql
UPDATE policy_templates SET enabled = true WHERE name = 'your-policy-name';
```

**Check Logs**:
```bash
kubectl logs -n ksam ksam-core-* | grep -i "policy\\|cel"
```

### CEL Expression Errors

**Symptom**: Logs show "CEL compilation error"

**Debug**:
```bash
# Test expression manually
curl -X POST http://localhost:8080/api/v1/policies/test \
  -d '{
    "expression": "object.metadata.name.startsWith(\"test\")",
    "object": {"metadata": {"name": "test-pod"}}
  }'
```

**Common Errors**:
- Typo in field name
- Missing `has()` check for optional fields
- Wrong operator (== vs =)

### Violations Not Creating Insights

**Check Risk Engine**:
```bash
kubectl logs -n ksam ksam-core-* | grep -i "insight\\|risk"
```

**Check Severity Threshold**:
- Only CRITICAL and HIGH violations create insights by default
- Configure in Risk Engine settings

---

## Performance

| Operation | Time | Notes |
|-----------|------|-------|
| Policy Compilation | ~5ms | Once per policy load |
| Expression Evaluation | ~1ms | Per resource |
| Batch Evaluation (1000 pods) | ~1s | Parallel evaluation |

**Optimization**:
- Compiled CEL programs cached
- Parallel policy evaluation
- Database batching for violations

---

## Related Components

- [Risk Engine](../risk-engine/) - Creates insights from violations
- [Core](../core/) - Orchestrates policy evaluation
- [Webhook](../core/webhook/) - Enforces policies at admission

---

**Last Updated**: December 16, 2025
**Status**: ✅ Production Ready
**Rule Count**: 20+ built-in policies
