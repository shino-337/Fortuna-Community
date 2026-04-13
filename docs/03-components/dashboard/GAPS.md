# Dashboard — Known Gaps

**Last Updated**: 2026-04-13

## Open Gaps

| ID | Description | Priority | Status |
|----|-------------|----------|--------|
| UI-1 | Global cluster selector/filter not implemented — no `useClusterStore` in header | P2 | Planned |
| UI-2 | Risk Center missing Status filter dropdown (only severity filter exists) | P2 | Known |
| UI-3 | Risk Center missing "Type" column (insight_type: vulnerability, rbac, etc.) | P2 | Known |
| UI-4 | "Assets" column shows only namespace; should show affected asset count/summary | P3 | Known |
| UI-5 | Cluster Detail page not implemented — no tabs (Overview/Inventory/Agents/Security) | P2 | Planned |
| UI-6 | `affectedPodCount` not available from API — Dashboard Home can't show "Affected Workloads" | P2 | Needs Backend |
| UI-7 | Cluster stats missing `RiskCount` and `AgentCount` per cluster | P2 | Needs Backend |
| UI-8 | Dashboard Data Integrity — some data paths not fully end-to-end verified | P2 | In Progress |
| UI-9 | "Catalog" vs "Metadata" naming inconsistency for capability browser | P3 | Known |
| UI-10 | Node Detail page not implemented | P3 | Planned |
| UI-11 | Identity Detail page incomplete | P3 | Planned |

## Completed

| Item | Description |
|------|-------------|
| Risk Center | Severity filter, risk table, click→detail, runtime signals reference tab |
| Pod Detail | Full 6-tab view (runtime, processes, network, events, security, overview) |
| Network Activity | 4 views (connections, pods, destinations, talkers) with cluster/namespace filter |
| SBOM Analysis | Pod SBOM list, component detail, CVE matches |
| Capabilities | Full metadata browser with MITRE ATT&CK display |
| Auto-refresh | Polling with configurable interval |
| Time window | sinceMinutes filter for Risk Center and Runtime Signals |
| Cross-links | Resources Explorer → Pod Detail → Cluster → Node navigation |
