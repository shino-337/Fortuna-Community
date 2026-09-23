# Security invariants and regression-prevention contract

Fortuna treats security fixes as invariants, not one-time patches. The goal of the
C3c–F work (#45–#51 planning sequence) is to make known security failure classes
machine-detectable so a later change cannot silently reintroduce them.

This contract reduces regression risk; it is not a claim that software can never
contain a new defect. New findings must be converted into a reproducible test or
static invariant before their fixing PR is considered complete.

## Invariant 1 — cluster-qualified resource identity

A Kubernetes resource UID is not a Fortuna-wide identity. Any persisted or
authorized workload reference is identified by the pair:

`{cluster_id, resource_uid}`

For Pods this is `{cluster_id, pod_uid}`. The same Pod UID in two clusters must be
able to coexist without read, write, cache, reconciliation, runtime, finding or
graph state crossing the cluster boundary.

Rules:

- Authorization must never infer ownership from `pod_uid LIMIT 1`.
- Storage keys, upserts, caches and reconciliation keys must retain cluster ID.
- A request-supplied cluster cannot override the authenticated/scoped cluster.
- Missing or ambiguous ownership fails closed. Migration/backfill must not guess.
- Existing non-empty ownership that conflicts with authoritative Pod identity must
  be rejected rather than silently rewritten.
- Generic resource references without a trustworthy resource type must not be
  guessed to be Pod references merely because their UID happens to match a Pod.
- Image digest identifies reusable image content, not workload ownership.

`core/pkg/resourceidentity` is the canonical in-process identity primitive.
`scripts/verify/check-cluster-resource-models.py` is the first static ratchet: a
persisted model carrying `PodUID` must also carry `ClusterID`, and persisted Pod
models plus explicitly classified resource-typed Pod models must remain represented
in the cluster-resource migration manifest.

## Invariant 2 — incomplete evidence is not clean evidence

Database, collector, runtime, SBOM, CVE or authorization-state failure must not be
converted into an empty, zero, healthy or clean security result. Required evidence
that is unavailable, stale, malformed or incomplete must remain explicit and must
not auto-resolve an active finding.

The D/E packages extend this into source health, observation time, completeness,
freshness and UI/API availability semantics.

## Invariant 3 — authentication is preserved through storage

Successful scoped HTTP or gRPC authentication is only the first boundary. The
trusted `{cluster_id, agent_id}` principal must remain authoritative while resolving
Pods and while persisting every derived object. A handler may not authenticate a
cluster-qualified Pod and later write to a UID-only storage key.

## Invariant 4 — batches are authorized before effects

For batch ingest, every target must be authenticated, ownership-resolved and
validated before the first deduplication, database, runtime-processing or other
handler effect. A mixed valid/foreign batch must not partially persist.

## Invariant 5 — ambiguous historical state is quarantined

Legacy data may lack cluster ownership. Backfill is permitted only where one
resource UID maps to exactly one observed cluster. If the same UID maps to more
than one cluster, ownership remains unresolved until an explicit repair process
can establish it. Namespace/name/digest similarity is not ownership proof.

## Invariant 6 — security regressions are named CI requirements

A security fix is incomplete until its regression is anchored in
`scripts/verify/check-security-regressions.py` or an equivalent mandatory CI
contract. Removal, rename, skip or failure of a required regression must fail CI.
Static architectural invariants belong under `scripts/verify/` and are run by CI.

Each #45–#51 package must add or strengthen its corresponding gate rather than
only relying on broad `go test ./...` coverage.

## Invariant 7 — migration history is append-only

The legacy migration runner currently records migration versions by slice position.
Existing entries in the migration list therefore must not be reordered or inserted
in front of already-shipped entries. New versioned migrations are append-only.
Security-critical schema invariants that cannot safely rely on the legacy runner
must fail startup explicitly until the migration framework is made identity-based.

## Invariant 8 — real multi-cluster behavior is a release gate

Unit and SQLite tests are necessary but do not establish end-to-end isolation.
#45 includes a focused PostgreSQL migration gate for the cluster-resource identity
foundation; package F (#51 plan) must still provide reproducible populated-database
and real two-cluster tests for at least:

- identical Pod UID in different clusters;
- identical node/Agent names in different clusters;
- identical image digest used by workloads in different clusters;
- concurrent ingestion and retry/replay;
- certificate rotation/revocation on established streams;
- missing/stale evidence and recovery;
- populated-database schema migration and upgrade behavior;
- authorization for restricted users and aggregate endpoints;
- deletion/replacement UID race behavior.

The #51 gate becomes permanent CI/lab evidence. Future changes touching cluster
identity, ingest, storage, authorization, runtime evidence, findings or migration
code must continue to pass it.

## Repository governance prerequisite

CODEOWNERS covers security-sensitive paths, but CODEOWNERS alone does not enforce
review. The repository owner must protect `main` (ruleset or branch protection) and
require the relevant CI checks plus CODEOWNER review. Direct or force pushes that
bypass those gates defeat the regression-prevention model.

## Completion rule for a finding

For every security finding fixed during #45–#51:

1. reproduce the failure;
2. encode the expected invariant;
3. implement the fix;
4. add a named regression/static check;
5. exercise negative and failure paths, not only success;
6. include PostgreSQL/two-cluster evidence where storage or scope is involved;
7. document any remaining migration/compatibility limitation;
8. keep the gate after merge.

A finding is not considered permanently closed merely because its current code
path was patched.

## Evidence eligibility for automatic resolution (D1 + D2 inventory)

Database absence is not an authoritative deletion observation. Reconciliation
must retain findings for missing/ambiguous owners, stale/invalid snapshots,
malformed evidence, unavailable detectors and unknown collection coverage.
Runtime silence and expiry of a lookback window never prove remediation.

D1 permits automatic resolution only for self-contained Role/ClusterRole CEL
checks reading fields from that exact static snapshot. D2 inventory additionally
requires an authenticated collection receipt: the resource-kind List start bound
must be at most ten minutes old, not future-dated, not older than the finding, and
its digest must match that exact cluster/kind/UID/name/namespace/rules projection. A missing, failed, unverified or
stale receipt blocks resolution. The rules array must be structurally valid. CEL dependencies are checked
from the parsed expression; aliases/bracket access cannot introduce unverified
runtime or cross-resource inputs. Unknown dependencies are denied.

A snapshot update timestamp is NOT a collector observation or completeness
receipt. Agent heartbeats, Cluster.LastSync, runtime event timestamps and projector
UpdatedAt must not be substituted for such receipts. Runtime, Pod and
cross-resource auto-resolution remains blocked until D2 provides a verified
producer-to-evaluator coverage contract. This is intentionally conservative:
findings may remain active after remediation and need explicit human review.

The resource snapshot is locked and version-checked before committing resolution;
the finding update compares its original UpdatedAt and active status. A concurrent
sync/detection/manual update must preserve the newer state. The status change and
cluster-qualified resolution audit commit together or both roll back.

Diagnostics are emitted by InsightStatusUpdater with a resolution-evidence reason.
Inventory observation status/timestamps and complete-empty lists are persisted
and returned to the scoped Agent. Runtime coverage windows and API/UI presentation
remain D2 runtime/E work; inventory receipts do not establish sensor coverage.

## Inventory receipt integrity (D2 inventory)

Only scoped Agent identity can issue a verified receipt. A request authenticated
as a scoped Agent must include collection evidence; missing collection metadata is
a hard reject and must never downgrade into an unverified write. Agent health metadata
from the same HTTP sync commits with the inventory transaction; failed inventory
persistence must not advance Agent liveness. Scoped HTTP mode is exclusive when
the credential registry is configured and does not fall back to the legacy shared
token. Any explicit unverified compatibility write still invalidates prior verified
status because it may alter the projection without trustworthy evidence. Core startup
must establish the receipt schema before accepting traffic; old rows are never
backfilled as complete. Collection namespace, all eight list counts and all eight
per-kind Kubernetes List start bounds are checked before inventory effects. The eight List calls are
sequential: the receipt is a bounded interval, not an atomic Kubernetes snapshot.
Cross-resource consistency must not be inferred from it. Missing/null lists, pagination left by Kubernetes and
collection errors cannot be represented as successful empty observations.

Inventory writes and receipts commit in one transaction serialized per cluster.
Asynchronous capability/cleanup work starts after commit. Persistence errors roll
back the projection and record a failed attempt when storage is available; request
cancellation does not prevent this bounded failure recording. Failures to persist
the failure are returned, never hidden. Receipt and resource locks have consistent
ordering during resolution. Identical accepted replays are idempotent; changed
replays and older observations are rejected. Failed attempts are terminal: retry
by collecting a new attempt ID, as Syncer does on its next cycle.

A complete receipt proves the collection completed and its included rows were
accepted. It does not claim authoritative projection reconciliation or deletion.
The HTTP contract must expose non-authoritative deletion semantics, and retained
rows absent from a complete-empty collection must not become eligible for
auto-resolution. Existing
empty/partial deletion safeguards remain. Namespace-limited collections must reject out-of-scope namespaced rows and must
not prune namespaced inventory elsewhere. Namespaced object upsert identity is
`cluster_id + namespace + uid`; a same UID already persisted in another namespace
is an integrity conflict and must fail closed rather than move/adopt that row. ClusterRole/ClusterRoleBinding lists still
cover cluster-scoped objects. The PostgreSQL CI gate exercises concurrent replay,
SQL-trigger failure, rollback and recovery, including timestamp precision.


### Multi-Agent arbitration

The production Agent is a DaemonSet: more than one scoped Agent in the same
cluster can submit full inventory. The current receipt is intentionally the latest
accepted attempt per cluster. Ordering is by collection start time and the receipt
row is serialized with a database lock. Therefore:

- a newer failed Agent attempt may make resolution evidence unavailable but must
  not mutate the previously accepted inventory projection;
- an older Agent attempt must not roll the receipt or projection backwards;
- a newer complete Agent attempt must recover the receipt deterministically;
- different namespace scopes may supersede one another and reduce eligibility,
  but a scope mismatch must preserve findings rather than resolve them.

This is a fail-closed availability trade-off, not proof of multi-scope aggregation.
Package F must exercise the real DaemonSet topology and decide whether
latest-per-cluster remains acceptable or the receipt key must become scope-qualified.

## Security-change review protocol

For security-sensitive changes after #50, review the complete state machine before
writing the fix. Every PR touching identity, ingest, persistence, reconciliation,
runtime evidence or migrations must explicitly map these dimensions:

1. trust boundary and authoritative identity;
2. cluster / agent / namespace / resource ownership keys;
3. success, empty, partial, failed, replay and out-of-order states;
4. concurrent writers and transaction lock ordering;
5. rollback behavior and side effects that occur before/after commit;
6. legacy/admin/compatibility writers that can mutate the same projection;
7. evidence freshness, loss and negative evidence semantics;
8. migration from populated data and restart/rerun behavior;
9. supported deployment topology, including DaemonSet multi-writer behavior;
10. permanent named regression tests for every accepted invariant and every
    fail-closed boundary.

A code change resets merge readiness. After the final runtime-code commit, rerun
the full invariant review against the resulting diff, then require Core, Agent,
API, PostgreSQL and security-regression gates to pass on that exact head. Test/doc
commits may follow, but any further runtime-code change requires the review cycle
again.

## Runtime producer coverage integrity

Runtime event silence is never clean evidence. Verified coverage requires a scoped
Agent producer window. Complete windows have positive duration and zero
drop/invalid/error counts with `delivered == emitted`. Exact coverage retries are
immutable; changed replay, overlap, out-of-order windows and producer/source
rebinding fail closed.

Continuity extends only over adjacent complete windows. A gap or failed window
resets `continuous_since`. Historical queued windows may be persisted after an
outage, but freshness is evaluated independently. Any absence-based consumer must
require an explicit bounded interval whose required end is covered; receipt
freshness alone is insufficient.

File and Falco producers must treat truncate/replacement, partial records and
delivery failures as coverage-breaking conditions. Unresolved Falco Pod identity
is a drop, not clean silence.

The built-in eBPF sensor currently uses no-op tracepoints and is non-authoritative.
It must not emit complete coverage, including in simulation mode. Removing this
fail-closed rule requires a real event collector plus permanent observation/drop/
shutdown regressions.

Runtime coverage remains producer-specific. Before auto-resolution consumes it,
the caller must also prove the required producer/capability is currently enabled.
An old receipt from a disabled producer is not sufficient.

PostgreSQL coverage arbitration is a permanent CI gate: concurrent first reports
must converge without unique-key 500s, storage failure must roll back the accepted
window, and recovery/startup migration must preserve evidence.
