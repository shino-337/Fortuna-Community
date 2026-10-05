# User Guide

Fortuna is organized around operational workspaces. Use the left navigation after signing in.

## Access

Open the dashboard URL exposed by port-forward or your ingress. Local development usually uses:

```bash
kubectl port-forward --address 0.0.0.0 -n fortuna svc/fortuna-dashboard 8081:80
```

Then open `http://127.0.0.1:8081/`.

Development login:

- Username: `admin`
- Password: `<FORTUNA_ADMIN_PASSWORD>`, or `Fortuna_ChangeMe_123!` for a fresh bootstrap deploy where no admin password was configured. The bootstrap default must be changed at first login and never overwrites an existing admin. When `FORTUNA_ADMIN_PASSWORD` is set, Core re-applies it on every restart, so change the admin password through that Secret rather than only in the UI.

Roles determine route visibility and actions:

| Role | Main purpose |
|------|--------------|
| Admin | Full platform, security, policy, monitoring, and user administration. |
| User admin | Fortuna account administration only. No cluster or finding access. |
| Operator | Day-to-day investigation, triage, rules, runtime, and risk workflows. |
| Viewer | Read-oriented security posture and evidence review. |

## Main Workspaces

| Workspace | Route | Use it for |
|-----------|-------|------------|
| Platform Integrity | `/#/` | Telemetry reliability, governance, runtime coverage, and platform health. |
| Operations Dashboard | `/#/dashboard` | Executive summary of risk, exposure, attack paths, and cluster posture. |
| Findings Queue | `/#/risks/findings` | Triage findings using the unified risk score and workflow status. |
| Attack Paths | `/#/attack-paths` | Review attack paths, RBAC escalation, lateral movement, and runtime attack-step evidence. |
| Runtime Network | `/#/network-activity` | Inspect pod-to-pod and external network activity. |
| Kubernetes Inventory | `/#/resources` | Browse pods and open pod detail for SBOM, risk, runtime, events, and spec. |
| Rules & Catalog | `/#/rules` | Detection rules and policies, risk scoring rules (`/#/rules/risk-scoring`) and the capability catalog (`/#/rules/catalog`). |
| Platform Health | `/#/monitoring` | Verify pipeline processing, runtime event ingestion, Falco/eBPF visibility and data freshness; certificates (`/#/monitoring/certificates`) and notifications (`/#/monitoring/notifications`). |
| Reports | `/#/reports` | Export and review time-windowed operational reports. |
| Settings | `/#/settings` | Manage users, roles and sessions. |

## Cluster Scope

The header cluster selector controls most security data pages.

- `All clusters` is useful for Platform Integrity, Operations Dashboard, global finding triage, and reports.
- A specific cluster is required for Runtime Network and is recommended when investigating pod detail, attack paths, or inventory.
- If a remote cluster Agent is connected, it appears in the selector after its first full sync. Dashboard totals should equal the sum of active cluster rows.

## Workspace Screenshots

These images are representative captures from one local multi-cluster deployment. Your data, cluster name, and health states may differ.

### Platform Integrity

![Platform Integrity](../assets/screenshots/platform-integrity.png)

### Platform Health

![Platform Health](../assets/screenshots/monitoring.png)

### Findings Queue

![Findings Queue](../assets/screenshots/risk-operations.png)

### Attack Paths

![Attack Paths](../assets/screenshots/attack-analysis.png)

### Runtime Network

![Runtime Network](../assets/screenshots/network-activity.png)

### Kubernetes Inventory

![Kubernetes Inventory](../assets/screenshots/resources.png)

### Rules & Catalog

![Rules & Catalog](../assets/screenshots/policy-rules.png)

### Reports

![Reports](../assets/screenshots/reports.png)

## Reading Empty or Blocked States

| State | Meaning | Action |
|-------|---------|--------|
| Unauthenticated | The JWT/session is missing or expired. | Sign in again. |
| Forbidden | Your role lacks the required permission. | Ask an admin to update role or permission grants. |
| Cluster scope | You selected or opened a cluster outside your assigned scope. | Change cluster selector or request access. |
| No telemetry | The page needs Agent or runtime data that has not arrived for this cluster. | Check Platform Health and the Agent or sensor for that cluster. |
| No data | The route is allowed and data is flowing, but current filters have no records. | Clear filters or widen the time range. |
| Stale | Data exists but its freshness checks failed, so it may not reflect the cluster now. | Check data timestamps and Agent sync before acting on it. |

