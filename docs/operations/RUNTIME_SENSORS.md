# Runtime sensors

Runtime evidence comes from Falco through the Agent. The built-in eBPF sensor is an experimental scaffold that attaches no-op tracepoints; keep `EBPF_ENABLED=false` and `EBPF_SIMULATE` disabled for real evidence. What runtime evidence does and does not prove is in [runtime evidence](../reference/RUNTIME_EVIDENCE.md).

## Install Falco

```bash
./scripts/deploy/install-falco-fortuna.sh   # Falco with deploy/falco/helm-values-fortuna.yaml
kubectl -n fortuna rollout restart daemonset/fortuna-agent
```

The Agent reads Falco's JSONL output from `/var/log/falco/events.jsonl` on each node (`FALCO_EVENTS_ENABLED`, `FALCO_EVENTS_PATH`). Pipeline & Runtime Health tells apart "no events arrived" from "no sensor is enabled".

## Preserve Falco delivery state

The bundled Agent DaemonSet enables `FALCO_DELIVERY_STATE_PATH` at
`/var/lib/fortuna-agent/falco-delivery.json`, backed by that directory on each
node. Merely updating the image on an older DaemonSet does not add the new env,
volume and mount; include those manifest changes in the eventual rollout while
preserving its scoped credential overlay. Keep `/var/log/falco` read-only.
The Agent needs write access to its state directory; checkpoints are atomically
written with mode `0600`, synced to disk and exclusively locked.

The outbox binds the exact cluster ID, Agent ID, Core URL and Falco source path.
It retains canonical event payloads, physical source-record IDs, cursor and retry
deadlines across restart and source-file rotation. Only explicit ownership
rejections enter quarantine; all other failures preserve unsent evidence and
stop the current flush. Quarantine is retried every five minutes with a shared
12-request budget, and transient backoff honors `Retry-After` with a 30-second
minimum. Pending or quarantined evidence always marks coverage failed; it does
not enable absence-based resolution.

Limits are 2048 pending-plus-quarantined events, a 16 MiB checkpoint, and a 1 MiB
source read per poll. Large backlogs drain in bounded batches without skipping
records. An individual physical record larger than the read limit, a full
quarantine, write failure, corrupt/wrong-binding state or another writer blocks
ingestion and reports failed coverage. Investigate Agent logs, free space,
directory permissions and the authenticated inventory before retrying. Resolve
stale ownership through inventory reconciliation, not by weakening Core checks.

Routine rebuilds, database resets and cleanup scripts do **not** clear this
node-local state or the Falco source log. Back up the state and source before
any explicitly authorized reset/rebinding, and reconcile retained evidence:
a fresh database no longer contains previous ingest deduplication receipts, so
replaying old queued records is not the same as replay into an unchanged DB.
Do not delete state to silence a binding/corruption error. Configure a separate
state path for a different Agent/cluster and retain the previous evidence under
an operator-approved retention decision. Without the state-path setting, the
legacy reader retries an ownership-rejected slice as a whole; do not claim the
durable mixed-batch fix is active in that mode.

## Signed source health (optional)

Core accepts runtime source health only when it is signed by an attestor that is independent of the Agent; the [protocol](../reference/RUNTIME_EVIDENCE.md#independently-signed-runtime-source-health-protocol-v1) defines the report and the `FORTUNA_SOURCE_HEALTH_REGISTRY` file.

The Agent can relay reports produced by an independent attestor. Configure:

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
