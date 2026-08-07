# Fortuna Security Scenario Contract

Fortuna scenarios are **security regression fixtures**, not arbitrary Kubernetes demos. Each scenario must make a claim that can be deployed, observed in Fortuna, and independently verified.

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
Verification
  ↓
Teardown
```

## Required properties

Every maintained scenario must define:

1. **Intent** — the security condition being exercised.
2. **Preconditions** — Kubernetes features, Fortuna services, and optional runtime dependencies.
3. **Expected detection** — the attack-path, capability, runtime signal, or risk behavior that Fortuna should produce.
4. **Negative boundary** — what Fortuna should *not* report when the scenario is intentionally incomplete or noisy.
5. **Evidence** — API response, path class, technique ID, risk factor, or other observable result.
6. **Teardown** — resources that must be removed, including cluster-scoped objects.

## Severity and safety

Scenarios may intentionally create privileged Kubernetes objects. They must:

- use a dedicated namespace where possible;
- use deterministic resource names;
- clearly identify cluster-scoped RBAC objects;
- never contain real credentials, tokens, registry credentials, or external production endpoints;
- document any host-level or runtime prerequisites;
- provide a teardown command.

Run these scenarios only on an isolated test cluster. Do not apply them to a production cluster.

## Verification contract

The verifier should distinguish between:

- **PASS** — required evidence was observed;
- **FAIL** — a required assertion was not observed or the Fortuna API could not be queried;
- **WARN** — an optional runtime dependency was unavailable, so a secondary assertion could not be evaluated.

A warning must never be presented as a successful verification of the optional capability.

## Scenario matrix

| ID | Security claim | Required evidence | Optional evidence |
|---|---|---|---|
| S1 | HostPath + privileged RBAC creates an escape/escalation path | Pod-specific attack path + escape classification | Runtime/MITRE correlation |
| S2 | RBAC-only escalation is distinguishable from an escape path | Pod-specific RBAC path without `ESCAPE_PATH` | Runtime correlation |
| S3 | Discovery noise should not create an inflated attack chain | Scenario is present and risk remains bounded | Runtime correlation precision |
| S4 | Repeated ServiceAccount API activity can support token-reuse correlation | Pod-specific path/evidence | `SA_TOKEN_REUSE` runtime technique |
| S5 | Broken/incomplete chains must not be promoted as complete escalation | No false positive escalation chain | Capability-validation diagnostics |

## Adding a scenario

A new scenario should add or update:

```text
scenarios/<scenario>.yaml
scenarios/README.md
scripts/verify-k8s-e2e.sh
```

If the scenario requires a new external dependency, document it explicitly rather than silently weakening the verifier.
