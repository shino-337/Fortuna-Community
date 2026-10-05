# Runtime evidence

How runtime producer coverage is reported, and the signed source-health protocol that can make a runtime source authoritative. Operator setup for Falco and the source-health relay is in [runtime sensors](../operations/RUNTIME_SENSORS.md).

## Runtime coverage evidence

The Agent reports authenticated coverage evidence for each runtime producer
(`POST /api/v2/runtime/producers` and `POST /api/v2/runtime/coverage`). It lets
Core distinguish a genuinely observed zero-event interval from event silence
caused by an unavailable sensor, malformed input, dropped records or delivery
failure.

### Trust boundary

Coverage is accepted only from a scoped Agent principal and is stored under
`{cluster_id, agent_id, producer_id}`. Runtime event silence never creates
coverage. Legacy shared-token runtime ingest cannot create verified coverage.

The current producer registry is intentionally bounded:

- `runtime-file` / `file`
- `falco` / `falco`
- `ebpf-exec|ebpf-connect|ebpf-all` / `ebpf`

A producer ID cannot be rebound to a different source kind.

### Window semantics

A runtime coverage record is a positive-duration interval
`[windowStart, windowEnd]`.

A window is `complete` only when all of the following are true:

- `dropped == 0`
- `invalid == 0`
- `errors == 0`
- `delivered == emitted`

A complete window may contain zero events, but only because the Agent-side
producer loop reported the interval explicitly. **Complete does not imply
authoritative absence evidence.** Runtime producer manifest v1 has no independent
upstream source-health proof, so current file, Falco and built-in eBPF producers
are all non-authoritative for absence reasoning. A missing report is unknown, not
clean.

Historical windows are accepted so immutable queued reports can drain after a long
Core outage. Historical acceptance does not make them current:
`EffectiveStatus` returns stale when the latest window is outside the freshness
bound.

Absence reasoning must name both ends of the interval it requires. The supported
primitive is `CoversInterval(requiredStart, requiredEnd, now)`; a fresh receipt
whose window ended before `requiredEnd` cannot prove the interval.

### Producer lifecycle and restart/disable semantics

Core persists one lifecycle row per `{cluster_id, agent_id, producer_id}` with
the Agent execution `session_id`, configured enablement, operational state,
lease timestamps, last coverage end and an explicit evidence-gap marker.

The Agent reports the complete bounded producer registry before starting runtime
collectors and renews it periodically. A new Agent process uses a new session ID.
A new session resets eligibility and records an `agent_restart` gap beginning at
the last accepted coverage end when available. Graceful shutdown reports
`stopped`; crash/network silence expires the lifecycle lease. A disabled producer
is persisted as `disabled`.

Operational state and evidence authority are intentionally separate:

- `starting`: configured but no valid current observation yet;
- `active`: the Agent-side reader/sensor loop is producing valid coverage windows;
- `degraded`: the producer reported loss/error;
- `disabled`: configuration says the producer is off;
- `stopped`: the Agent session closed.

An `active` state does **not** mean absence-authoritative. File readability cannot
prove its upstream writer is alive, and tailing an empty Falco JSONL file cannot
prove Falco itself is healthy. Manifest v1 therefore rejects
`authoritative=true` from every producer. A future protocol version must carry an
independent verifiable source-health contract before this can change.

Old receipts are tied to the old session ID. They remain stored for diagnostics
but `EffectiveStatus`/`CoversInterval` reject them after restart, disable,
stopping or lifecycle lease expiry.

### Continuity and retry

Coverage POST is at-least-once. Until Core acknowledges a report, the Agent keeps
the same coverage ID and exact payload. New observations accumulate behind that
immutable pending report.

Core accepts exact replay, rejects changed replay and prevents overlapping or
out-of-order windows. Storage is deliberately split:

- `runtime_coverages` is the mutable latest projection keyed by
  `{cluster_id,agent_id,producer_id}` for lock/order/lifecycle evaluation;
- `runtime_coverage_receipts` is immutable accepted history keyed by
  `{cluster_id,agent_id,producer_id,session_id,coverage_id}`.

