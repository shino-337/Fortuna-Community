# Kubernetes E2E workloads (Fortuna validation)

Deployable manifests for **live-cluster** verification of attack-path detection, MITRE mapping, runtime correlation, noisy resistance, capability realism, and risk scoring.

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

## Prerequisites (production parity)

| Requirement | Why |
|-------------|-----|
| Fortuna Core API reachable | Script uses `GET/POST /api/v1/...` |
| `CLUSTER_ID` matches Fortuna’s cluster record | Graph/risk queries filter by cluster |
| Inventory sync has seen these pods | Paths/chains require pods in DB |
| Optional: runtime ingest (Falco/agent → Fortuna) | S3/S4 MITRE / grounding / correlation |
| Network egress for S3/S4 | Scanner + curl need outbound traffic |
| HostPath allowed for S1 | Some clusters restrict `hostPath`; Pod may stay Pending |

## Run automated checks

From repo root (after deploy + Fortuna sync + rebuild as you described):

```bash
export FORTUNA_API_URL="https://<fortuna-core-host>"
export FORTUNA_CLUSTER_ID="<cluster-uuid-or-id>"
# If auth enabled:
# export FORTUNA_JWT="<token>"

./scripts/verify-k8s-e2e.sh
```

## Scenarios

| ID | Manifest | Intent |
|----|----------|--------|
| S1 | `s1-escape-privesc.yaml` | HostPath escape surface + `cluster-admin` binding |
| S2 | `s2-rbac-only.yaml` | Privilege via RBAC only (no escape class) |
| S3 | `s3-noisy-discovery.yaml` | High-volume discovery noise (anti-noise / correlation) |
| S4 | `s4-token-lateral.yaml` | Repeated API calls with SA token |
| S5 | `s5-broken-chain.yaml` | Default SA, no escalation wiring — gaps / low realism |

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
