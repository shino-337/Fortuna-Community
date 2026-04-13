# Pod Capability Engine (PCE) Component

## Overview

The Pod Capability Engine (PCE) answers one question: **"If an attacker compromises this Pod, what can they do next in the cluster?"**

PCE performs **static analysis** of pod specifications and runtime metadata to identify offensive capabilities. It is deterministic, explainable, and rule-based — no ML or fuzzy scoring. Each capability maps to MITRE ATT&CK for Containers/Kubernetes.

**Key Design Principles:**
- Static analysis of YAML/runtime metadata
- Every capability has clear trigger conditions + MITRE ATT&CK mapping
- Input for Attack Graph Phase 2
- Phase 1: "What access does attacker have?" / Phase 2: "What will attacker do?"

## Architecture

### Capability Model

Capability ID naming convention: `<DOMAIN>_<VECTOR>_<SCOPE>[_<QUALIFIER>]`

| Domain | Description |
|--------|-------------|
| **ESC** | Container → Node escape |
| **NET** | Network pivot / sniffing |
| **ID** | Identity & credential abuse |
| **API** | Kubernetes API abuse |
| **CTRL** | Control-plane impact |
| **FS** | File system / host filesystem access |

### Capability ID Reference

| Capability ID | Severity | Confidence | MITRE | Description |
|--------------|----------|------------|-------|-------------|
| `ESC_PRIV_POD` | CRITICAL | 0.7 | T1611 | Privileged container enabled |
| `ESC_HOSTPID_POD` | HIGH | 0.6 | T1611, T1068 | hostPID enabled — host process namespace access |
| `ESC_HOSTIPC_POD` | HIGH | 0.6 | T1611 | hostIPC enabled — host IPC namespace access |
| `ESC_HOSTPATH_NODE` | CRITICAL | 0.8 | T1611 | hostPath mount — node filesystem access |
| `ESC_RUNTIME_PROC_ROOT` | CRITICAL | 0.9 | T1611 | Confirmed /proc/1/root access (proc root pivot) |
| `ESC_RUNTIME_PROBE` | HIGH | 0.5 | T1611 | Static risk + runtime signal suggesting escape capability |
| `ESC_RUNTIME_ACTIVE` | CRITICAL | 0.95 | T1611 | Active runtime escape confirmed |
| `NET_HOSTNETWORK` | MEDIUM | 0.4 | T1046, T1595 | hostNetwork — host network access |
| `ID_TOKEN_POD` | MEDIUM | 0.5 | T1528 | Can steal ServiceAccount token |
| `API_RBAC_WRITE_CLUSTER` | HIGH | 0.7 | T1609 | RBAC write access to K8s API |
| `CTRL_CONTROL_PLANE_POD` | HIGH | 0.7 | T1496 | Pod running in kube-system |

### Capability States

| State | Meaning |
|-------|---------|
| `detected` | Static analysis found the capability |
| `confirmed` | Runtime signal confirms the capability is exercisable |
| `exploited` | Active exploitation detected |
| `chained` | Part of an attack chain with other capabilities |

### CapabilityStateController (CSC)

- Single owner responsible for state transitions
- Uses `SUM(count)` for `MinOccurrences` check on runtime signals
- Promotes capabilities based on runtime evidence

## Processing Flow

### Static Analysis (Pod Spec → Capabilities)

```
Pod YAML / Runtime Metadata
    ↓
PCE Evaluator (rule-based)
    ↓
For each rule: check trigger condition
    ↓
If match → emit capability with evidence
    ↓
Dedup by capability_id
    ↓
Persist to pod_capabilities
    ↓
Emit event: fortuna.pod.capability.created
```

**Trigger:** `EvaluateAndUpsertPod(ctx, db, pod, expectedSpecHash)` — called after pod sync. Uses `specHash` to skip re-evaluation when pod spec hasn't changed.

### Runtime Detection (Container Escape, REP)

Three-layer detection model:

| Layer | Source | Purpose |
|-------|--------|---------|
| **Kernel/eBPF** | syscall, audit, seccomp notify | Low-level escape probes |
| **Agent Sensors** | `/proc` scanning, namespace checks | Container escape indicators |
| **CSC (Core)** | Capability promotion rules | Promote `detected` → `confirmed` → `exploited` |

**REP (Runtime Escape Probe Detection)** focuses on detecting pods affected by runtime CVEs (runc/containerd) even without SBOM CVE matches — by monitoring runtime behavior + syscall + namespace interaction.

