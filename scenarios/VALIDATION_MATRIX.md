# Fortuna Security Scenario Validation Matrix

This document defines the validation boundary for Kubernetes security regression scenarios.

The objective is not only to prove that Fortuna detects risky conditions, but also to prove that incomplete chains do not become false attack paths.

## Validation model

```text
Scenario setup
      |
      v
Fortuna inventory/runtime ingestion
      |
      v
Attack-path correlation
      |
      v
Evidence validation
      |
      v
PASS / FAIL / WARN
```

## Scenario coverage

| ID | Security claim | Required evidence | Negative boundary | Confidence |
|---|---|---|---|---|
| S1 | HostPath and privileged RBAC can create an escalation path | Workload identity, service account, RBAC binding, pod-specific escape path | Missing dangerous permission or unrelated workload | High |
| S2 | RBAC escalation can be detected without classifying every RBAC issue as escape | Workload-specific RBAC path without escape classification | RBAC object exists without exploitable permission | High |
| S3 | Discovery activity should not create inflated risk | Bounded risk/correlation evidence | Noise without privilege impact | Medium |
| S4 | ServiceAccount token activity can support lateral movement correlation | Workload identity, token evidence, reachable target or runtime signal | Token mount without impact path | Medium |
| S5 | Broken chains must not become complete escalation paths | Missing-edge validation evidence | Must not emit complete escalation classification | High |

## Review criteria

Before marking a scenario production-ready:

- The security claim is explicitly documented.
- Evidence explains why the result was produced.
- Negative conditions are tested where applicable.
- Runtime-dependent assertions degrade to WARN when runtime telemetry is unavailable.
- Teardown removes all cluster-scoped objects.
