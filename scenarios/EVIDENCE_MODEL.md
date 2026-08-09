# Fortuna Scenario Evidence Model

Security scenarios must produce explainable evidence. A detection without traceable evidence is not considered validated.

## Minimum evidence categories

### Identity

```text
namespace
pod
service account
workload owner
```

### Attack path

```text
source workload
relationship edges
capability or permission
impacting condition
```

### Risk justification

```text
exploitability
impact
reachability
risk classification
```

## Result semantics

### PASS

Required evidence was observed and the security assertion was validated.

### FAIL

Required evidence was missing, inconsistent, or the Fortuna API could not complete verification.

### WARN

An optional dependency such as runtime telemetry was unavailable. WARN must never represent successful validation of the missing capability.

## Evidence principles

- Prefer workload-specific evidence over cluster-wide existence checks.
- Prefer complete attack chains over isolated risky objects.
- Preserve negative boundaries to prevent false positives.
