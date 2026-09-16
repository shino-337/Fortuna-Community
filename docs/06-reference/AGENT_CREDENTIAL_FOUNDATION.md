# Agent credential foundation — C1

**Status: library and regression gate only. HTTP and gRPC endpoints do not yet
use this registry. Existing authentication behavior is unchanged. Do not treat C1
as completed per-cluster isolation.**

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

- Define the explicit migration mode and configuration wiring; no silent fallback
  from configured scoped credentials to the legacy shared token.
- Authenticate once through shared middleware before deduplication or DB effects.
- Validate every payload cluster/agent alias and every referenced resource UID.
  Sync ingestion needs separate handling for new inventory identities.
- Validate complete batches before writes; reject foreign references rather than
  accepting the valid prefix of a batch.
- Exercise actual registered routes, not only helper functions. Test missing and
  conflicting aliases, foreign UIDs, revoked credentials and invalid registry.

## C3: gRPC and storage isolation

- Install unary and stream interceptors using the same principal implementation.
- Validate AgentId and resource ownership in Ping, RegisterAgent, SendSBOMFinding,
  SendCombinedFinding and every BatchSendSBOMFindings message.
- Reauthenticate each received stream message to catch revocation/expiry on an
  established connection. Define cancellation behavior for in-flight work.
- Audit digest-based SBOM reuse separately from workload ownership: shared package
  content must not permit cross-cluster Pod links or finding updates.
- Cover actual gRPC service registration with in-process transport tests.
- Add safe provisioning and rollout instructions for per-node agent credentials,
  certificate rotation and rollback; do not reuse one agent identity across a DS.

## Completion gate

C is complete only after HTTP/gRPC wiring, cross-cluster storage tests, rotation/
revocation on established streams and migration instructions pass. C1 is the
prerequisite and is intentionally not enabled by deployment manifests.
