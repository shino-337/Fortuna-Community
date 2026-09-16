# Agent credential foundation — C1/C2/C3

**Status:** C1 credential registry/principal, C2 HTTP scoped identity and C3a gRPC
transport identity are merged. C3b adds RPC claim/resource authorization on top of
the authenticated gRPC principal. SBOM/workload storage isolation and deployable
per-Agent gRPC certificate provisioning remain C3c/provisioning work. Do not treat
the current state as completed end-to-end multi-cluster isolation.

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

C3a is merged in PR #43. It remains opt-in because the current deployment overlay
does not yet provision distinct per-Agent client-certificate fingerprints.

## C3b: RPC claims and resource ownership

C3b installs authorization directly after C3a authentication when the scoped gRPC
registry is enabled:

- The trusted Principal is the authorization source. A client-provided
  `x-cluster-id` must either be absent or exactly match `Principal.ClusterID`;
  handlers receive the canonical trusted cluster value.
- `RegisterAgent`, `Ping` and `Heartbeat` require `agent_id` to match the trusted
  Principal. Existing Agent rows already bound to another cluster cannot be
  updated. Legacy Agent rows with an empty cluster are bound to the trusted
  cluster after a successful scoped control operation.
- Scoped `RegisterAgent` returns the trusted Principal cluster rather than
  `DEFAULT_CLUSTER_ID`.
- `SendSBOMFinding` and `SendCVEFinding` require an exact Agent claim and resolve
  Pod ownership with `(pod_uid, principal.cluster_id)` before the handler runs;
  namespace/name claims, when present, must also match inventory.
- `SendCombinedFinding` requires both SBOM and CVE halves to describe the same
  owned Pod and compatible image identity before handler effects.
- Every `BatchSendSBOMFindings` message is transport-reauthenticated by C3a and
  resource-authorized by C3b before it is returned to the stream handler.
- Unknown future scoped AgentService RPCs/messages fail closed until an explicit
  authorization rule is added.
- Ownership database failures return `Unavailable`; they are not interpreted as
  missing/clean/authorized state.
- The Agent collector uses configured `AGENT_ID` for Register/Heartbeat, matching
  the identity provisioned to the credential instead of implicitly using
  `NODE_NAME`.

The C3b regressions are named CI gates: foreign Agent claims, cross-cluster Pods,
forged cluster metadata, mixed CombinedFinding identity, foreign stream messages,
ownership-store failure, future-method fail-closed behavior and Agent ID client
compatibility are all required to run and pass.

C3b is a request/resource authorization boundary. It does not change the older
SBOM storage data model enough to make global image-digest reuse cluster-safe; that
is intentionally C3c.

## Remaining C3 work

### C3c — storage isolation

- Separate reusable image/SBOM content identity from workload ownership. Current
  legacy combined-ingest code can locate/reuse SBOMs globally by image digest and
  update workload fields; the same digest in two clusters must not become a
  cross-cluster association/update primitive.
- Define explicit workload-to-SBOM association keys and ensure every Pod/finding
  link is scoped by cluster/resource identity even when package content is shared.
- Audit repository/upsert conflict keys, CVE match linkage, event payloads and
  reconciliation paths for assumptions that one digest implies one workload.
- Add two-cluster regressions for the same image digest, duplicate Pod names/UID
  boundaries, retries, replacement objects and concurrent ingestion.
- Include atomic ownership behavior for control/storage writes so a failed bind or
  persistence step cannot leave an ambiguous cross-cluster record.

### C3 provisioning

- Extend the operator tooling/overlay to issue or register distinct client
  certificate fingerprints per Agent/node without distributing one Agent identity
  to every DaemonSet pod.
- Define overlap rotation, revocation and rollback for client certificates before
  enabling `FORTUNA_GRPC_AGENT_CREDENTIAL_REGISTRY` in deployment manifests.

## Completion gate

C is complete only after C3c, certificate migration instructions, cross-cluster
storage tests, rotation/revocation on established streams and integration package
F pass. C1/C2/C3a/C3b are required foundations, not a claim that the full HTTP +
gRPC + storage boundary is complete.