### MITRE ATT&CK Mapping

| MITRE ID | Tactic | Technique | Capabilities |
|----------|--------|-----------|-------------|
| T1611 | Privilege Escalation | Escape to Host | ESC_PRIV_POD, ESC_HOSTPID_POD, ESC_HOSTPATH_NODE, ESC_RUNTIME_* |
| T1611.001 | Privilege Escalation | Exploit Container Runtime | ESC_RUNTIME_PROBE, ESC_RUNTIME_ACTIVE |
| T1611.002 | Privilege Escalation | Abuse Privileged Container | ESC_PRIV_POD |
| T1610 | Defense Evasion | Modify Container Runtime | ESC_RUNTIME_PROC_ROOT |
| T1068 | Privilege Escalation | Exploitation for PrivEsc | ESC_HOSTPID_POD |
| T1046 | Discovery | Network Service Discovery | NET_HOSTNETWORK |
| T1528 | Credential Access | Steal Application Access Token | ID_TOKEN_POD |
| T1609 | Execution | Container Administration Command | API_RBAC_WRITE_CLUSTER |
| T1496 | Impact | Resource Hijacking | CTRL_CONTROL_PLANE_POD |

## Technical Details

### Database Schema

**Table: `pod_capabilities`**

| Column | Type | Description |
|--------|------|-------------|
| id | uuid | Primary key |
| pod_uid | string | Pod identifier |
| capability_id | string | Standardized capability ID |
| group | string | Domain group (ESC, NET, etc.) |
| severity | string | CRITICAL/HIGH/MEDIUM/LOW |
| confidence | float | 0.0–1.0 |
| state | string | detected/confirmed/exploited/chained |
| evidence | jsonb | Trigger evidence (field, value) |
| mitre | text[] | MITRE ATT&CK technique IDs |
| created_at | timestamp | First detection time |

**Table: `capability_metadata`** — definition catalog with extended MITRE fields (migrations 047, 050, 060, 061):

| Column | Description |
|--------|-------------|
| capability_id | Primary key |
| name, summary, full_description | Human-readable info |
| domain, category | Classification |
| severity_base, confidence_base | Base scoring |
| mitre_tactic, mitre_technique, mitre_subtechnique | ATT&CK mapping |
| kill_chain_stage | Kill chain position |
| technical_indicators | JSONB: detection indicators |
| impact | JSONB: impact description |
| recommended_mitigations | JSONB: mitigation steps |
| preconditions, produces_attack_steps | Graph building |

### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/pod-capabilities` | GET | List all pod capabilities |
| `/api/v1/pod-capabilities/:podUid` | GET | Capabilities for a specific pod |
| `/api/v1/pod-capabilities/summary` | GET | Summary statistics |
| `/api/v1/pod-capabilities/summary/severity` | GET | Summary by severity |
| `/api/v1/pod-capabilities/trends` | GET | Capability trends over time |
| `/api/v1/capability-metadata` | GET | Full capability catalog |
| `/api/v1/capability-metadata/:capabilityId` | GET | Single capability definition |

### Key Code Paths

| Component | Path |
|-----------|------|
| PCE Evaluator | `core/pkg/capability/evaluator.go` |
| CSC (State Controller) | `core/pkg/capability/csc.go` |
| Capability Models | `core/pkg/models/pod_capability.go`, `capability_metadata.go` |
| PCE API Handlers | `core/internal/api/capability_handlers.go` |
| Migration 060 | `core/migrations/060_add_capability_metadata_extended_columns.go` |
| Migration 061 | `core/migrations/061_seed_capability_metadata_extended.go` |
| Dashboard Types | `dashboard/types.ts` — `CapabilityMetadata` |
| Dashboard UI | `dashboard/components/CapabilityMetadataBrowser.tsx` |

### UI Components

- **Capabilities Page** (`/capabilities`): Full browser with search, domain filter, expandable details
- **Risk Center → Reference tab**: Same `CapabilityMetadataBrowser` component embedded
- **Pod Detail**: Capability summary per pod

> **Note:** "Capability Catalog" and "Capability Metadata" in the UI refer to the same data source (`capability_metadata` table). The dual naming is a known UX issue.

## Related ADRs

- [ADR-001: Capability reasoning semantics](../../adr/001-capability-reasoning-semantics.md)
- [ADR-004: REP detector governance](../../adr/004-rep-detector-governance.md)
- [ADR-005: Pod capability single table](../../adr/005-pod-capability-single-table.md)