Every accepted non-replay window appends one immutable receipt in the same
transaction that updates the latest projection and lifecycle state. A failed SQL
transaction leaves neither a new latest state nor a history receipt. Exact replay
does not duplicate history. Migration backfills a pre-history latest row only when
it already contains a complete evidence identity/window; partial legacy rows are
preserved but are not fabricated into historical evidence.

Continuity extends only across adjacent complete authoritative windows. A failed
window or time gap resets continuity.

The `cluster-identity-postgres` CI job verifies concurrent first report arbitration, exact replay,
immutable history, real SQL rollback/recovery, a populated legacy schema with
missing columns, valid pre-history backfill and migration rerun behavior.

### Producer behavior

#### Generic runtime file

The reader tracks file identity, an in-memory cursor and an incomplete trailing
record. The source file itself is the durable retry buffer for partial records.

- truncate or inode replacement resets the cursor, consumes the new file from the
  beginning and marks the interval failed;
- the cursor advances only past newline-terminated records; it never advances past
  a partial trailing prefix;
- `lineBuf` is diagnostic state only. Partial reconstruction re-reads the prefix
  from the source file, so Agent restart cannot silently skip bytes merely because
  the in-memory buffer was lost;
- Agent restart begins a new lifecycle session/evidence gap. Because the generic
  reader cursor itself is not persisted, restart may replay earlier complete
  records (at-least-once behavior), but a surviving partial prefix is not skipped;
- malformed complete records increment `invalid`;
- failed event delivery keeps the prior cursor for retry.

#### Falco

Falco has an explicit initialized state; `offset == 0` is not a first-run
sentinel.

- a non-empty file present at Agent startup is tailed past the last
  newline-terminated historical record; a trailing partial prefix remains unread
  so it can survive Agent restart without being skipped;
- an empty file at startup is initialized correctly, so the first event written
  later is not skipped;
- truncate or inode replacement processes the new file from the beginning and
  breaks continuity;
- partial lines remain buffered;
- records that cannot resolve a Pod UID are counted as dropped;
- event delivery failure retains both offset and partial-line state.

#### Built-in eBPF

The built-in eBPF sensor is currently experimental and attaches no-op tracepoints.
It does not collect authoritative exec/connect syscall records.

Therefore eBPF coverage is deliberately fail-closed: it may report pipeline,
preflight, drop and delivery state, but it must never establish `complete`
coverage until a real observation pipeline and its loss semantics are implemented
and regression-tested. `EBPF_SIMULATE=true` is synthetic test traffic and is not
runtime evidence.

### Storage and capacity

Core does not prune, partition or archive `runtime_coverage_receipts`; the table
grows with every accepted window. For a large installation, plan retention
(7 days is a reasonable starting point), time partitioning and storage
monitoring yourself, and budget about 2 KiB per receipt plus headroom for the
composite string indexes on `runtime_coverages` and `runtime_coverage_receipts`.

At startup Falco locates the last complete record by scanning backwards in 64 KiB
blocks. Valid JSONL normally finds a newline near EOF. A pathological newline-free
file can require scanning the complete file; this is an operational I/O edge case,
not an evidence-correctness failure.

### Runtime event replay idempotency

Runtime event delivery is at-least-once, so event processing has a separate physical
source-record identity from the older semantic `event_id`.

For file and Falco JSONL inputs the Agent derives `source_record_id` from the
physical file identity, byte offset, raw record bytes and concatenated-record
ordinal. Two identical records in the same second therefore remain distinct when
they occupy different physical positions, while a generic reader restart
reconstructs the same source identity for the same record.

Core never trusts an Agent-supplied identity as globally unique by itself. In
scoped mode the atomic replay key is
`{cluster_id, agent_id, source_record_id}`, where `agent_id` comes from the
authenticated Agent principal and never from request JSON. Explicit legacy
shared-token mode has no trusted Agent identity; it remains compatible by using a
pod-local `legacy-pod:<pod_uid>` replay namespace. That fallback is deliberately
not treated as authenticated Agent ownership and cannot create verified coverage.

