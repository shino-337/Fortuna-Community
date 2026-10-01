# D3/F/G/H integration acceptance — 2026-09-29

## Permanent execution

Run `python3 scripts/verify/run-local-ci-native.py all` from the checkout to execute
the workflow locally, including its isolated PostgreSQL 16/AGE 1.6 service and
real two-cluster kind gate. Results contain the exact source SHA, source/workflow
fingerprints and log hashes outside the worktree. Only an unchanged clean all-job
pass is publishable. Hosted CI resumed on 2026-10-01 for #54; that run does not
validate the later D3/F/G/H branch until its own exact head is tested.

The dedicated live gate is `scripts/verify/run-two-cluster-integration.py`. It
builds the real Agent, creates only its uniquely named disposable clusters/images,
verifies the pinned kind binary checksum and node image, and cleans up those owned
resources. `FORTUNA_TEST_POSTGRES_URL` must point to an isolated test database.
No lab database, kubeconfig context or deployed workload is used. The gate refuses
missing/skipped/failed named-test events and reads back all required evidence
flags; its negative validator tests run in CI.

## Live backend coverage

The topology contains two clusters, three nodes (two in A), four DaemonSets and
six scoped Agents collecting two namespace scopes. All Agent inventory uses the
real binary, Kubernetes API, scoped HTTP credentials and Core's registered routes.
JWT users/sessions, permissions and cluster scope use the production middleware.
The Core routes run on a host test server with real PostgreSQL; Core main, NATS,
gRPC and a Dashboard browser are outside this harness.

The gate checks cross-cluster JWT and ingest rejection, same Pod names in repeated
namespaces, authenticated receipts for every Agent, scoped relational attack paths,
YAML finding creation, concurrent acknowledge/upserts and reconciliation. It then
previews/executes actual revocation, verifies Kubernetes SubjectAccessReview denies
A while B remains allowed, and verifies refreshed binding inventory. Real UID
replacement survives a queued delete. A PostgreSQL audit trigger fails after a
successful Kubernetes deletion; lease replay through NotFound completes its durable
ledger. A real Agent Pod restart changes execution session without granting
runtime authority.

Agent sync cadence is 30 seconds, the database pool limit is 25, and both graph
and security-state caches use `0s` for explicit fresh-read verification. Default
security-state caching is five minutes; optional
`FORTUNA_SECURITY_STATE_CACHE_TTL` accepts durations from zero to five minutes.
Production timing/resource expectations must include the configured cadence/cache.
Concurrent full syncs coalesce global evaluation into one running pass plus a
pending refresh, rather than creating overlapping global scans per Agent.

Inventory receipts remain latest-per-cluster with explicit namespace scope.
The gate does not fabricate an aggregate completeness receipt for independent
namespace writers. A reconciliation error or ineligible receipt must preserve the
finding; a verified resolution requires its transactionally persisted evidence audit.

## PostgreSQL and migration coverage

Permanent PostgreSQL selections additionally cover duplicate Pod UID/node name,
image content/digest ownership, concurrent ingest and immutable runtime replay,
source-health rollback, insight lifecycle and AGE traversal. AGE tests use real
vertices/edges, duplicate names/UIDs in separate physical graphs, imported foreign
intermediates, parameterized delimiter-containing data and transaction-local pool
search paths.

Run populated backup rehearsals with:

```bash
python3 scripts/verify/rehearse-populated-migration.py --backup /path/to/backup.dump
```

The command restores into its own loopback PostgreSQL container, runs startup
migration twice and rejects evidence-row count loss. The September 27 pre-depfix
backup retained 253 insights, 29 Pods, 29 risk snapshots, 1,646 runtime events,
25 SBOMs and 2,194 components. The pre-reset backup retained 394 insights, 34 Pods,
34 risk snapshots, 1,640 events, 28 SBOMs and 3,132 components. Both rehearsals
passed with equal before/after counts. Their retained backups do not reproduce
the original ten ownership collisions; the synthetic populated PostgreSQL
regression separately verifies collision quarantine, complete payload preservation,
idempotent reruns and unchanged production snapshot upserts.

## Remaining deployment acceptance

Deploying these commits and independent attestor keys/measurement remains a
separate action. Sensor health cannot be inferred from file-reader activity;
absence-based runtime auto-resolution stays disabled. The live browser walkthrough,
Dashboard revocation controls, real mTLS/gRPC certificate lifecycle across clusters,
ingest backlog/state-volume rollout and production-scale storage retention/sizing
remain explicit operational gates. Backend first-investigation evidence and mocked
Dashboard browser regressions do not close those gates.
