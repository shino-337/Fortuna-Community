# Kubernetes E2E Security Scenarios

Deployable manifests for **live-cluster** verification of Fortuna attack-path detection, MITRE mapping, runtime correlation, noisy resistance, capability realism, and risk scoring.

These are security regression fixtures, not production workloads. Run them only on an isolated test cluster.

**Scenario contract:** [`SCENARIO_CONTRACT.md`](./SCENARIO_CONTRACT.md)

## Deploy

```bash
kubectl apply -f scenarios/
```

Apply order is handled by multi-doc YAML filenames (`00-namespace.yaml` first).

Wait for workloads:

```bash
kubectl wait --for=condition=Ready pod -l fortuna.io/e2e-scenario -n fortuna-test --timeout=180s || true
kubectl get pods -n fortuna-test -o wide
```

## Prerequisites

| Requirement | Why |
|-------------|-----|
| Fortuna Core API reachable | Verifier queries attack paths and risk scores |
| `FORTUNA_CLUSTER_ID` matches Fortuna's cluster record | Graph/risk queries filter by cluster |
| Inventory sync has seen these pods | Paths/chains require pods in the Fortuna database |
| Optional: runtime ingest (Falco/agent → Fortuna) | S3/S4 runtime/MITRE assertions |
| Network egress for S3/S4 | Scanner + API activity scenarios |
| HostPath allowed for S1 | Some clusters restrict `hostPath`; the pod may remain Pending |

## Verify

From the repository root, after deployment and Fortuna inventory/path reconciliation:

```bash
export FORTUNA_API_URL="https://<fortuna-core-host>"
export FORTUNA_CLUSTER_ID="<fortuna-cluster-id>"
# If auth is enabled:
# export FORTUNA_JWT="<token>"

./scripts/verify-k8s-e2e.sh
```

The verifier is intentionally evidence-driven:

- **PASS** means a required security assertion was observed.
- **FAIL** means required evidence is missing or the Fortuna API could not be queried.
- **WARN** means an optional runtime capability was not available; it is not treated as a successful runtime verification.

## Scenarios

| ID | Manifest | Intent | Required evidence |
|----|----------|--------|-------------------|
| S1 | `s1-escape-privesc.yaml` | HostPath escape surface + `cluster-admin` binding | Pod-specific `ESCAPE_TO_PRIV_ESC` + `ESCAPE_HOSTPATH` |
| S2 | `s2-rbac-only.yaml` | Privilege via RBAC only | Attack path without `ESCAPE_PATH` classification |
| S3 | `s3-noisy-discovery.yaml` | High-volume discovery noise | Bounded risk/correlation behavior |
| S4 | `s4-token-lateral.yaml` | Repeated API calls with SA token | Token-reuse evidence when runtime ingest is enabled |
| S5 | `s5-broken-chain.yaml` | Default SA, no escalation wiring | No false-positive `ESCAPE_TO_PRIV_ESC` chain |

Optional lab manifests (same CRB/SA as S2; apply manually if needed):

| File | Intent |
|------|--------|
| `optional/s2-rbac-pod-master.yaml` | `rbac-pod` on control-plane node (Falco co-located with Fortuna on small clusters). Delete the existing `rbac-pod` before applying because pod scheduling fields are immutable. |
| `optional/s2-rbac-api-sim-pod.yaml` | `rbac-api-sim` with `curl` image for valid TLS to the Kubernetes API (busybox cannot) |

Attack-path design, MITRE, and `risk_signals`: [`docs/03-components/COMPONENTS.md#attack-path-and-risk-signals`](../docs/03-components/COMPONENTS.md#attack-path-and-risk-signals).

## Teardown

```bash
kubectl delete namespace fortuna-test
# ClusterRoleBindings are cluster-scoped:
kubectl delete clusterrolebinding crb-escape-admin crb-rbac-admin 2>/dev/null || true
```

If names collide with an existing cluster, rename bindings in the YAML before apply.
