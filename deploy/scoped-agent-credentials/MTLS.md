# Per-Agent gRPC certificates and composite identity

Core now persists Agents by `(cluster_id, agent_id)`. Two clusters may use the same
node/Agent name. Legacy rows with an empty cluster stay quarantined; registration
creates a new owned row rather than adopting them. PostgreSQL installs the new
unique key before removing global AgentID uniqueness. Ownership is immutable;
soft-delete restoration affects only the exact composite identity.

Without `FORTUNA_GRPC_AGENT_CREDENTIAL_REGISTRY`, Core refuses all AgentService
writes and streams, including clients holding the old shared certificate. Only a
read-only Ping returns `identity_required`. Provision before upgrading Core; do
not interpret endpoint reachability as authenticated ingest readiness.

## Issue and install

Keep the signing CA private key on the operator host; never copy it to Agent
nodes or a Kubernetes Secret. Core's server certificate must remain valid for
`fortuna-core.fortuna.svc.cluster.local`. The Agent verifies that server using
the existing `fortuna-ca-cert` Secret; its client certificate may be signed by
a separate, dedicated Agent CA.

If the original Fortuna CA private key is unavailable, do **not** run
`MTLS_REGEN=1 scripts/utils/create_mtls_secret.sh` merely to issue Agent certs:
that replaces the Core and webhook certificates and the shared trust Secret.
Instead, create a dedicated Agent-client CA in a private operator directory,
then install only its public certificate as Core's client trust root:

```bash
install -d -m 0700 /secure/agent-client-ca
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes \
  -keyout /secure/agent-client-ca/ca.key -out /secure/agent-client-ca/ca.crt \
  -days 365 -sha256 -subj '/CN=Fortuna Agent Client CA/O=Fortuna' \
  -addext 'basicConstraints=critical,CA:TRUE,pathlen:0' \
  -addext 'keyUsage=critical,keyCertSign,cRLSign'
chmod 0600 /secure/agent-client-ca/ca.key
kubectl -n fortuna create secret generic fortuna-agent-client-ca \
  --from-file=ca.crt=/secure/agent-client-ca/ca.crt \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl -n fortuna patch deployment fortuna-core --type strategic \
  --patch-file deploy/scoped-agent-credentials/core-client-ca-patch.yaml
```

Core's `TLS_CA_CERT_PATH` is its gRPC **client** trust pool, separate from the
server certificate/key. This patch keeps `fortuna-core-tls`, `fortuna-ca-cert`,
and the webhook Secret unchanged. The Agent mTLS overlay keeps
`TLS_CA_CERT_PATH=/etc/fortuna/ca-cert/ca.crt` to verify Core's existing server
certificate. Preserve the new CA key securely for future 30-day leaf renewals;
back it up outside the repository and plan CA renewal before its expiry.

```bash
python3 scripts/deploy/agent-certificate-tool.py issue \
  --cluster-id '<canonical-cluster-id>' --node '<node-name>' \
  --ca-cert '/secure/ca.crt' --ca-key '/secure/ca.key' \
  --ttl-days 30 --output-dir '/secure/generation-1'
```

Repeat `--node` for additional nodes. For a configured AGENT_ID override, use
`--identity NODE=AGENT_ID`. Different clusters may use identical Agent IDs; each
must receive a different private key and fingerprint. Extend the aggregate Core
registry using `--existing-registry` when issuing for another cluster.

The tool validates the registry and CA lifetime, issues EC P-256 client-only leaf
certificates with random serials, verifies the resulting chain, and publishes a
new private output directory. Existing output directories are refused. The Core
registry contains fingerprints, exact validity times and ownership, never keys.
Only copy `nodes/<this-node>/` to the corresponding node. Keep its key mode 0600
and directories private to the Agent's runtime UID (adjust ownership if non-root).
The tool also copies the signing CA certificate into that directory for offline
chain verification; the Agent overlay uses the existing Core-server CA mount for
TLS server verification.

Store generations beneath `/etc/fortuna/agent-mtls/releases/` on each node. Switch
`current` using a relative symlink rename on that same filesystem:

```bash
# Run on the intended node after installing its generation-1 directory.
cd /etc/fortuna/agent-mtls
ln -s releases/generation-1 current.next
mv -Tf current.next current
```

Mount the whole parent directory, not a file subPath, so rotation reaches the pod.
The parent contains only that node's credentials, never every Agent's keys.

```bash
kubectl -n fortuna create secret generic fortuna-agent-mtls-registry \
  --from-file=registry.json='/secure/generation-1/registry.json' \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl -n fortuna patch deployment fortuna-core --type strategic \
  --patch-file deploy/scoped-agent-credentials/core-mtls-registry-patch.yaml
kubectl -n fortuna patch daemonset fortuna-agent --type strategic \
  --patch-file deploy/scoped-agent-credentials/agent-mtls-patch.yaml
```

Keep the HTTP token overlay enabled independently: initial HTTP inventory sync
must establish Pod ownership before gRPC SBOM requests can be authorized. Wait for
both rollouts; verify each cluster/Agent registration and SBOM ingest. Shared
certificate-only deployments will experience rejected ingest until provisioned.

## Rotate, revoke, roll back

1. Issue into a new directory with `--existing-registry` pointing to the current
   aggregate registry. Retain old entries during overlap.
2. Publish the overlapping registry to Core; wait for its projected Secret update
   on every Core replica before switching Agent credentials.
3. Install the new generation only on its node and atomically replace `current`.
   Restart that Agent pod to establish a new connection immediately. Credentials
   are also reloaded on every new TLS handshake; a current connection retains the
   old certificate until reconnect. Invalid files fail closed without cached keys.
4. Verify registration/ingest under the new fingerprint before revoking the old ID.

```bash
python3 scripts/deploy/agent-certificate-tool.py revoke \
  --registry '/secure/current/registry.json' --credential-id '<old-credential-id>'
# Republish this registry using the Secret command above.
```

Core reauthenticates unary calls and stream messages. Revocation that occurs while
RecvMsg blocks is checked again before the received message reaches the handler.
Propagation is bounded by projected Secret delivery; it is not instantaneous on
all replicas. Revocation does not cancel a unary operation already authorized and
executing. Revoke compromised credentials without overlap; temporary ingest
failure is preferable to continued acceptance of the compromised key.

Rollback during overlap: switch the node symlink back and restart that pod while
the old entry is still valid. After revocation, issue a new credential; do not
reactivate a compromised key. Do not remove the registry to restore legacy access:
that quarantines writes. Downgrading Core to a global-AgentID writer is incompatible
with duplicate IDs now stored across clusters and needs a separate data migration.
CA rollover is a separate trust-store rollout; the client root pool is loaded at
connection setup, so restart/reconnect clients after updating their trust bundle.

## Required evidence

CI covers populated PostgreSQL cutover/rerun, concurrent duplicate Agent IDs,
legacy quarantine, scoped restoration, read-only legacy Ping, blocked legacy
streams, certificate issuance/overlap/revocation, key-file rotation and revocation
during a blocked receive. The two-cluster CI gate
(`scripts/verify/run-two-cluster-integration.py`) runs six scoped Agents across two
kind clusters. Your own rollout still needs the checks above on each cluster.
