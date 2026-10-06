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
| Home | `/#/` | What needs your attention in the header scope. Four counts (needs triage, critical open, your open cases, exposed workloads), the five open findings with the highest risk that nobody has acknowledged, your open cases, a one-line data freshness check, new findings per day for 30 days by risk level, and **Export brief**. Admins also see a Platform card with agent and pipeline status. Every count uses the risk level. Old `/#/dashboard` links open Home. |
| Findings | `/#/risks/findings` | The triage queue. Views for Needs triage, Assigned to me, In review, Resolved, Dismissed and All, one row of filters, and a detail panel with every action the finding's state and your role allow, including who owns the finding. Press J and K to move through the queue and O to open the full page; after an action the panel moves to the next finding. Operators also get Runtime evidence. `/#/risks` opens the queue. |
| Cases | `/#/investigation` | Work that spans several findings. Views for Open, Mine, Closed and All; one case at `/#/investigation?case=<id>` shows its lifecycle, linked findings with their current status, affected assets, the timeline, remediation tasks and owner. **Add to case** on a finding, pod or attack path links it to an open case or starts a new one, and the finding panel says which cases already link it. **Close case** asks for a reason and can resolve the linked findings that are still open in the same step. |
| Attack Paths | `/#/attack-paths` | **Break these first** lists the fixes that cut the most paths. Below it, one list of paths (level, entry pod → target, confidence) and the selected path: its graph, steps, fix, and every pod, service account and binding on it, each linked. `?podUid=` keeps only the paths through one pod and `?path=` opens one path. |
| Network | `/#/network-activity` | Inspect pod-to-pod and external network activity. Link to one pod with `?clusterId=<id>&namespace=<ns>&podUid=<uid>`. |
| Inventory | `/#/resources` | Three views: **Workloads** ranked by risk with level counts, **Identities** (service accounts, roles and bindings, filtered by kind) and **Clusters**. Clicking a row opens a side panel with the workload's open findings and attack paths, or what an identity can do and who uses it. Pod detail opens with links to the pod's findings, attack paths, network flows and service account, and has an **Attack paths** tab. |
| Rules | `/#/rules` | Detection rules and policies, risk scoring rules (`/#/rules/risk-scoring`), the capability catalog (`/#/rules/catalog`) and capability exposure (`/#/rules/exposure`): which pods hold each capability, by namespace and severity. Old `/#/risks/pce` links redirect. |
| Platform | `/#/monitoring` | Whether Fortuna is collecting complete, fresh data from every cluster. One page, top to bottom: a verdict with the first problem and its fix, clusters and agents (reporting, last heartbeat, agent version), the pipeline stages with when each last produced data, certificates (manage them at `/#/monitoring/certificates`), errors in the last 24 hours grouped by message, and the full operational log. |
| Notifications | `/#/notifications` | Every alert, with unread and read views. Open it from **View all** in the bell. Old `/#/monitoring/notifications` links redirect. |
| Setup | `/#/setup` | Admin only, until its required steps are done: the first-run checklist (agent reporting, first scan, first finding triaged, team invited, and optionally another cluster). Each step checks itself from live data, and the sidebar shows how many are done. |
| Audit | `/#/governance` | Admin only, in three views: **Activity** (security activity, filterable by domain, severity, result and action), **Platform log** (counts by resource and action, then the full log) and **Access** (account hygiene, correlation signals and permissions by role). Old `?tab=` links open the view that now holds them. |
| Users & Access | `/#/settings` | Admin and User admin: manage users, roles, cluster access and other people's sessions. |
| Account | `/#/account` | Every role: your profile, password and your own sessions. Open it from the avatar menu in the header. |

## Cluster Scope

The scope button in the header sets the cluster and the time window (plus auto refresh) for most security data pages. The header also holds finding search, notifications and the account menu (Account, Sign out).

At most one system banner appears under the header: an active incident first, then several investigations competing for attention, then data that may be incomplete because a pipeline is behind.

- `All clusters` is useful for Home, the Executive brief, and global finding triage.
- A specific cluster is required for Network and is recommended when investigating pod detail, attack paths, or inventory.
- If a remote cluster Agent is connected, it appears in the scope menu after its first full sync. Dashboard totals should equal the sum of active cluster rows.

## Workspace Screenshots

These images are representative captures from one local multi-cluster deployment. Your data, cluster name, and health states may differ.

### Home

![Home](../assets/screenshots/platform-integrity.png)

### Platform

![Platform](../assets/screenshots/monitoring.png)

### Findings

![Findings](../assets/screenshots/risk-operations.png)

### Attack Paths

![Attack Paths](../assets/screenshots/attack-analysis.png)

### Network

![Network](../assets/screenshots/network-activity.png)

### Inventory

![Inventory](../assets/screenshots/resources.png)

### Rules

![Rules](../assets/screenshots/policy-rules.png)

### Executive brief

![Executive brief](../assets/screenshots/reports.png)

This capture was taken when the brief was still a separate Reports page; the same cards now sit in the Executive brief section at the bottom of Home.

## Reading Empty or Blocked States

| State | Meaning | Action |
|-------|---------|--------|
| Unauthenticated | The JWT/session is missing or expired. | Sign in again. |
| Forbidden | Your role lacks the required permission. | Ask an admin to update role or permission grants. |
| Cluster scope | You selected or opened a cluster outside your assigned scope. | Change cluster selector or request access. |
| No telemetry | The page needs Agent or runtime data that has not arrived for this cluster. | Check Platform and the Agent or sensor for that cluster. |
| No data | The route is allowed and data is flowing, but current filters have no records. | Clear filters or widen the time range. |
| Stale | Data exists but its freshness checks failed, so it may not reflect the cluster now. | Check data timestamps and Agent sync before acting on it. |

