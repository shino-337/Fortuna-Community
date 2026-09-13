# Your first RBAC investigation

Use this walkthrough after installing Fortuna in an **isolated, disposable cluster**. It uses the existing S2 fixture, which grants a test ServiceAccount `cluster-admin`. Do not apply it to a production or shared cluster.

This is a manual validation procedure, not a recorded successful run. Record your version and results. The expected path is pod → ServiceAccount → ClusterRoleBinding → ClusterRole.

## Before you begin

- Fortuna is running and has collected inventory from the selected cluster.
- You can open the dashboard and inspect Attack Paths and Kubernetes Inventory.
- Your kubectl identity can create the fixture and impersonate a ServiceAccount for `auth can-i` checks.
- Use the scenario files from your selected checkout; the commands below use the existing S2 object names.
- Check `kubectl config current-context`. Stop if this is not your disposable lab.
- Check for existing `fortuna-test` namespace and `crb-rbac-admin` ClusterRoleBinding. If either already exists, use a fresh lab to avoid changing someone else's fixtures.

## 1. Create only the S2 fixture

From the repository root:

```bash
kubectl apply -f scenarios/00-namespace.yaml
kubectl apply -f scenarios/s2-rbac-only.yaml
kubectl wait --for=condition=Ready pod/rbac-pod -n fortuna-test --timeout=180s
```

Do not apply the entire scenarios directory for this walkthrough. Other fixtures exercise additional behaviors.

## 2. Check the effective permission

```bash
kubectl auth can-i get secrets --all-namespaces   --as=system:serviceaccount:fortuna-test:sa-rbac
```

Expected output: `yes`. This checks authorization without reading Secret values. An impersonation error, connection error, or timeout is not a negative authorization result.

Inspect the chain:

```bash
kubectl get pod rbac-pod -n fortuna-test -o yaml
kubectl get serviceaccount sa-rbac -n fortuna-test -o yaml
kubectl get clusterrolebinding crb-rbac-admin -o yaml
kubectl get clusterrole cluster-admin -o yaml
```

## 3. Find the evidence in Fortuna

Wait for an inventory sync and graph/risk reconciliation. The current main Agent manifest sets `SYNC_INTERVAL` to `5m`; do not assume the dashboard updates immediately. Check pipeline health and the data timestamps if the workload is missing.

1. Select the lab cluster in the dashboard.
2. Find `fortuna-test/rbac-pod` in Kubernetes Inventory.
3. Inspect its ServiceAccount and RBAC evidence.
4. Open Attack Paths and locate the path for this workload.
5. Record the pod, ServiceAccount, binding, role, path classification, and evidence timestamp.

Expected: an RBAC permission path associated with this pod. S2 should not become a hostPath escape merely because it has powerful RBAC. Do not require a particular numeric risk score: scoring changes by version and available evidence.

The sleeping pod does not simulate a compromise. A static permission path alone must not be reported as an observed attack.

## 4. Remove the dangerous grant and check again

```bash
kubectl delete clusterrolebinding crb-rbac-admin
kubectl auth can-i get secrets --all-namespaces   --as=system:serviceaccount:fortuna-test:sa-rbac
```

Expected output in a clean lab: `no` (kubectl returns a nonzero status for denial). If it is `yes`, inspect other grants to this identity before drawing conclusions.

Wait for Fortuna to ingest the removal and reconcile. Check whether the active path still depends on the removed binding. Historical findings may remain; inspect their status and timestamps. If a current path still claims the removed grant, record that discrepancy as feedback rather than declaring success.

| Check | Before removal | After removal |
|---|---|---|
| Kubernetes authorization | `yes` | `no`, absent another grant |
| Binding | Present | Absent |
| Fortuna evidence | Workload-specific RBAC path | Removed grant must not remain valid current evidence after reconciliation |
| Runtime compromise | Not demonstrated | Not demonstrated |

## 5. Clean up

Only run this cleanup for the fresh lab namespace created above:

```bash
kubectl delete -f scenarios/s2-rbac-only.yaml --ignore-not-found
kubectl delete -f scenarios/00-namespace.yaml --ignore-not-found
```

Deleting the namespace alone does not remove a ClusterRoleBinding. The first command removes the cluster-scoped fixture too.

## Share the result

Record the Fortuna version/commit, Kubernetes version, runtime, sync timestamps, authorization outputs, and expected versus observed path. Redact tokens, credentials, and sensitive identifiers. Use the repository's feedback channels when enabled; security vulnerabilities follow [SECURITY.md](../../SECURITY.md).
