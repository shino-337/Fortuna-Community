# Security Scenario Quality Gate

A scenario is **review-ready** only when it defines the security claim, the evidence required to prove it, and the boundary that must prevent a false positive.

## Required contract

Every scenario must document:

1. **Setup** — the Kubernetes objects and assumptions required to reproduce it.
2. **Positive assertion** — the security condition Fortuna is expected to correlate.
3. **Evidence** — workload/identity, permission or capability, reachability, and impact evidence where applicable.
4. **Negative boundary** — a deliberately incomplete or lower-risk condition that must not produce the same attack-path classification.
5. **Teardown** — all objects created by the scenario must be removable without leaving cluster-scoped residue.

## Result semantics

- **PASS** — the expected assertion and required evidence were observed.
- **FAIL** — the expected assertion was not observed, evidence was incomplete/inconsistent, or verification could not complete.
- **WARN** — an optional dependency (for example runtime telemetry) was unavailable; WARN is not successful validation.

## Review rules

- A Kubernetes object existing by itself is not an attack path.
- Cluster-wide RBAC scope must be distinguished from namespace-local scope.
- Wildcard permissions must be correlated with effective scope and workload identity.
- Risk severity must be justified by exploitability, impact, and reachability rather than by a single risky field.
- Scenario fixtures must not claim runtime detection has passed unless Fortuna's actual collector/correlation path was exercised.
