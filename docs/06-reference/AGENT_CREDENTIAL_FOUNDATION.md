# Agent credential foundation — C1/C2

**Status:** C1 credential registry/principal is merged. HTTP Agent and runtime
ingest use scoped agent identity whenever `FORTUNA_AGENT_CREDENTIAL_REGISTRY` is
configured. Sync, Pod evidence and runtime event paths validate trusted identity /
resource ownership before effects. Deployments that have not enabled the registry
remain on the explicit legacy shared-token mode. gRPC enforcement/storage isolation
remains C3, so do not treat C2 as completed end-to-end multi-cluster isolation.

`core/pkg/agentidentity.Store` provides a shared trusted principal for transport
adapters. A credential maps to exactly one cluster and one agent. A principal's
`CheckClaims` rejects foreign or missing cluster/agent claims.

The registry is an operator-managed JSON file with a `credentials` array. Each
entry contains `id`, `cluster_id`, `agent_id`, exactly one of `token_sha256` or
`certificate_sha256`, required RFC3339 `expires_at`, optional `not_before`, and
optional `revoked`. Digests are lowercase SHA-256 hex. Never store plaintext
bearer tokens in this file. Tokens should contain at least 32 cryptographically
random bytes; the library's minimum length check does not establish entropy.

TLS authentication requires a completed handshake and a verified certificate
chain supplied by the transport, then matches the leaf certificate fingerprint.
The library also rechecks the leaf validity period. A caller-supplied fingerprint
or unverified certificate alone is insufficient.

The file is reread for every authentication. Invalid, ambiguous or unreadable
configuration fails closed; no cached credential is accepted. Replace the whole
file atomically, with permissions limited to the operator and Core. To rotate,
provision a distinct new entry for the same principal, overlap while distributing
its secret/certificate, then revoke or remove the old entry. Revocation takes
effect at the next authentication, not retroactively for an operation already
running. An empty credentials array denies all credentials.

## C2: HTTP enforcement

Implemented boundaries:

- `FORTUNA_AGENT_CREDENTIAL_REGISTRY` is the explicit Core migration switch. When
  configured, `/api/v1/agent/*`, `/api/v1/runtime/events` and
  `/api/v2/runtime/events` do not fall back to the legacy shared token.
- Registered ingest routes authenticate through shared middleware before handler
  work, deduplication or database effects.
- `/sync` validates cluster aliases and Agent ID against the authenticated
  principal before normalization/rate limiting/writes.
- Pod metrics/process/network/event ingest validates Pod UID, namespace and
  cluster ownership before effects; event batches are validated completely.
- Runtime v1/v2 validate the complete batch against Pod UID/namespace ownership in
  the authenticated principal's cluster before either runtime handler executes.
  Mixed-cluster batches are rejected as a whole, so a valid prefix cannot create
  partial effects.
- Generic runtime JSONL, Falco and eBPF senders retain telemetry across transient
  Core rejects so identity/inventory convergence does not silently discard data.
- Per-node provisioning uses a node-local `FORTUNA_AGENT_TOKEN_FILE`. The Agent
  rereads it for every scoped HTTP ingest request, including runtime v1/v2; an
  unreadable/invalid configured file fails closed rather than falling back to
  `FORTUNA_INGEST_TOKEN` or a Bearer header.
- `FORTUNA_INGEST_TOKEN` remains only as the explicit backward-compatible mode for
  deployments where the scoped registry/token file are not configured.
- Registered-route regressions cover legacy-token rejection in scoped mode,
  cluster-A credential versus cluster-B Pod isolation, mixed-batch no-effects,
  immediate revocation, invalid registry fail-closed behavior and explicit legacy
  compatibility when scoped mode is not enabled.

Deployment, issuance, overlap rotation, revocation and rollback procedures are in
`deploy/scoped-agent-credentials/README.md`. The reference overlay is opt-in so
existing installations are not silently switched to scoped mode.

C2 HTTP implementation is complete at the code/regression boundary. Reproducible
two-cluster rollout and recovery evidence remains part of integration package F;
it is intentionally separate from the transport implementation claim.

## C3: gRPC and storage isolation

- Install unary and stream interceptors using the same principal implementation.
- Validate AgentId and resource ownership in Ping, RegisterAgent, SendSBOMFinding,
  SendCombinedFinding and every BatchSendSBOMFindings message.
- Reauthenticate each received stream message to catch revocation/expiry on an
  established connection. Define cancellation behavior for in-flight work.
- Audit digest-based SBOM reuse separately from workload ownership: shared package
  content must not permit cross-cluster Pod links or finding updates.
- Cover actual gRPC service registration with in-process transport tests.
- Extend provisioning to the gRPC credential/certificate path without reusing one
  agent identity across a DaemonSet.

## Completion gate

C is complete only after C3 gRPC wiring, cross-cluster storage tests,
rotation/revocation on established streams, migration instructions and two-cluster
integration evidence pass. C1/C2 are required transport foundations, not a claim
that the full HTTP + gRPC + storage boundary is complete.