The initial `runtime_events` insert is the idempotency claim and shares one
database transaction with behavior facts, synthesized/adapted signals, incident
correlation, runtime risk scoring and capability promotion. PostgreSQL uniqueness
serializes concurrent duplicate submissions. An exact replay returns duplicate and
does not execute downstream effects or rescore notification; reusing the same
physical identity with changed semantic payload fails closed.

Regression tests cover:
- generic runtime-file restart reconstructing identical source IDs;
- two identical same-second records retaining separate physical IDs;
- API exact replay leaving event/signal/risk counts unchanged;
- concurrent PostgreSQL duplicate submissions producing one effect;
- the same local source identity under different authenticated Agents remaining
  separate.

### Deliberate limitations

Coverage never auto-resolves Pod, runtime or cross-resource findings. Coverage
is producer-specific, and protocol v1 deliberately has **zero authoritative
runtime producers** for absence reasoning.

The Agent coverage pending/backlog queue remains intentionally in memory. Restart
can lose a coverage window that Core never acknowledged, so immutable history is
not guaranteed to contain every locally observed pre-restart interval. The new
execution session is persisted as an explicit continuity boundary and `GapSince`
reaches back to the prior accepted coverage end when available. Old receipts from
the prior session cannot satisfy `EffectiveStatus` or `CoversInterval`. This
trade-off preserves fail-closed correctness.

Before absence-based auto-resolution can be enabled, the system must first add
and verify an upstream source-health/enablement proof for the required producer
(e.g. Falco readiness/heartbeat or an equivalent signed/owned health signal).
Configuration enablement, file existence, reader heartbeat, and complete-empty
windows are insufficient.

Coverage and source health are stored by Core for evaluation; there is no read
API or Dashboard view for them yet. External source replacement/truncation while
the Agent is down can still destroy
source bytes; the new lifecycle session/gap prevents absence reasoning across that
period but cannot reconstruct data removed outside Fortuna.

## Independently signed runtime source health (protocol v1)

Core accepts `POST /api/v2/runtime/source-health` through scoped Agent HTTP
credentials. An Agent credential cannot grant source authority: Core separately
verifies an Ed25519 signature against `FORTUNA_SOURCE_HEALTH_REGISTRY`, an
operator-managed JSON file containing `version: 1` and `keys`. Each key contains
`id`, `clusterId`, `agentId`, `producerId`, base64 Ed25519 `publicKey`,
`notBefore`, `expiresAt`, and `revoked`. Reloading the registry applies to new
reports immediately. Previously accepted leases expire within one minute;
revocation does not retrospectively rewrite immutable receipts.

The signed envelope contains `report` and base64 `signature`. The report fields
are defined in `api/collection/source_health.go`. Signing uses the domain prefix
`fortuna/runtime-source-health/v1\n` followed by JSON in the Go struct field order,
with UTC timestamps truncated to microseconds. `sourceSessionId` identifies one
sensor execution; `sessionId` identifies the Agent execution. Sequence numbers
increase within a sensor execution. Healthy windows must have zero errors and
zero dropped records. Windows span at most one minute, end in the past, and
expire no later than one minute after their end or the key's expiration.
Only adjacent, sequential healthy windows extend continuity. Restarts, failure,
disable, stopping, expired health and expired Agent leases break eligibility.
Exact replay returns success without renewing receipt time or authority.

The independent sensor/attestor must verify upstream collection health for the
whole interval, including loss/drop counters and sensor restart state. File
existence, file-reader activity, an Agent heartbeat, configuration enablement,
and a clean empty window cannot establish this contract. Built-in file/Falco
readers and the current eBPF stub do not generate trusted health themselves.
eBPF health is rejected until a real upstream contract is implemented.

Runtime absence-based automatic resolution remains disabled. This protocol only
records bounded, independently verified source evidence. Receipt retention and
partition sizing are deployment responsibilities (see
[storage and capacity](#storage-and-capacity)).
