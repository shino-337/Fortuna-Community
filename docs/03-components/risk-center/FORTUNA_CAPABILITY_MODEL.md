# FORTUNA_CAPABILITY_MODEL.md

## Status
Draft v1.0

## Normative ADRs (G-SEM-01 / G-DB-01)
- [ADR-001: Capability reasoning semantics (class vs progression)](../../adr/001-capability-reasoning-semantics.md)
- [ADR-005: Single-table `pod_capabilities` decision (Phase P1)](../../adr/005-pod-capability-single-table.md)

Code helpers: `core/pkg/capability/semantics.go` (`ValidateCapabilityClass`, `ValidateProgressionState`).

## Purpose
This document defines the capability model used by Fortuna to reason about what a workload can do, has done, or can effectively achieve.

Capabilities are a core abstraction for runtime-aware risk reasoning.

They are intended to answer:

> What can this workload realistically do right now, based on both configuration and observed runtime behavior?

---

# 1. Goals

## 1.1 Primary Goals
- Define Fortuna’s capability abstraction.
- Separate:
  - declared/static capability
  - observed/runtime capability
  - effective/inferred capability
- Standardize:
  - capability initialization
  - promotion
  - confidence
  - decay
  - suppression
- Enable capability-driven:
  - risk scoring
  - attack path reasoning
  - explainability
  - runtime correlation

## 1.2 Non-Goals
This document does not define:
- full attack graph modeling
- final scoring formula
- insight workflow states

---

# 2. Why Capability Exists

Raw runtime events are evidence.
Signals are semantics.
Capabilities are operational meaning.

Example:

- Event: process read `/var/run/secrets/.../token`
- Signal: `SERVICEACCOUNT_TOKEN_READ`
- Capability: `OBSERVED_K8S_API_ACCESS`

Risk is not ultimately about “what event happened”.

Risk is about:

> what the workload is now able to do.

That is why capability is a first-class model.

---

# 3. Capability Classes

Fortuna defines three capability classes:

1. Declared Capability
2. Observed Capability
3. Effective Capability

---

# 4. Declared Capability

## 4.1 Definition
A capability inferred from static or declarative system state.

Sources include:
- pod spec
- security context
- mounts
- Linux capabilities
- RBAC permissions
- service account bindings
- workload networking / exposure metadata

Declared capability answers:

> What should this workload be able to do based on configuration?

---

## 4.2 Examples

- `CAN_ACCESS_HOST_FS`
- `CAN_USE_HOST_NETWORK`
- `CAN_REACH_K8S_API`
- `CAN_READ_K8S_SECRETS`
- `CAN_EXECUTE_AS_ROOT`
- `CAN_MODIFY_NETWORK_STACK`

---

# 5. Observed Capability

## 5.1 Definition
A capability inferred from observed runtime evidence.

Sources include:
- runtime signals
- runtime incidents
- correlated execution sequences

Observed capability answers:

> What has this workload demonstrated it can do at runtime?

---

## 5.2 Examples

- `OBSERVED_PROCESS_EXECUTION`
- `OBSERVED_EXTERNAL_EGRESS`
- `OBSERVED_K8S_API_ACCESS`
- `OBSERVED_SECRET_MATERIAL_ACCESS`
- `OBSERVED_HOST_FILE_TOUCH`
- `OBSERVED_PRIVILEGE_BOUNDARY_TOUCH`

---

# 6. Effective Capability

## 6.1 Definition
A capability derived from the union of:
- declared capability
- observed capability
- inferred reasoning

Effective capability answers:

> What can this workload effectively do from a risk perspective?

This is the capability layer the Risk Engine should primarily consume.

---

## 6.2 Examples

- `EFFECTIVE_K8S_CONTROL_PLANE_ACCESS`
- `EFFECTIVE_SECRET_ACCESS`
- `EFFECTIVE_EXTERNAL_COMMAND_AND_CONTROL`
- `EFFECTIVE_HOST_ACCESS`
- `EFFECTIVE_PRIVILEGE_EXPANSION`
- `EFFECTIVE_LATERAL_MOVEMENT_PRIMITIVE`

---

# 7. Capability Schema

