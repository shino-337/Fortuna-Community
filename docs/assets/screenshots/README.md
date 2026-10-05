# Dashboard Screenshots

These screenshots are committed as user-guide assets. They show a local multi-cluster Fortuna deployment at the time the documentation was updated, so exact counts, cluster names, and health states may differ in another environment.

## Current Assets

| File | Route | Documentation Use |
|------|-------|-------------------|
| `platform-integrity.png` | `/#/` | Home, top: the role's next step (Platform Integrity for admins) |
| `dashboard-overview.png` | `/#/` | Home, risk overview and drill-downs (captured as the former `/#/dashboard` page) |
| `monitoring.png` | `/#/monitoring` | Platform Health |
| `risk-operations.png` | `/#/risks/findings` | Findings Queue |
| `attack-analysis.png` | `/#/attack-paths` | Attack Paths |
| `network-activity.png` | `/#/network-activity` with a selected cluster | Runtime Network |
| `resources.png` | `/#/resources` | Kubernetes Inventory and pod investigation entry point |
| `policy-rules.png` | `/#/rules` | Rules & Catalog |
| `reports.png` | `/#/?section=brief` | Executive brief on Home (captured as the former `/#/reports` page) |

## Refresh Guidance

Use a deployed dashboard and capture at `1440x1000` so the images stay comparable across documentation updates. Use `All clusters` for global pages and a real selected cluster for cluster-scoped pages such as Runtime Network. Do not commit screenshots that expose private customer names, tokens, kubeconfigs, credentials, or incident data.
