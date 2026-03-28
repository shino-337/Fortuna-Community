# FORTUNA_RUNTIME_RISK_ENGINE_ARCHITECTURE.md

## Status
Draft v1.0

## Purpose
This document defines the architecture of Fortuna’s runtime-aware risk engine.

It describes:

- the full runtime → risk pipeline
- unified security state projection
- runtime-aware insight model
- scoring model
- explainability flow

This is the target architecture for evolving Fortuna from a telemetry processor into a runtime-aware security risk platform.

---

# 1. Problem Statement

Runtime telemetry alone does not create usable security risk intelligence.

Without structure, runtime systems degrade into:

- noisy event feeds
- shallow alerting
- hard-to-explain scores
- low-confidence findings
- poor prioritization

Fortuna’s runtime-aware risk engine exists to convert runtime evidence into:

- explainable security conclusions
- workload-level risk state
- operational prioritization
- attack-relevant reasoning

---

# 2. Architecture Goals

## 2.1 Primary Goals
- Ingest runtime telemetry from heterogeneous sources.
- Normalize runtime evidence into stable semantics.
- Correlate runtime behavior into security-relevant state.
- Combine runtime and static context into workload risk.
- Produce explainable, stateful insights.
- Prioritize assets and findings using factorized scoring.

## 2.2 Non-Goals
This architecture does not attempt to be:
- a full SIEM
- a packet analytics platform
- a generic observability pipeline

It is specifically a **runtime-aware security risk engine**.

---

# 3. High-Level Architecture

