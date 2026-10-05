# Agent identity

How Core binds each Agent credential to one cluster and Agent, and how Agent rows carry their cluster. To provision credentials, follow [scoped HTTP credentials](../../deploy/scoped-agent-credentials/README.md) and [per-Agent mTLS](../../deploy/scoped-agent-credentials/MTLS.md).

## Agent credentials

`core/pkg/agentidentity.Store` binds each credential to one exact cluster/Agent.
Registries hold SHA-256 token or certificate digests, required validity times and
optional revocation. Invalid/ambiguous/unreadable registries fail closed and are
reread on authentication. TLS requires a completed verified chain and a registered
leaf fingerprint; caller metadata and unverified certificates are insufficient.

### HTTP

`FORTUNA_AGENT_CREDENTIAL_REGISTRY` enables scoped HTTP authentication for
`/api/v1/agent/*` and `/api/v2/runtime/events`. Agent identity claims and complete
runtime batches are checked before effects. Senders read a node-local
`FORTUNA_AGENT_TOKEN_FILE`; a configured invalid file never falls back to the
shared `FORTUNA_INGEST_TOKEN`. There are no v1 runtime ingest routes.
The shared-token mode remains an explicit HTTP migration boundary when scoped
HTTP authentication is not configured.

See [HTTP provisioning](../../deploy/scoped-agent-credentials/README.md).

### gRPC

`FORTUNA_GRPC_AGENT_CREDENTIAL_REGISTRY` requires TLS and binds verified leaf
certificates to trusted principals. Register/Ping/Heartbeat claims must match the
principal. Pod findings resolve ownership using cluster + Pod UID. Conflicting
cluster metadata and unknown methods fail closed. Combined finding writes are
retired; standalone CVE findings are not implemented (Core derives CVEs).

Without the registry, Core quarantines gRPC writes and streams; a read-only Ping reports
`identity_required`. Scoped Agent writes use `(cluster_id, agent_id)` atomically.
Unowned legacy records remain separate; one cluster cannot adopt or update another
cluster's row even when IDs/node names match. Soft-delete restoration is scoped.

Streams reauthenticate both before and after receiving a message so revocation
during a blocked receive is enforced before handler effects. Already-running
unary operations are not cancelled. Registry projection latency on each Core
replica determines when an operator update becomes visible.

See [certificate issuance, overlap, revocation and rollback](../../deploy/scoped-agent-credentials/MTLS.md).
Agents reload a matched certificate/key generation on new TLS handshakes; rotate
via a versioned node-local directory and restart the Agent pod for immediate
connection replacement. A malformed replacement fails closed.

### Storage and regression tests

SBOM content is reusable but ownership remains on cluster/Pod/container/digest
observations. PostgreSQL uniqueness/ownership guards and populated migrations are
covered by required CI jobs. Agent composite-key migration similarly preserves
unowned records and rejects identity reassignment. Named security regressions
cover foreign claims, duplicate IDs/UIDs, stream revocation, unavailable storage,
legacy quarantine and credential rotation. A failing or incomplete security scan
must be reported separately from passing unit/integration tests.

## Agent cluster identity

The `agents` table has an indexed `cluster_id`. Rows created before it existed
stay unassigned; node-name similarity is not used as migration evidence. Agent IDs
remain globally unique for compatibility with the existing gRPC contract.

The existing authenticated HTTP sync payload supplies the canonical cluster ID.
The identity upsert atomically claims an unassigned agent, updates an agent in the
same cluster, or returns 409 for an ID already assigned elsewhere. Database
failures return 500 rather than being ignored. Cluster agent lists and Dashboard
counts use explicit cluster_id; unassigned legacy agents are excluded until sync.

Without the gRPC registry, Register/Ping can create an unassigned agent and refresh it by agent
ID. A Ping without an agent ID never updates a row by node name, which prevents
accidental cross-cluster node-name correlation. On its own this adds no per-cluster cryptographic attestation; that
comes from the scoped credentials above. In legacy shared-token mode any Agent
holding the token can still claim any cluster.

After upgrading Core, allow a successful HTTP inventory sync from each Agent so
existing rows are assigned. Do not backfill `cluster_id` by node name. For a
deliberate Agent reassignment, investigate the conflicting identity instead of
moving it automatically. Keep the additive column if rolling back Core.
