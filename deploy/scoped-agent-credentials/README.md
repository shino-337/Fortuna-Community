# Scoped HTTP agent credentials — C2 rollout

This overlay enables the scoped HTTP identity path for Agent inventory/evidence and
runtime v1/v2 ingest without changing the base manifests. When the Core registry
and Agent token file are configured, `/api/v1/agent/*`, `/api/v1/runtime/events`
and `/api/v2/runtime/events` all use the same per-agent credential. Deployments
that do not enable the overlay remain on the explicit legacy shared-token mode.

## Preconditions

- Deploy Core and Agent builds containing C2e2 sender retry protection and the C2e3
  runtime scoped-auth cutover.
- Use the canonical `clusterId` produced by cluster discovery.
- Confirm each Agent ID. The default is `<nodeName>-agent`; if `AGENT_ID` is
  overridden, provision with `--identity NODE=AGENT_ID`.
- Keep generated plaintext tokens outside the repository and backup path.
- Confirm the initial inventory sync can populate Pod ownership before runtime
  telemetry is expected to be accepted. Sender retries cover temporary convergence
  rejects; they are not a substitute for a healthy inventory path.

## 1. Issue per-node credentials

Example for default Agent IDs:

```bash
python3 scripts/deploy/agent-credential-tool.py issue \
  --cluster-id '<canonical-cluster-id>' \
  --node '<node-1>' \
  --node '<node-2>' \
  --output-dir '/secure/operator/path/agent-credentials'
```

Example when an Agent ID is overridden:

```bash
python3 scripts/deploy/agent-credential-tool.py issue \
  --cluster-id '<canonical-cluster-id>' \
  --identity '<node-1>=<explicit-agent-id>' \
  --output-dir '/secure/operator/path/agent-credentials'
```

The tool writes:

- `registry.json`: Core registry with SHA-256 token digests and identity bindings;
- `nodes/<node>/token`: the plaintext token for that node only, mode `0600`.

## 2. Install one token on each node

Copy only the matching node token. Do not distribute the whole output directory to
every node.

```bash
sudo install -d -m 0700 /etc/fortuna/agent-credentials
sudo install -m 0600 '/secure/source/token' /etc/fortuna/agent-credentials/token
```

Use SSH, Ansible, your node bootstrap system, or a secret CSI provider that can
provide a distinct file per node. The supplied DaemonSet patch uses node-local
`hostPath` because it works in the kubeadm/on-prem deployment without giving every
Agent pod access to a cluster-wide plaintext token set.

## 3. Publish the digest-only Core registry

```bash
kubectl -n fortuna create secret generic fortuna-agent-credential-registry \
  --from-file=registry.json='/secure/operator/path/agent-credentials/registry.json' \
  --dry-run=client -o yaml | kubectl apply -f -
```

The Core credential store rereads the projected registry for every authentication.
Kubernetes projected Secret updates therefore take effect without a Core restart.

## 4. Enable scoped mode

Apply both patches in the same maintenance window. Either order can create a short
period of rejected Agent or runtime requests while the other workload rolls; the
sender/reconciliation paths must recover after convergence.

```bash
kubectl -n fortuna patch deployment fortuna-core --type strategic \
  --patch-file deploy/scoped-agent-credentials/core-registry-patch.yaml
kubectl -n fortuna patch daemonset fortuna-agent --type strategic \
  --patch-file deploy/scoped-agent-credentials/agent-token-file-patch.yaml

kubectl -n fortuna rollout status deployment/fortuna-core
kubectl -n fortuna rollout status daemonset/fortuna-agent
```

Verify that:

- Agent `/sync` succeeds for every node and Core records the expected Agent ID;
- Pod metrics/process/network/event ingest succeeds after inventory convergence;
- runtime v1/v2 event ingest succeeds with the node's scoped credential;
- a node/cluster-A token cannot submit identity claims or runtime Pod UIDs owned by
  cluster B;
- a mixed runtime batch containing one foreign Pod is rejected as a whole and does
  not create a partial `runtime_events` prefix;
- Core does not log registry-unavailable, identity-mismatch, ownership-mismatch or
  repeated 401/403 after convergence.

## Rotation with overlap

Never replace the old registry entry first. Create the new token while preserving
the old entry, publish the overlapping registry, then replace the node token
atomically.

```bash
python3 scripts/deploy/agent-credential-tool.py issue \
  --cluster-id '<canonical-cluster-id>' \
  --node '<node-to-rotate>' \
  --existing-registry '/secure/current/registry.json' \
  --output-dir '/secure/rotation/new'

kubectl -n fortuna create secret generic fortuna-agent-credential-registry \
  --from-file=registry.json='/secure/rotation/new/registry.json' \
  --dry-run=client -o yaml | kubectl apply -f -

sudo install -m 0600 '/secure/rotation/new/nodes/<node-to-rotate>/token' \
  /etc/fortuna/agent-credentials/token.new
sudo mv /etc/fortuna/agent-credentials/token.new \
  /etc/fortuna/agent-credentials/token
```

The Agent rereads `FORTUNA_AGENT_TOKEN_FILE` for every scoped HTTP ingest request,
including runtime v1/v2, so an atomic file replacement does not require restarting
the pod. After successful sync/evidence/runtime verification, revoke the old
credential ID and republish the registry:

```bash
python3 scripts/deploy/agent-credential-tool.py revoke \
  --registry '/secure/rotation/new/registry.json' \
  --credential-id '<old-credential-id>'
```

## Rollback

Before old-credential revocation, rollback is restoring the previous node token
file while the overlapping registry still accepts it. If the old credential has
already been revoked, rollback requires an explicit operator registry rollback or
a newly issued credential. Once `FORTUNA_AGENT_TOKEN_FILE` is configured, scoped
Agent/runtime requests never silently fall back to the shared token.

To roll an installation all the way back to explicit legacy mode, remove both the
Core registry configuration and Agent token-file configuration in one controlled
change, then verify the shared token path. Do not remove only one side and assume
the other side will silently compensate.

## Security requirements

- Node credential directory: `0700`; token file: `0600`, root/operator managed.
- Never commit generated tokens, registry working directories, or copied Secrets.
- Do not mount one plaintext token set into every DaemonSet pod.
- `FORTUNA_INGEST_TOKEN` may remain in the base manifests for backward-compatible
  installations, but scoped HTTP ingest does not use it while the registry/token
  file are configured.
- Registry entries should overlap only for controlled rotation and should have a
  bounded expiration time.