```mermaid
flowchart TD
  subgraph Sensors["Sensors / Collectors"]
    F["Falco"] --> AF["Falco Adapter"]
    E["eBPF / LSM"] --> AE["eBPF Adapter"]
    T["Tetragon / Tracee"] --> AT["Runtime Adapter"]
  end

  subgraph Agent["Fortuna Agent"]
    AF --> DTO["Canonical RuntimeEventDTO"]
    AE --> DTO
    AT --> DTO
    DTO --> ING["Ingest Client\nretry / batch / backpressure"]
  end

  subgraph Core["Fortuna Core"]
    ING --> API["Runtime Ingest API\nvalidate / persist"]
    API --> RE[(runtime_events)]

    RE --> BF["Behavior Fact Extractor"]
    BF --> RBF[(runtime_behavior_facts)]

    RBF --> REP["REP v2\nNormalization + Signal Synthesis + Correlation"]
    REP --> RS[(runtime_signals)]
    REP --> RINC[(runtime_incidents)]

    RS --> CAP["Capability Engine\ninit / promote / suppress / decay"]
    RINC --> CAP
    CAP --> CAPDB[(effective_capabilities)]

    CAPDB --> STATE["Security State Projector"]
    RS --> STATE
    RINC --> STATE
    STATE --> ASS[(asset_security_state)]

    ASS --> RISK["Risk Engine\nCEL + risk conditions + toxic combinations"]
    RISK --> INS["Insight Manager\nstateful explainable findings"]
    INS --> INSDB[(insights)]

    INSDB --> SCORE["Scorer v3\nfactorized score + priority + confidence"]
    SCORE --> SCOREDB[(risk_scores)]
  end

  subgraph OtherState["Other Security State"]
    INV[(inventory / assets / RBAC / exposure)]
    SBOM[(SBOM / packages / vulns)]
    INV --> ASS
    SBOM --> ASS
  end

  subgraph UI["Dashboard / API"]
    RE --> POD1["Pod Runtime Events"]
    RS --> POD2["Pod Runtime Signals"]
    CAPDB --> POD3["Pod Capabilities"]
    INSDB --> POD4["Pod Insights"]
    SCOREDB --> RISKUI["Risk Overview"]
    RS --> COV["Coverage View\nMITRE x source x signal"]
    RINC --> LIVE["Live Runtime Timeline"]
  end
4. Runtime-to-Risk Pipeline

The pipeline is intentionally layered.

This separation is not cosmetic. It is what prevents the system from collapsing into rule spaghetti.

4.1 Layer 1 — Runtime Evidence
Inputs
Falco
eBPF / LSM
Tetragon
Tracee
future runtime sensors
Output
runtime_events
Purpose

Persist immutable source-independent runtime evidence.

Defined in:

FORTUNA_RUNTIME_EVENT_MODEL.md
4.2 Layer 2 — Behavior Semantics
Inputs
runtime_events
Outputs
runtime_behavior_facts
runtime_signals
runtime_incidents
Purpose

Convert telemetry into security semantics.

Defined in:

FORTUNA_RUNTIME_SIGNAL_MODEL.md
4.3 Layer 3 — Capability Reasoning
Inputs
runtime signals
runtime incidents
static inventory / RBAC / exposure state
Outputs
declared capabilities
observed capabilities
effective capabilities
Purpose

Express attacker-relevant power and reach.

Defined in:

FORTUNA_CAPABILITY_MODEL.md
4.4 Layer 4 — Unified Security State
Inputs
inventory
exposure
RBAC
SBOM / vulnerabilities
runtime signals
runtime incidents
capabilities
Output
asset_security_state
Purpose

Provide a single coherent risk context for each asset.

This is the primary input to the Risk Engine.

4.5 Layer 5 — Risk Evaluation
Inputs
asset_security_state
Outputs
insights
risk_scores
Purpose

Evaluate risk conditions and prioritize them.

5. Asset Security State

The asset_security_state model is the backbone of the runtime-aware risk engine.

Without it, risk evaluation becomes a brittle collection of joins and ad-hoc rules.

5.1 Purpose

For each asset (primarily pod/workload), Fortuna should maintain a unified risk context snapshot.

It should answer:

What is this thing?
How exposed is it?
How privileged is it?
What software risk does it carry?
What runtime behavior has it exhibited?
What capabilities does it effectively have?
How dangerous is it now?
5.2 Canonical Asset Security State Schema
{
  "asset_id": "pod:payments/api-123",
  "asset_type": "pod",
  "cluster_id": "prod-cluster",
  "identity": {
    "namespace": "payments",
    "pod_uid": "pod-uid",
    "workload_kind": "Deployment",
    "workload_name": "payments-api",
    "service_account": "payments-sa",
    "owner": "payments-team",
    "environment": "prod"
  },
  "exposure": {
    "internet_exposed": true,
    "service_type": "LoadBalancer",
    "public_ingress": true,
    "external_reachability": "high"
  },
  "privilege": {
    "run_as_root": true,
    "privileged": false,
    "host_network": false,
    "host_pid": false,
    "host_ipc": false,
    "host_path_mount": true,
    "linux_capabilities": ["NET_RAW"]
  },
  "software": {
    "critical_vulns": 2,
    "high_vulns": 5,
    "known_exploitable_vulns": 1,
    "fix_available_vulns": 3
  },
  "runtime": {
    "recent_signals": [
      "SERVICEACCOUNT_TOKEN_READ",
      "EXTERNAL_EGRESS"
    ],
    "recent_incidents": [
      "POST_EXPLOIT_EXEC_CHAIN"
    ],
    "runtime_confidence": "high",
    "last_runtime_activity_at": "2026-03-26T10:07:12Z"
  },
  "capabilities": {
    "declared": [
      "CAN_REACH_K8S_API"
    ],
    "observed": [
      "OBSERVED_K8S_API_ACCESS"
    ],
    "effective": [
      "EFFECTIVE_K8S_CONTROL_PLANE_ACCESS"
    ]
  },
  "blast_radius": {
    "can_read_secrets": true,
    "can_create_pods": false,
    "can_access_host": false,
    "lateral_movement_potential": "medium"
  },
  "freshness": {
    "last_state_update_at": "2026-03-26T10:07:12Z"
  }
}
5.3 Design Rules
Rules
The state must be recomputable.
The state must not depend on hidden mutable side effects.
The state must support partial updates.
The state must be explainable.
6. Risk Engine

The Risk Engine evaluates runtime-aware security conditions over asset_security_state.

6.1 Core Principle

The Risk Engine should evaluate:

meaningful security state

—not raw telemetry.

This means rules should primarily consume:

exposure context
privilege context
software risk
runtime signals/incidents
effective capabilities
blast radius

—not arbitrary low-level events.

6.2 Risk Rule Types

Fortuna risk rules should be grouped into three classes:

6.2.1 Static Risk Rules

Derived from static state only.

Example
internet exposed + known exploitable vulnerability
privileged + hostPath mount
service account can read secrets
6.2.2 Runtime Risk Rules

Derived from runtime state only.

Example
credential access observed
suspicious shell execution observed
post-exploitation execution chain observed
6.2.3 Combined Risk Rules

Derived from static + runtime context.

Example
internet exposed + credential access + external egress
root workload + hostPath mount + host path access observed
KEV present + suspicious execution chain observed

These are generally the highest-value rules.

6.3 Rule Engine Technology

CEL is appropriate for:

deterministic predicates
explainable conditions
threshold logic
boolean composition

CEL is not the right place for:

temporal sequence detection
correlation windows
graph traversal logic
probabilistic scoring

Those must happen before CEL.

6.4 Example Rule
id: runtime_credential_access_on_exposed_workload
severity: high
when: |
  exposure.internet_exposed == true &&
  runtime.recent_signals.exists(s, s == "SERVICEACCOUNT_TOKEN_READ") &&
  capabilities.effective.exists(c, c == "EFFECTIVE_K8S_CONTROL_PLANE_ACCESS")
insight_type: RUNTIME_CREDENTIAL_ACCESS_ON_EXPOSED_WORKLOAD
summary: Service account token access observed on an externally exposed workload.
7. Insight Model

Insights are the primary user-facing security conclusions produced by the runtime-aware risk engine.

They are not raw alerts.
They are not just rule matches.

An insight is a stateful, explainable, triageable risk conclusion.

7.1 Insight Schema
{
  "insight_id": "uuid",
  "asset_id": "pod:payments/api-123",
  "insight_type": "RUNTIME_CREDENTIAL_ACCESS_ON_EXPOSED_WORKLOAD",
  "severity": "high",
  "confidence": "high",
  "status": "open",
  "opened_at": "2026-03-26T10:04:00Z",
  "last_seen_at": "2026-03-26T10:07:12Z",
  "resolved_at": null,
  "rule_id": "runtime_credential_access_exposed",
  "summary": "Service account token access was observed on an externally exposed workload.",
  "why_risky": [
    "Workload is externally reachable",
    "Service account token file was accessed at runtime",
    "The workload has effective Kubernetes API access"
  ],
  "evidence_refs": [
    "event:abc",
    "signal:def",
    "incident:ghi"
  ],
  "capability_refs": [
    "EFFECTIVE_K8S_CONTROL_PLANE_ACCESS"
  ],
  "score_factors": {
    "runtime_threat": 85,
    "blast_radius": 70,
    "confidence": 90
  },
  "suppressed": false
}
7.2 Insight Lifecycle

Insights should support the following states:

open
acknowledged
suppressed
resolved
reopened
7.3 Insight Design Rules
Rules
Insights must be evidence-backed.
Insights must preserve why they exist.
Insights must support recomputation and reopening.
Insights must be explainable without reading raw logs.
8. Scorer Model

The scorer exists to prioritize what matters most.

If it only emits one opaque number, it is decorative nonsense.

8.1 Scoring Philosophy

Risk should be:

factorized
explainable
freshness-aware
confidence-aware
8.2 Recommended Score Factors

Fortuna should score at least these dimensions:

1. Exposure

How reachable is the workload?

2. Exploitability

How realistically exploitable is the workload?

3. Privilege

How dangerous is the workload if compromised?

4. Runtime Threat

How suspicious is recent observed behavior?

5. Blast Radius

How much downstream damage is possible?

6. Confidence

How strongly supported is the conclusion?

Optional:

7. Criticality

How important is this asset to the business/platform?

8.3 Example Factorized Score Model
overall_risk = weighted(
  exposure,
  exploitability,
  privilege,
  runtime_threat,
  blast_radius,
  confidence,
  criticality
)
8.4 Example Interpretation
High Runtime Threat, Low Exposure

A suspicious internal-only workload may still be urgent, but not necessarily platform-critical.

Medium Runtime Threat, High Blast Radius

A less noisy but high-privilege workload may deserve higher priority.

High Exposure + Credential Access + API Reach

This should heavily boost overall risk.

8.5 Suggested Score Boosters

Boosters are important for toxic combinations.

Examples:

internet exposed + known exploitable vulnerability
serviceaccount token read + external egress
root + hostPath mount + host file access
shell execution + payload fetch + tmp binary exec

These should not be treated as simple additive noise.

They should materially boost priority.

8.6 Decay / Freshness

Runtime-heavy scores must decay.

Recommended:

recent runtime incidents carry more weight
stale runtime-only signals should lose impact over time
score must be recomputed as state changes

Otherwise the system becomes permanently red and permanently ignored.

9. Explainability Flow

Explainability is not a nice-to-have.
It is the difference between “security theater” and an actually usable platform.

9.1 Explainability Objective

Every runtime-aware insight and risk score should answer:

Why is this risky?
What evidence supports it?
What changed recently?
What can this workload effectively do?
Why is this prioritized above others?
9.2 Explainability Pipeline
flowchart TD
  A["runtime_events"] --> B["behavior_facts"]
  B --> C["runtime_signals / incidents"]
  C --> D["capabilities"]
  D --> E["asset_security_state"]
  E --> F["risk rule matched"]
  F --> G["insight created"]
  G --> H["score factors composed"]
  H --> I["explanation rendered"]
  9.3 Explanation Layers
Layer 1 — Evidence

Raw events and extracted facts.

Layer 2 — Semantics

Signals and incidents.

Layer 3 — Meaning

Capabilities and blast radius.

Layer 4 — Risk Conclusion

Matched risk conditions and insight summary.

Layer 5 — Priority

Score factors and priority rationale.

9.4 Example Explanation

This workload is high risk because:

it is externally exposed
it accessed its Kubernetes service account token at runtime
it subsequently demonstrated effective Kubernetes API access
it has meaningful blast radius through secret access permissions

This is far more useful than:

“Falco rule matched. Severity high.”

10. UI / API Consumption Model

The runtime-aware risk engine should expose data in layered views.

10.1 Pod Detail View

Should clearly separate:

runtime events
runtime signals
runtime incidents
capabilities
insights

These should not be collapsed into one undifferentiated stream.

10.2 Coverage View

Should support:

signal distribution by domain
MITRE tactic coverage
source coverage
detector coverage
10.3 Risk Overview

Should support:

top risky workloads
runtime-active risky workloads
high-confidence runtime compromise candidates
toxic combination hotspots
11. Recommended Persistence Objects

Recommended materialized objects / tables:

runtime_events
runtime_behavior_facts
runtime_signals
runtime_incidents
declared_capabilities
observed_capabilities
effective_capabilities
asset_security_state
insights
risk_scores
12. Design Principles Summary

The runtime-aware risk engine must be:

layered
explainable
replay-safe
idempotent
runtime-first
state-driven
confidence-aware
freshness-aware

If Fortuna skips these principles, it will become a noisy detector warehouse.

If Fortuna follows them, it can become a genuinely useful runtime-aware security platform.

13. Final Summary

Fortuna’s runtime-aware risk engine should transform:

runtime evidence

into:

explainable, prioritized, workload-level security risk

It does this by layering:

canonical runtime events
behavior facts
runtime signals and incidents
capability reasoning
unified asset security state
risk conditions and insights
factorized scoring and explainability

That is the architecture that scales.

14. Implementation Checkpoint (2026-03-26)

Implemented in code (minimal rollout-first):

- Layer 2 tables/models: `runtime_behavior_facts`, `runtime_incidents`.
- Layer 4 table/model: `asset_security_state` + projector.
- Compatibility bridge: `object.fortuna.*` now derived from `asset_security_state` (with fallback for pre-migration/tests).
- CEL input v2 scaffold: `object.securityState.*` is projected from `asset_security_state` (booleans + runtime_signals_by_type + effective_capabilities); legacy `object.fortuna.*` still supported.
- Rule loading unified to YAML-only source (`/core/rules`, optionally via `FORTUNA_RULES_DIR`); no hardcoded-rule fallback path.
- REP-A extractor is active (`runtime_events -> runtime_behavior_facts`).
- REP-B now has persist path (facts -> runtime_signals) với anti-double-count; vẫn giữ compare logging để quan sát mismatch.
- REP-C minimal correlator is active: `RECON_BURST` stateful incident persisted to `runtime_incidents`.
- REP-C also includes `POST_EXPLOIT_EXEC_CHAIN` stateful incident (10m execution-chain correlator).
- REP-C also includes `EXFIL_LIKE_SEQUENCE` (5m ordered token-read -> external-connect chain, minimal heuristic).
- Runtime-first capability init in REP flow (initialize then promote).
- Read APIs added: `/api/v2/runtime/pods/:uid/security-state`, `/facts`, `/incidents`.

Not yet completed:

- Additional REP-C detectors beyond current set (`RECON_BURST`, `POST_EXPLOIT_EXEC_CHAIN`, `EXFIL_LIKE_SEQUENCE`) và fine-grained suppression/tuning hardening (cooldown cơ bản đã có).
- Canonical RuntimeEventDTO v2 ingest contract cut-over.
- Risk scorer v2 full cut-over to `asset_security_state` dimensions.
- Explainability refs end-to-end (`event -> fact -> signal -> incident -> capability -> insight`).