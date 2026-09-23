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

A complete window may contain zero events, but only because the producer reported
the interval explicitly. A missing report is unknown, not clean.

Historical windows are accepted so immutable queued reports can drain after a long
Core outage. Historical acceptance does not make them current:
`EffectiveStatus` returns stale when the latest window is outside the freshness
bound.

Absence reasoning must name both ends of the interval it requires. The supported
primitive is `CoversInterval(requiredStart, requiredEnd, now)`; a fresh receipt
whose window ended before `requiredEnd` cannot prove the interval.

## Continuity and retry

Coverage POST is at-least-once. Until Core acknowledges a report, the Agent keeps
the same coverage ID and exact payload. New observations accumulate behind that
immutable pending report.

Core accepts exact replay, rejects changed replay, prevents overlapping or
out-of-order windows and persists `continuous_since`. Continuity extends only
across adjacent complete windows. A failed window or time gap resets continuity.

The PostgreSQL CI gate verifies concurrent first report arbitration, exact replay,
real SQL rollback/recovery and populated startup migration behavior.

## Producer behavior

### Generic runtime file

The reader tracks file identity, offset and an incomplete trailing record.

- truncate or inode replacement resets the cursor, consumes the new file from the
  beginning and marks the interval failed;
- an incomplete trailing JSON record is retained for the next poll rather than
  consumed as invalid;
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
producer-specific, not proof that every required runtime source is enabled for a
cluster or workload.

An in-memory Agent coverage queue is not durable across Agent restart. Losing that
queue creates an evidence gap; it must never be reconstructed as clean coverage.
A future consumer must also verify the required producer set/capability is enabled
rather than trusting an old still-fresh receipt after a producer is disabled.

API/UI availability and explanations remain package E work. Live DaemonSet,
two-cluster, restart and populated migration acceptance remain package F gates.
