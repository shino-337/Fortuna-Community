# Risk Engine Component

The Risk Engine calculates risk scores, aggregates security findings, and creates actionable insights for security teams.

---

## Overview

**Status**: ✅ Production Ready (MVP2)
**Risk Scoring**: Multi-factor weighted algorithm
**Insight Sources**: CVE matches, policy violations, RBAC analysis
**Update Frequency**: Real-time + periodic re-evaluation

---

## Architecture

```
Risk Sources
    ├─▶ CVE Matches (vulnerability scanner)
    ├─▶ Policy Violations (policy engine)
    ├─▶ RBAC Analysis (permission analyzer)
    └─▶ Behavioral Anomalies (planned)
        ↓
Risk Scoring Algorithm
    ├─▶ Base Score (from source)
    ├─▶ Context Multipliers
    │    ├─▶ Namespace sensitivity
    │    ├─▶ Resource exposure
    │    └─▶ Privilege level
    └─▶ Final Risk Score (0-10)
        ↓
Insight Creation
    ├─▶ Aggregation (group related findings)
    ├─▶ De-duplication
    ├─▶ Severity assignment
    └─▶ Database storage
        ↓
Dashboard Display + API Access
```

---

## Risk Scoring Algorithm

### Base Formula

```
Final Risk Score = Base Score × Context Multiplier × Exposure Factor

Where:
- Base Score: Source-specific (CVE CVSS, Policy severity, RBAC weight)
- Context Multiplier: 1.0 - 2.0 (based on namespace, resource type)
- Exposure Factor: 1.0 - 1.5 (based on network exposure, privilege)
```

### Base Scores

**CVE Severity**:
```
CRITICAL → 10.0
HIGH     → 8.0
MEDIUM   → 5.0
LOW      → 2.0
```

**Policy Violations**:
```
CRITICAL → 9.0
HIGH     → 7.0
MEDIUM   → 4.0
LOW      → 2.0
```

**RBAC Issues**:
```
cluster-admin binding       → 10.0
Wildcard permissions        → 8.0
Secrets access              → 7.0
Elevated pod permissions    → 6.0
```

### Context Multipliers

**Namespace Sensitivity**:
```
kube-system, kube-public    → 1.5× (critical namespaces)
default                     → 1.3× (often over-permissioned)
production, prod-*          → 1.4× (production workloads)
dev, test, staging          → 1.0× (non-production)
```

**Resource Type**:
```
Pod with hostNetwork        → 1.5×
ServiceAccount with secrets → 1.3×
DaemonSet                   → 1.2×
Regular Pod                 → 1.0×
```

### Exposure Factors

**Network Exposure**:
```
LoadBalancer service        → 1.5×
NodePort service            → 1.3×
ClusterIP (external access) → 1.2×
ClusterIP (internal only)   → 1.0×
```

**Privilege Level**:
```
privileged: true            → 1.5×
hostPID/hostIPC: true       → 1.4×
hostPath volumes            → 1.3×
capabilities: SYS_ADMIN     → 1.4×
```

### Example Calculation

**Scenario**: Critical CVE in nginx:1.19.0 in kube-system namespace

```
Base Score: 10.0 (CRITICAL CVE)
Context Multiplier: 1.5× (kube-system)
Exposure Factor: 1.3× (NodePort service)

Final Score = 10.0 × 1.5 × 1.3 = 19.5 → capped at 10.0
```

---

## Insight Types

### 1. CVE-Based Insights

**Source**: SBOM + CVE matching

**Example**:
```json
{
  "title": "Critical Vulnerability: CVE-2021-33560",
  "description": "libgcrypt memory leak in pod nginx-prod",
  "severity": "CRITICAL",
  "risk_score": 9.5,
  "source": "cve_scanner",
  "cve_id": "CVE-2021-33560",
  "affected_resource": "nginx-prod",
  "remediation": "Update to nginx:1.21.0 or later"
}
```

### 2. Policy Violation Insights

**Source**: Policy engine evaluation

**Example**:
```json
{
  "title": "Privileged Container Detected",
  "description": "Pod 'admin-debug' running with privileged: true",
  "severity": "CRITICAL",
  "risk_score": 9.0,
  "source": "policy_engine",
  "policy_name": "privileged-container-detected",
  "affected_resource": "admin-debug",
  "remediation": "Remove privileged flag, use specific capabilities"
}
```

### 3. RBAC Analysis Insights

**Source**: ServiceAccount permission analysis

**Example**:
```json
{
  "title": "Cluster Admin Binding Detected",
  "description": "ServiceAccount 'default' in namespace 'kube-system' has cluster-admin role",
  "severity": "CRITICAL",
  "risk_score": 10.0,
  "source": "rbac_analyzer",
  "affected_resource": "kube-system/default",
  "remediation": "Remove cluster-admin, create least-privilege role"
}
```

