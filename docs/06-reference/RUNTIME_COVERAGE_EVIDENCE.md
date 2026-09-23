# Runtime Coverage Evidence

PR #52 adds authenticated producer coverage evidence for runtime telemetry. The
goal is to distinguish a genuinely observed zero-event interval from event silence
caused by an unavailable sensor, malformed input, dropped records or delivery
failure.

## Trust boundary

Coverage is accepted only from a scoped Agent principal and is stored under
`{cluster_id, agent_id, producer_id}`. Runtime event silence never creates
coverage. Legacy shared-token runtime ingest cannot create verified coverage.

The current producer registry is intentionally bounded:

- `runtime-file` / `file`
- `falco` / `falco`
- `ebpf-exec|ebpf-connect|ebpf-all` / `ebpf`

A producer ID cannot be rebound to a different source kind.

## Window semantics

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

## Producer lifecycle and restart/disable semantics

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

## Continuity and retry

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

The PostgreSQL CI gate verifies concurrent first report arbitration, exact replay,
immutable history, real SQL rollback/recovery, a populated legacy schema with
missing columns, valid pre-history backfill and migration rerun behavior.

## Producer behavior

### Generic runtime file

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

### Falco

Falco has an explicit initialized state; `offset == 0` is not a first-run
sentinel.

- a non-empty file present at Agent startup is tailed from EOF once and no
  historical coverage is claimed;
- an empty file at startup is initialized correctly, so the first event written
  later is not skipped;
- truncate or inode replacement processes the new file from the beginning and
  breaks continuity;
- partial lines remain buffered;
- records that cannot resolve a Pod UID are counted as dropped;
- event delivery failure retains both offset and partial-line state.

### Built-in eBPF

The built-in eBPF sensor is currently experimental and attaches no-op tracepoints.
It does not collect authoritative exec/connect syscall records.

Therefore eBPF coverage is deliberately fail-closed: it may report pipeline,
preflight, drop and delivery state, but it must never establish `complete`
coverage until a real observation pipeline and its loss semantics are implemented
and regression-tested. `EBPF_SIMULATE=true` is synthetic test traffic and is not
runtime evidence.

## Deliberate limitations

PR #52 does not enable Pod/runtime/cross-resource auto-resolution. Coverage is
producer-specific, and current protocol v1 deliberately has **zero authoritative
runtime producers** for absence reasoning.

The Agent coverage queue remains in memory. Restart can lose queued observations,
but the new execution session is persisted as an explicit continuity boundary and
`GapSince` reaches back to the prior accepted coverage end when available. Old
receipts from the prior session cannot satisfy `EffectiveStatus` or
`CoversInterval`.

To enable absence-based auto-resolution in a later PR, the system must first add
and verify an upstream source-health/enablement proof for the required producer
(e.g. Falco readiness/heartbeat or an equivalent signed/owned health signal).
Configuration enablement, file existence, reader heartbeat, and complete-empty
windows are insufficient.

API/UI availability and explanations remain package E work. External source replacement/truncation while the Agent is down can still destroy
source bytes; the new lifecycle session/gap prevents absence reasoning across that
period but cannot reconstruct data removed outside Fortuna.

Live DaemonSet, two-cluster, restart and populated migration acceptance remain
package F gates.