```json
{
  "capability_id": "uuid",
  "asset_ref": {
    "cluster_id": "prod-cluster",
    "pod_uid": "pod-uid",
    "container_id": "containerd://abc"
  },
  "capability_type": "OBSERVED_K8S_API_ACCESS",
  "capability_class": "declared|observed|effective",
  "state": "active|suppressed|expired",
  "confidence": "low|medium|high",
  "severity_hint": "low|medium|high|critical",
  "first_seen_at": "2026-03-26T10:01:02Z",
  "last_seen_at": "2026-03-26T10:01:02Z",
  "expires_at": null,
  "evidence_refs": [
    "signal:abc",
    "incident:def"
  ],
  "derived_from": [
    "declared:CAN_REACH_K8S_API",
    "observed:OBSERVED_K8S_API_ACCESS"
  ],
  "metadata": {
    "source_reason": "serviceaccount token read + api egress"
  }
}
8. Capability Registry

All capability types must be registered.

8.1 Registry Fields

Each capability type should define:

capability_type
capability_class
domain
default_confidence
default_decay_window
severity_hint
promotable_to (optional)
suppression_eligible
risk_tags
8.2 Example Registry Entry
{
  "capability_type": "OBSERVED_K8S_API_ACCESS",
  "capability_class": "observed",
  "domain": "k8s-control-plane",
  "default_confidence": "medium",
  "default_decay_window": "24h",
  "severity_hint": "high",
  "promotable_to": [
    "EFFECTIVE_K8S_CONTROL_PLANE_ACCESS"
  ],
  "suppression_eligible": true,
  "risk_tags": [
    "credential",
    "control-plane"
  ]
}
9. Capability Domains

Recommended capability domains:

execution
filesystem
network
privilege
credentials
kubernetes
host-access
persistence
discovery
evasion
10. Capability Initialization

This section defines how capabilities are created.

10.1 Initialization Principles

Capability initialization must be:

runtime-first
idempotent
replay-safe
order-independent

This is a hard architectural requirement.

If capability creation depends on insertion order, the model is broken.

10.2 Initialization Sources

Capabilities may be initialized from:

Static sources
pod spec
RBAC graph
security context
mounts
namespace/service exposure metadata
Runtime sources
runtime signals
runtime incidents
detector outputs
10.3 Runtime-First Initialization Rule

If a runtime signal implies a capability and no capability row exists:

Fortuna must create the capability on demand.

This prevents blind spots caused by ingestion order.

Example

If:

signal = SERVICEACCOUNT_TOKEN_READ

Then:

create OBSERVED_K8S_API_ACCESS if absent

Do not skip promotion simply because the capability did not previously exist.

11. Promotion Rules

Promotion rules map lower-level evidence into stronger capability conclusions.

11.1 Promotion Types

There are three main promotion types:

A. Signal → Observed Capability

Example:

SERVICEACCOUNT_TOKEN_READ
→ OBSERVED_K8S_API_ACCESS
B. Multiple Signals → Stronger Observed Capability

Example:

SERVICEACCOUNT_TOKEN_READ
EXTERNAL_EGRESS
→ stronger confidence for OBSERVED_K8S_API_ACCESS
C. Declared + Observed → Effective Capability

Example:

CAN_REACH_K8S_API
OBSERVED_K8S_API_ACCESS
→ EFFECTIVE_K8S_CONTROL_PLANE_ACCESS
11.2 Promotion Rule Contract

Recommended promotion rule schema:

id: promote_k8s_api_access
inputs:
  any_signals:
    - SERVICEACCOUNT_TOKEN_READ
    - K8S_API_TOKEN_USE
  optional_signals:
    - EXTERNAL_EGRESS
  declared_requirements:
    - CAN_REACH_K8S_API
emit:
  observed:
    - OBSERVED_K8S_API_ACCESS
  effective:
    - EFFECTIVE_K8S_CONTROL_PLANE_ACCESS
confidence:
  base: medium
  boost_if:
    optional_signals_present: high
11.3 Promotion Design Rules
Rules
Promotions must be deterministic.
Promotions must preserve evidence references.
Promotions must be idempotent.
Promotions must support re-evaluation if new evidence arrives.
Promotions must not assume event ordering.
12. Example Capability Mappings
12.1 Credential / K8s Access
Evidence	Capability
SERVICEACCOUNT_TOKEN_READ	OBSERVED_K8S_API_ACCESS
K8S_API_TOKEN_USE	OBSERVED_K8S_API_ACCESS
CAN_REACH_K8S_API + observed token access	EFFECTIVE_K8S_CONTROL_PLANE_ACCESS
12.2 Host Access
Evidence	Capability
HOST_PATH_ACCESS	OBSERVED_HOST_FILE_TOUCH
declared hostPath mount	CAN_ACCESS_HOST_FS
hostPath mount + observed host access	EFFECTIVE_HOST_ACCESS
12.3 Execution / C2
Evidence	Capability
INTERACTIVE_SHELL_EXEC	OBSERVED_PROCESS_EXECUTION
REMOTE_PAYLOAD_FETCH	OBSERVED_EXTERNAL_EGRESS
shell + remote fetch + tmp exec	EFFECTIVE_EXTERNAL_COMMAND_AND_CONTROL
12.4 Privilege / Escape
Evidence	Capability
HOST_NAMESPACE_ACCESS	OBSERVED_PRIVILEGE_BOUNDARY_TOUCH
declared privileged container	CAN_TOUCH_PRIVILEGE_BOUNDARY
privileged + runtime boundary touch	EFFECTIVE_PRIVILEGE_EXPANSION
13. Confidence Model

Capabilities must have confidence.

13.1 Confidence Sources

Confidence may be influenced by:

source quality
detector confidence
number of corroborating signals
correlation depth
declared + observed agreement
13.2 Recommended Confidence Rules
Low
single weak runtime hint
incomplete metadata
weak source context
Medium
one strong signal
clear static declaration
limited corroboration
High
multi-signal corroboration
incident-backed evidence
declared + observed alignment
14. Decay Model

Capabilities should not live forever.

If they do, the platform becomes a graveyard of stale fear.

14.1 Decay Principles

Capabilities must support:

first_seen_at
last_seen_at
expires_at
periodic decay evaluation
14.2 Decay by Capability Class
Declared Capability

Usually tied to inventory/config state.

Behavior
persists until static state changes
expires when asset/config relationship disappears
Observed Capability

Tied to runtime evidence.

Behavior
decays after configurable inactivity window
Effective Capability

Derived from current declared + observed state.

Behavior
recomputed whenever dependencies change
14.3 Recommended Decay Windows
Capability Class	Typical Window
Declared	state-driven, not time-driven
Observed	24h to 7d depending on type
Effective	recomputed, not independently aged
Examples
OBSERVED_EXTERNAL_EGRESS: 24h
OBSERVED_HOST_FILE_TOUCH: 72h
OBSERVED_K8S_API_ACCESS: 24h
OBSERVED_SECRET_MATERIAL_ACCESS: 72h
15. Suppression Model

Some capabilities should be suppressible.

Example:

known operational shell in admin namespace
expected hostPath access by storage daemon
expected privileged workload behavior
15.1 Suppression Rules

Suppression may apply by:

asset
namespace
cluster
capability_type
time window
owner-approved exception
15.2 Suppression Effects

Suppression should not delete capability records.

Instead it should:

set state = suppressed
preserve evidence
preserve auditability
reduce UI noise
optionally reduce score contribution
15.3 Suppression Metadata

Recommended fields:

suppressed_by
suppressed_at
suppression_reason
suppression_scope
suppression_expires_at
16. Capability Persistence Model

Recommended tables:

declared_capabilities
observed_capabilities
effective_capabilities
capability_suppressions

Alternative:
Single table with capability_class.

17. Relationship to Risk Engine

Capabilities are one of the primary inputs into risk evaluation.

Risk rules should prefer capability predicates over raw runtime event predicates wherever possible.

Good
effective_capabilities contains EFFECTIVE_HOST_ACCESS
Bad
raw_event.file.path starts with /host

The first is explainable and stable.
The second is brittle and low-level.

18. Relationship to Explainability

Capabilities are highly explainable because they connect:

evidence
behavior
operational meaning
risk

Example explanation:

This workload is considered capable of effective Kubernetes API access because it:

is configured to reach the API
read its service account token at runtime
subsequently exhibited API-related behavior

This is far more useful than showing three random events.

19. Summary

Fortuna capabilities are the bridge between:

raw evidence
runtime semantics
real attacker-operational meaning

They must be:

initialized safely
promoted deterministically
decayed responsibly
suppressed auditably
consumed consistently by the risk engine

If signals are the language of runtime behavior, capabilities are the language of runtime power.