## Investigation workflow

A typical investigation:

1. Start at Platform Integrity to confirm data freshness and runtime coverage.
2. Open Findings Queue and sort by unified risk score.
3. Open a finding drawer or full detail page to inspect evidence.
4. Jump to Attack Paths for path context.
5. Open the affected pod in Kubernetes Inventory for SBOM, runtime, network, and event detail.
6. Use Rules & Catalog to understand the rule or catalog entry behind the finding.
7. Export from Reports when you need a time-windowed operational handoff.

Each step is described below.

### 1. Confirm Platform Health Before Investigation

Goal: make sure missing data is not caused by ingestion or sensor failure.

Reference screens: [Platform Integrity](../assets/screenshots/platform-integrity.png), [Platform Health](../assets/screenshots/monitoring.png).

Steps:

1. Open `/#/`.
2. Check platform integrity, telemetry, runtime coverage, and governance indicators.
3. Open `/#/monitoring`.
4. Confirm pipeline processing activity, agent visibility, Falco/runtime event visibility, and recent data timestamps.

Decision rule: do not treat a quiet Findings Queue as safe until Platform Health confirms ingestion is healthy.

### 2. Triage High-Risk Findings

Goal: prioritize work using one user-facing risk value.

Reference screen: [Findings Queue](../assets/screenshots/risk-operations.png).

Steps:

1. Open `/#/risks/findings`.
2. Sort or filter by final risk level and score.
3. Open the finding drawer.
4. Review affected resource, evidence, linked rules, and workflow status.
5. Acknowledge, resolve, dismiss, or escalate based on role permissions.

Expected data source: `GET /api/v1/risk/insights`, risk summary APIs, and linked evidence APIs.

### 3. Investigate an Attack Path

Goal: explain how a compromised workload can reach a sensitive target.

Reference screen: [Attack Paths](../assets/screenshots/attack-analysis.png).

Steps:

1. Open `/#/attack-paths`.
2. Select a priority path.
3. Review graph nodes, edge labels, attack steps, confidence, and runtime evidence.
4. Open the source pod or linked finding for detail.
5. Validate whether the path is inventory-derived, runtime-supported, or both.

Expected data source: graph attack-path bundle and summary APIs, runtime attack-step evidence, RBAC inventory, and network telemetry.

### 4. Review a Pod Supply-Chain Posture

Goal: understand SBOM, CVE, malware package, and runtime context for a workload.

Reference screen: [Kubernetes Inventory](../assets/screenshots/resources.png).

Steps:

1. Open `/#/resources`.
2. Search by namespace, pod name, image, or risk.
3. Open pod detail.
4. Review SBOM/CVE, risk, runtime, process, network, event, and spec tabs.
5. Use linked findings to return to Findings Queue.

Expected data source: pod inventory, SBOM extraction results, CVE catalog matches, risk scores, runtime snapshots, and Kubernetes events.

### 5. Verify Runtime Network

Goal: distinguish in-cluster traffic, service traffic, and external destinations.

Reference screen: [Runtime Network](../assets/screenshots/network-activity.png).

Steps:

1. Open `/#/network-activity`.
2. Check topology, edge width, node type, and destination classification.
3. Use filters for namespace, direction, protocol, and time.
4. Open pod detail when an edge needs workload-level evidence.

Expected data source: agent network activity snapshots and runtime telemetry APIs.

### 6. Manage and Audit Rules

Goal: understand why a rule matched and whether it is catalog-backed.

Reference screen: [Rules & Catalog](../assets/screenshots/policy-rules.png).

Steps:

1. Open `/#/rules`.
2. Search by rule UID, name, category, severity, or source.
3. Open detail using `/#/rules/uid/<uid>`.
4. Review catalog metadata, matching behavior, affected findings, and linked capabilities.

Expected data source: rule catalog APIs and legacy code-to-rule mapping records.

### 7. Produce a Time-Windowed Report

Goal: create a focused operational summary for a review period.

Reference screen: [Reports](../assets/screenshots/reports.png).

Steps:

1. Open `/#/reports`.
2. Pick a time filter such as 1 day, 3 days, 7 days, or 30 days.
3. Review included findings, resource changes, runtime events, and posture summary.
4. Export only after confirming filters match the intended scope.

Expected data source: report APIs scoped by cluster, role, and time window.