## Investigation workflow

A typical investigation:

1. Start at Home to confirm data freshness and runtime coverage.
2. Open Findings and sort by unified risk score.
3. Open a finding drawer or full detail page to inspect evidence.
4. When several findings are one incident, use **Add to case** and work it from the case page.
5. Jump to Attack Paths for path context.
6. Open the affected pod in Inventory for SBOM, runtime, network, and event detail.
7. Use Rules to understand the rule or catalog entry behind the finding.
8. Close the case, resolving its findings, or export from the Executive brief on Home when you need a time-windowed operational handoff.

Each step is described below.

### 1. Confirm Platform Before Investigation

Goal: make sure missing data is not caused by ingestion or sensor failure.

Reference screens: [Home](../assets/screenshots/platform-integrity.png), [Platform](../assets/screenshots/monitoring.png).

Steps:

1. Open `/#/`.
2. Check platform integrity, telemetry, runtime coverage, and governance indicators.
3. Open `/#/monitoring`.
4. Confirm pipeline processing activity, agent visibility, Falco/runtime event visibility, and recent data timestamps.

Decision rule: do not treat a quiet Findings as safe until Platform confirms ingestion is healthy.

### 2. Triage High-Risk Findings

Goal: prioritize work using one user-facing risk value.

Reference screen: [Findings](../assets/screenshots/risk-operations.png).

Steps:

1. Open `/#/risks/findings`. It starts on **Needs triage**, highest risk first.
2. Narrow it with the risk level, namespace, type or search filters.
3. Select a finding, or press J, to open the detail panel.
4. Review affected resource, evidence, linked rules, and workflow status.
5. Use **Take it** to own the finding, or **Assign…** to hand it to a teammate. Only people who can triage that finding (its cluster is in their scope) are offered. Your open findings are in the **Assigned to me** view, and Home shows how many there are.
6. Acknowledge, resolve, dismiss or add it to a case, as your role allows. Resolve and dismiss ask for a reason, which goes to the audit trail. The panel then moves to the next finding.

Expected data source: `GET /api/v1/risk/insights` (`assignee=me` for Assigned to me), `PUT /api/v1/risk/insights/:id/assignee`, `POST /api/v1/risk/insights/bulk` and linked evidence APIs.

### 3. Investigate an Attack Path

Goal: explain how a compromised workload can reach a sensitive target.

Reference screen: [Attack Paths](../assets/screenshots/attack-analysis.png).

Steps:

1. Open `/#/attack-paths`.
2. Start with **Break these first**, or pick a path from the list by level.
3. Read its steps and fix; steps marked **seen at runtime** matched runtime events.
4. Open a pod, its findings or its network flows from **On this path**.
5. Check the confidence (Confirmed, Probable, Theoretical) and **Evidence and assumptions** before acting.

Expected data source: graph attack-path bundle and summary APIs, runtime attack-step evidence, RBAC inventory, and network telemetry.

### 4. Review a Pod Supply-Chain Posture

Goal: understand SBOM, CVE, malware package, and runtime context for a workload.

Reference screen: [Inventory](../assets/screenshots/resources.png).

Steps:

1. Open `/#/resources`.
2. Search by pod name, namespace, node or UID, or pick a risk level.
3. Click a row to preview its findings and attack paths, or open pod detail.
4. Review the Risk & SBOM, Attack paths, Runtime, Network and Spec tabs.
5. Use **Findings** at the top of pod detail to return to that pod's findings.

Expected data source: pod inventory, SBOM extraction results, CVE catalog matches, risk scores, runtime snapshots, and Kubernetes events.

### 5. Verify Network

Goal: distinguish in-cluster traffic, service traffic, and external destinations.

Reference screen: [Network](../assets/screenshots/network-activity.png).

Steps:

1. Open `/#/network-activity`.
2. Check topology, edge width, node type, and destination classification.
3. Use filters for namespace, direction, protocol, and time.
4. Open pod detail when an edge needs workload-level evidence.

Expected data source: agent network activity snapshots and runtime telemetry APIs.

### 6. Manage and Audit Rules

Goal: understand why a rule matched and whether it is catalog-backed.

Reference screen: [Rules](../assets/screenshots/policy-rules.png).

Steps:

1. Open `/#/rules`.
2. Search by rule UID, name, category, severity, or source.
3. Open detail using `/#/rules/uid/<uid>`.
4. Review catalog metadata, matching behavior, affected findings, and linked capabilities.

Expected data source: rule catalog APIs and legacy code-to-rule mapping records.

### 7. Produce a Time-Windowed Report

Goal: create a focused operational summary for a review period.

Reference screen: [Executive brief](../assets/screenshots/reports.png).

Steps:

1. Open Home and expand **Executive brief** at the bottom, or open `/#/?section=brief` (old `/#/reports` links land here).
2. Pick a report window: 1, 3, 7 or 30 days. Findings follow the window; pipeline health, inventory and investigation counts are current snapshots.
3. Review critical exposure, critical attack paths, runtime exploited signals, fleet inventory, open investigations and the finding distribution.
4. Use **Download brief** for a Markdown summary, or **Risks CSV** / **Risks PDF** (needs `export.findings`) for the findings in the window.

The brief only loads when it is expanded. Its numbers come from `/dashboard/stats`, `/risk/insights/summary`, `/monitoring/pipeline-health` and `/investigations/stats`, scoped by your clusters and permissions.

Audit counts by resource and action, and the full audit log, are on the Audit page's Platform log (`/#/governance?tab=platform`) and need `system.audit.read`.
