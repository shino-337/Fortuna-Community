# Agent credential foundation — C1/C2/C3

**Status:** C1 credential registry/principal and C2 HTTP scoped identity are merged.
HTTP Agent and runtime ingest use scoped agent identity whenever
`FORTUNA_AGENT_CREDENTIAL_REGISTRY` is configured. C3a adds an independent,
opt-in gRPC transport identity boundary through
`FORTUNA_GRPC_AGENT_CREDENTIAL_REGISTRY`; RPC claim/resource enforcement and
SBOM/workload storage isolation remain C3b/C3c. Do not treat the current state as
completed end-to-end multi-cluster isolation.

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
The library also rechecks the leaf validity period. A caller-supplied fingerprint,
metadata value or unverified certificate alone is insufficient.

The file is reread for every authentication. Invalid, ambiguous or unreadable
configuration fails closed; no cached credential is accepted. Replace the whole
file atomically, with permissions limited to the operator and Core. To rotate,
provision a distinct new entry for the same principal, overlap while distributing
its secret/certificate, then revoke or remove the old entry.

## C2: HTTP enforcement

Implemented boundaries:

- `FORTUNA_AGENT_CREDENTIAL_REGISTRY` is the explicit HTTP migration switch. When
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
  Mixed-cluster batches are rejected as a whole.
- Generic runtime JSONL, Falco and eBPF senders retain telemetry across transient
  Core rejects so identity/inventory convergence does not silently discard data.
- Per-node provisioning uses a node-local `FORTUNA_AGENT_TOKEN_FILE`; an
  unreadable/invalid configured file fails closed rather than falling back to
  `FORTUNA_INGEST_TOKEN` or a Bearer header.
- `FORTUNA_INGEST_TOKEN` remains only as the explicit backward-compatible mode for
  deployments where scoped HTTP identity is not configured.

Deployment, issuance, overlap rotation, revocation and rollback procedures are in
`deploy/scoped-agent-credentials/README.md`. C2 HTTP is complete at the
code/regression boundary; reproducible two-cluster rollout evidence remains F.

## C3a: gRPC transport identity

C3a introduces a separate migration switch because HTTP bearer credentials and
gRPC client-certificate credentials are not provisioned at the same time:

- `FORTUNA_GRPC_AGENT_CREDENTIAL_REGISTRY` points to a registry using the same
  schema and trusted `agentidentity.Principal` implementation as HTTP.
- Enabling this switch requires `TLS_ENABLED=true`; Core refuses to construct the
  gRPC server if scoped identity is requested over plaintext transport.
- The unary interceptor derives identity only from verified gRPC `TLSInfo` and the
  client leaf certificate fingerprint. It installs the resulting trusted
  `{credential, cluster, agent}` principal into the handler context.
- The stream interceptor authenticates at stream establishment and before every
  `RecvMsg`. Registry revocation/expiry and client-certificate expiry therefore
  take effect on the next received message of an established stream.
- Registry failure maps to gRPC `Unavailable`; absent/unverified/revoked identity
  maps to `Unauthenticated`. There is no metadata/fingerprint fallback.
- If `FORTUNA_GRPC_AGENT_CREDENTIAL_REGISTRY` is empty, current gRPC behavior is
  preserved during migration.

C3a is a transport boundary only. It is intentionally not enabled by the current
C2 deployment overlay because per-Agent certificate fingerprint provisioning has
not yet been implemented. Do not point the gRPC switch at a token-only registry
and expect existing client certificates to authenticate.

## Remaining C3 work

### C3b — RPC claims and resource ownership

- Validate trusted principal against `agent_id` and cluster/resource ownership in
  `Ping`, `RegisterAgent`, `Heartbeat`, `SendSBOMFinding`, `SendCVEFinding` and
  `SendCombinedFinding`.
- Validate every `BatchSendSBOMFindings` message after per-message transport
  reauthentication and before any message-specific effects.
- Stop using untrusted `x-cluster-id` metadata as an authorization source. It may
  remain correlation/rate-limit input only after consistency with the trusted
  principal is established.
- Cover actual AgentService registration through in-process transport tests.

### C3c — storage isolation

- Audit digest-based SBOM reuse separately from workload ownership: shared package
  content may be reusable, but a credential for cluster A must not create/update
  Pod/SBOM/finding associations owned by cluster B.
- Define uniqueness and lookup keys so image-digest reuse cannot become a
  cross-cluster write primitive.
- Add two-cluster regression coverage for reused images, Pod UID ownership,
  retries, replacement objects and concurrent ingestion.

### C3 provisioning

- Extend the operator tooling/overlay to issue or register distinct client
  certificate fingerprints per Agent/node without distributing one Agent identity
  to every DaemonSet pod.
- Define overlap rotation, revocation and rollback for client certificates before
  enabling `FORTUNA_GRPC_AGENT_CREDENTIAL_REGISTRY` in deployment manifests.

## Completion gate

C is complete only after C3b/C3c, certificate migration instructions,
cross-cluster storage tests, rotation/revocation on established streams and
integration package F pass. C1/C2/C3a are required foundations, not a claim that
the full HTTP + gRPC + storage boundary is complete.
