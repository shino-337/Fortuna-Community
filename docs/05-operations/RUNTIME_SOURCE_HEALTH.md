# Independently signed runtime source health (protocol v1)

Core accepts `POST /api/v2/runtime/source-health` through scoped Agent HTTP
credentials. An Agent credential cannot grant source authority: Core separately
verifies an Ed25519 signature against `FORTUNA_SOURCE_HEALTH_REGISTRY`, an
operator-managed JSON file containing `version: 1` and `keys`. Each key contains
`id`, `clusterId`, `agentId`, `producerId`, base64 Ed25519 `publicKey`,
`notBefore`, `expiresAt`, and `revoked`. Reloading the registry applies to new
reports immediately. Previously accepted leases expire within one minute;
revocation does not retrospectively rewrite immutable receipts.

The signed envelope contains `report` and base64 `signature`. The report fields
are defined in `api/collection/source_health.go`. Signing uses the domain prefix
`fortuna/runtime-source-health/v1\n` followed by JSON in the Go struct field order,
with UTC timestamps truncated to microseconds. `sourceSessionId` identifies one
sensor execution; `sessionId` identifies the Agent execution. Sequence numbers
increase within a sensor execution. Healthy windows must have zero errors and
zero dropped records. Windows span at most one minute, end in the past, and
expire no later than one minute after their end or the key's expiration.
Only adjacent, sequential healthy windows extend continuity. Restarts, failure,
disable, stopping, expired health and expired Agent leases break eligibility.
Exact replay returns success without renewing receipt time or authority.

The independent sensor/attestor must verify upstream collection health for the
whole interval, including loss/drop counters and sensor restart state. File
existence, file-reader activity, an Agent heartbeat, configuration enablement,
and a clean empty window cannot establish this contract. Built-in file/Falco
readers and the current eBPF stub do not generate trusted health themselves.
eBPF health is rejected until a real upstream contract is implemented.

Optional Agent relay configuration:

- `RUNTIME_SOURCE_HEALTH_CHALLENGE_PATH`: atomically published mode-0600 JSON
  containing the current cluster, Agent, session and session start.
- `RUNTIME_SOURCE_HEALTH_PATH`: independently produced JSON array of up to two
  signed reports, one per file/Falco producer. Mount this input read-only in
  the Agent. Keep signing keys outside the Agent and its credential mounts.

The relay polls every five seconds, preserves signatures, rejects a stale Agent
session and backs off all sibling requests after rejection or network failure.
The independent attestor must read a fresh challenge after every Agent restart.
For integration with an existing attestor, `api/cmd/source-health-sign` signs an
already measured report using a PKCS#8 Ed25519 PEM key; it does not measure or
assert sensor health. Send the envelope with the bound scoped Agent credential,
or atomically publish it in the relay array. Persist sensor sequence/session
state independently. Core stores both signed payload and signature for audit.

Runtime absence-based automatic resolution remains disabled. This protocol adds
bounded independently verified source evidence; enabling automatic resolution
still requires the package F live topology/evaluator acceptance gate. Receipt
retention and production partition sizing remain deployment responsibilities.