---

## Database Schema

### insights Table

```sql
CREATE TABLE insights (
    id UUID PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    severity VARCHAR(20) NOT NULL,  -- CRITICAL, HIGH, MEDIUM, LOW
    risk_score FLOAT,               -- 0.0 - 10.0
    source VARCHAR(50),             -- cve_scanner, policy_engine, rbac_analyzer

    -- Source-specific fields
    cve_id VARCHAR(20),
    policy_id UUID,

    -- Affected resource
    resource_type VARCHAR(50),
    resource_name VARCHAR(255),
    resource_namespace VARCHAR(255),
    resource_uid VARCHAR(100),

    -- Lifecycle
    status VARCHAR(20) DEFAULT 'active',  -- active, resolved, dismissed
    detected_at TIMESTAMP,
    resolved_at TIMESTAMP,

    -- Remediation
    remediation TEXT,

    created_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP,

    INDEX idx_insights_severity_status (severity, status),
    INDEX idx_insights_resource (resource_type, resource_namespace, resource_name)
);
```

---

## Configuration

### Environment Variables

```yaml
# Risk Engine
RISK_ENGINE_ENABLED: "true"
RISK_EVALUATION_INTERVAL: "300s"  # Re-evaluate every 5 min

# Scoring
RISK_SCORE_MIN: "0.0"
RISK_SCORE_MAX: "10.0"
RISK_NAMESPACE_MULTIPLIERS: '{"kube-system": 1.5, "default": 1.3}'

# Insight Creation
INSIGHT_MIN_SEVERITY: "MEDIUM"        # Don't create insights for LOW
INSIGHT_AUTO_RESOLVE: "true"          # Auto-resolve when issue fixed
INSIGHT_AGGREGATION_ENABLED: "true"   # Group similar insights

# Cleanup
INSIGHT_RETENTION_DAYS: "90"          # Keep resolved insights for 90 days
INSIGHT_CLEANUP_ENABLED: "true"
```

---

## API Endpoints

### Insights

```bash
# List insights
curl http://localhost:8080/api/v1/insights?severity=CRITICAL&status=active

# Get specific insight
curl http://localhost:8080/api/v1/insights/{insight_id}

# Resolve insight
curl -X POST http://localhost:8080/api/v1/insights/{insight_id}/resolve

# Dismiss insight
curl -X POST http://localhost:8080/api/v1/insights/{insight_id}/dismiss \
  -d '{"reason": "False positive - approved by security team"}'

# Get insights by resource
curl http://localhost:8080/api/v1/insights/by-resource/{namespace}/{name}
```

### Risk Scores

```bash
# Get risk score for ServiceAccount
curl http://localhost:8080/api/v1/serviceaccounts/{namespace}/{name}/risk-score

# Response:
{
  "resource": "kube-system/default",
  "risk_score": 9.5,
  "factors": {
    "rbac_issues": 10.0,
    "cve_matches": 0.0,
    "policy_violations": 9.0
  },
  "insights_count": {
    "critical": 2,
    "high": 5,
    "medium": 12,
    "low": 3
  }
}
```

### Risk Trends

```bash
# Get risk trends over time
curl http://localhost:8080/api/v1/risk-trends?days=30

# Response:
{
  "trends": [
    {
      "date": "2025-01-01",
      "avg_risk_score": 5.2,
      "critical_count": 12,
      "high_count": 45,
      "medium_count": 123,
      "low_count": 456
    }
  ]
}
```

---

## Insight Lifecycle

### States

1. **active**: Issue detected and unresolved
2. **resolved**: Issue fixed (auto or manual)
3. **dismissed**: Acknowledged but not fixed

### Auto-Resolution

Insights are automatically resolved when:

**CVE-based**:
- Image updated to non-vulnerable version
- Pod deleted
- SBOM re-scanned with no CVE match

**Policy violation**:
- Resource configuration changed
- Policy re-evaluated with no violation
- Resource deleted

**RBAC-based**:
- RoleBinding removed
- ServiceAccount permissions changed
- ServiceAccount deleted

### Manual Operations

```bash
# Resolve with reason
curl -X POST http://localhost:8080/api/v1/insights/{id}/resolve \
  -d '{"reason": "Image updated to latest version"}'

# Dismiss (won't fix)
curl -X POST http://localhost:8080/api/v1/insights/{id}/dismiss \
  -d '{"reason": "Accepted risk - requires privileged access"}'

# Re-activate
curl -X POST http://localhost:8080/api/v1/insights/{id}/reactivate
```

