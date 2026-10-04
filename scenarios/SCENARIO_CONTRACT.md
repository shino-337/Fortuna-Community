# Fortuna Security Scenario Contract

Fortuna scenarios are **security regression fixtures**, not arbitrary Kubernetes demos. Each scenario must make a claim that can be deployed, observed in Fortuna, and independently verified. The goal is to prove that Fortuna detects risky conditions **and** that incomplete chains do not become false attack paths.

## Scenario lifecycle

```text
Manifest
  ↓
Cluster deployment
  ↓
Fortuna inventory / runtime ingestion
  ↓
Detection / correlation
  ↓
Attack path + risk evidence
  ↓
Verification (PASS / FAIL / WARN)
  ↓
Teardown
```

## Required properties

Every maintained scenario must define:

1. **Intent**: the security condition being exercised.
2. **Preconditions**: Kubernetes features, Fortuna services, and optional runtime dependencies.
3. **Expected detection**: the attack-path, capability, runtime signal, or risk behavior that Fortuna should produce.
4. **Negative boundary**: what Fortuna should *not* report when the scenario is intentionally incomplete or noisy.
5. **Evidence**: API response, path class, technique ID, risk factor, or other observable result.
6. **Teardown**: resources that must be removed, including cluster-scoped objects.

## Scenario matrix

| ID | Security claim | Required evidence | Negative boundary | Optional evidence | Confidence |
|---|---|---|---|---|---|
| S1 | HostPath + privileged RBAC creates an escape/escalation path | Pod-specific attack path + escape classification | Missing dangerous permission or unrelated workload | Runtime/MITRE correlation | High |
| S2 | RBAC-only escalation is distinguishable from an escape path | Pod-specific RBAC path without `ESCAPE_PATH` | RBAC object exists without exploitable permission | Runtime correlation | High |
| S3 | Discovery noise should not create an inflated attack chain | Scenario is present and risk remains bounded | Noise without privilege impact | Runtime correlation precision | Medium |
| S4 | Repeated ServiceAccount API activity can support token-reuse correlation | Pod-specific path/evidence | Token mount without impact path | `SA_TOKEN_REUSE` runtime technique | Medium |
| S5 | Broken/incomplete chains must not be promoted as complete escalation | No false positive escalation chain | Must not emit complete escalation classification | Capability-validation diagnostics | High |

## Evidence model

A detection without traceable evidence is not considered validated. Each result should be explainable through:

| Category | Fields |
|---|---|
| Identity | namespace, pod, ServiceAccount, workload owner |
| Attack path | source workload, relationship edges, capability or permission, impacting condition |
| Risk justification | exploitability, impact, reachability, risk classification |

Principles:

- Prefer workload-specific evidence over cluster-wide existence checks.
- Prefer complete attack chains over isolated risky objects.
- Preserve negative boundaries to prevent false positives.

## Verification contract

The verifier must distinguish between:

- **PASS**: required evidence was observed and the security assertion was validated.
- **FAIL**: required evidence was missing or inconsistent, or the Fortuna API could not be queried.
- **WARN**: an optional runtime dependency was unavailable, so a secondary assertion could not be evaluated.

A warning must never be presented as a successful verification of the optional capability.

## Severity and safety

Scenarios may intentionally create privileged Kubernetes objects. They must:

- use a dedicated namespace where possible;
- use deterministic resource names;
- clearly identify cluster-scoped RBAC objects;
- never contain real credentials, tokens, registry credentials, or external production endpoints;
- document any host-level or runtime prerequisites;
- provide a teardown command.

Run these scenarios only on an isolated test cluster. Do not apply them to a production cluster.

## Adding a scenario

Before a scenario counts as maintained:

- the security claim and its negative boundary are in the matrix above;
- evidence explains why the result was produced;
- runtime-dependent assertions degrade to WARN when runtime telemetry is unavailable;
- teardown removes all cluster-scoped objects.

A new scenario should add or update:

```text
scenarios/<scenario>.yaml
scenarios/README.md
scenarios/SCENARIO_CONTRACT.md (matrix)
scripts/verify-k8s-e2e.sh
```

If the scenario requires a new external dependency, document it explicitly rather than silently weakening the verifier.
