# Main Use Cases

## 1. Confirm Platform Health Before Investigation

Goal: make sure missing data is not caused by ingestion or sensor failure.

Reference screens: [Platform Integrity](../assets/screenshots/platform-integrity.png), [Pipeline & Runtime Health](../assets/screenshots/monitoring.png).

Steps:

1. Open `/#/`.
2. Check platform integrity, telemetry, runtime coverage, and governance indicators.
3. Open `/#/monitoring`.
4. Confirm pipeline processing activity, agent visibility, Falco/runtime event visibility, and recent data timestamps.

Decision rule: do not treat a quiet Findings Queue as safe until Pipeline & Runtime Health confirms ingestion is healthy.

## 2. Triage High-Risk Findings

Goal: prioritize work using one user-facing risk value.

Reference screen: [Findings Queue](../assets/screenshots/risk-operations.png).

Steps:

1. Open `/#/risks/findings`.
2. Sort or filter by final risk level and score.
3. Open the finding drawer.
4. Review affected resource, evidence, linked rules, and workflow status.
5. Acknowledge, resolve, dismiss, or escalate based on role permissions.

Expected data source: `GET /api/v1/risk/insights`, risk summary APIs, and linked evidence APIs.

## 3. Investigate an Attack Path

Goal: explain how a compromised workload can reach a sensitive target.

Reference screen: [Attack Paths](../assets/screenshots/attack-analysis.png).

Steps:

1. Open `/#/attack-paths`.
2. Select a priority path.
3. Review graph nodes, edge labels, attack steps, confidence, and runtime evidence.
4. Open the source pod or linked finding for detail.
5. Validate whether the path is inventory-derived, runtime-supported, or both.

Expected data source: graph attack-path bundle and summary APIs, runtime attack-step evidence, RBAC inventory, and network telemetry.

## 4. Review a Pod Supply-Chain Posture

Goal: understand SBOM, CVE, malware package, and runtime context for a workload.

Reference screen: [Kubernetes Inventory](../assets/screenshots/resources.png).

Steps:

1. Open `/#/resources`.
2. Search by namespace, pod name, image, or risk.
3. Open pod detail.
4. Review SBOM/CVE, risk, runtime, process, network, event, and spec tabs.
5. Use linked findings to return to Findings Queue.

Expected data source: pod inventory, SBOM extraction results, CVE catalog matches, risk scores, runtime snapshots, and Kubernetes events.

## 5. Verify Runtime Network

Goal: distinguish in-cluster traffic, service traffic, and external destinations.

Reference screen: [Runtime Network](../assets/screenshots/network-activity.png).

Steps:

1. Open `/#/network-activity`.
2. Check topology, edge width, node type, and destination classification.
3. Use filters for namespace, direction, protocol, and time.
4. Open pod detail when an edge needs workload-level evidence.

Expected data source: agent network activity snapshots and runtime telemetry APIs.

## 6. Manage and Audit Policy Rules

Goal: understand why a rule matched and whether it is catalog-backed.

Reference screen: [Policy Rules](../assets/screenshots/policy-rules.png).

Steps:

1. Open `/#/rules`.
2. Search by rule UID, name, category, severity, or source.
3. Open detail using `/#/rules/uid/<uid>`.
4. Review catalog metadata, matching behavior, affected findings, and linked capabilities.

Expected data source: rule catalog APIs and legacy code-to-rule mapping records.

## 7. Produce a Time-Windowed Report

Goal: create a focused operational summary for a review period.

Reference screen: [Reports](../assets/screenshots/reports.png).

Steps:

1. Open `/#/reports`.
2. Pick a time filter such as 1 day, 3 days, 7 days, or 30 days.
3. Review included findings, resource changes, runtime events, and posture summary.
4. Export only after confirming filters match the intended scope.

Expected data source: report APIs scoped by cluster, role, and time window.