---

## Aggregation & De-duplication

### Aggregation Rules

**CVE Insights**:
- Group by CVE ID + namespace
- "5 pods in namespace 'production' affected by CVE-2021-33560"

**Policy Insights**:
- Group by policy + namespace
- "10 pods violating 'privileged-container' policy"

**RBAC Insights**:
- No aggregation (each SA is unique)

### De-duplication

**Unique Constraint**:
```
(resource_uid, source, cve_id, policy_id)
```

Prevents:
- Duplicate CVE insights for same pod
- Duplicate policy violations
- Re-creating resolved insights

---

## Monitoring

### Prometheus Metrics

```
# Insight counts
ksam_insights_total{severity="critical",status="active"}
ksam_insights_created_total{source="cve_scanner"}
ksam_insights_resolved_total{source="policy_engine"}

# Risk scores
ksam_risk_score_avg
ksam_risk_score_max
ksam_risk_score_distribution{bucket="critical"}

# Processing
ksam_risk_evaluation_duration_seconds
ksam_risk_evaluation_errors_total
```

### Grafana Dashboard

Metrics tracked:
- Active insights by severity
- Risk score trends
- Insight creation/resolution rates
- Top risky resources
- Insight sources breakdown

---

## Troubleshooting

### Insights Not Being Created

**Check Risk Engine**:
```bash
kubectl logs -n ksam ksam-core-* | grep -i "risk\\|insight"
```

**Check Database**:
```sql
-- Check CVE matches exist
SELECT COUNT(*) FROM cve_matches WHERE deleted_at IS NULL;

-- Check policy violations exist
SELECT COUNT(*) FROM policy_violations WHERE deleted_at IS NULL;
```

**Common Issues**:
- Risk engine disabled
- Severity below threshold (INSIGHT_MIN_SEVERITY)
- Database transaction failures

### Risk Scores Incorrect

**Debug Scoring**:
```bash
# Enable debug logging
kubectl set env deployment/ksam-core -n ksam LOG_LEVEL=debug

# Check scoring logs
kubectl logs -n ksam ksam-core-* | grep -i "score\\|multiplier"
```

**Verify Multipliers**:
```sql
-- Check namespace multipliers
SELECT
    namespace,
    AVG(risk_score) as avg_score
FROM serviceaccounts
GROUP BY namespace
ORDER BY avg_score DESC;
```

### Insights Not Auto-Resolving

**Check Auto-Resolve Setting**:
```bash
kubectl get configmap -n ksam ksam-core-config -o yaml | grep INSIGHT_AUTO_RESOLVE
```

**Manual Resolution**:
```sql
-- Resolve orphaned insights (resource deleted)
UPDATE insights
SET status = 'resolved', resolved_at = NOW()
WHERE status = 'active'
  AND resource_uid NOT IN (SELECT uid FROM serviceaccounts UNION SELECT uid FROM pods);
```

---

## Testing

### Create Test Insight

```bash
# Deploy vulnerable pod
kubectl run vuln-test --image=nginx:1.19.0

# Wait for SBOM + CVE matching
sleep 60

# Check insight created
kubectl exec -n ksam postgres-* -- psql -U postgres -d ksam -c \
  "SELECT title, severity, risk_score FROM insights WHERE resource_name = 'vuln-test';"
```

### Verify Auto-Resolution

```bash
# Delete pod
kubectl delete pod vuln-test

# Wait for cleanup
sleep 30

# Check insight resolved
kubectl exec -n ksam postgres-* -- psql -U postgres -d ksam -c \
  "SELECT status, resolved_at FROM insights WHERE resource_name = 'vuln-test';"
```

---

## Related Components

- [CVE Scanner](../cve-scanner/) - Provides CVE-based insights
- [Policy Engine](../policy-engine/) - Provides policy violation insights
- [Core](../core/) - Orchestrates risk evaluation
- [Dashboard](../dashboard/) - Displays insights and risk scores

---

## Future Enhancements

### Planned Features

1. **Machine Learning**:
   - Anomaly detection
   - Risk prediction
   - False positive reduction

2. **Advanced Correlation**:
   - Attack path risk scoring
   - Blast radius calculation
   - Cascading risk analysis

3. **Custom Risk Models**:
   - Industry-specific scoring (fintech, healthcare)
   - Compliance-based weights (PCI-DSS, HIPAA)
   - Organization-specific multipliers

4. **Insight Workflow**:
   - Assignment to security team
   - SLA tracking
   - Escalation rules
   - Integration with ticketing systems

---

**Last Updated**: December 16, 2025
**Status**: ✅ Production Ready
**Insight Sources**: 3 (CVE, Policy, RBAC)
