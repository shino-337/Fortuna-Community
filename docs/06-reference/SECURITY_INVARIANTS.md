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
