# Changelog

## Unreleased

### Security

- Assigning a finding needs `findings.ack` and the finding in the caller's cluster scope (otherwise 404), and the person assigned must be active, hold `findings.ack` and have the finding's cluster in scope. An ineligible user and an unknown user id get the same 422, so the endpoint does not reveal which accounts exist. Every change is written to the security audit log as `findings.assignee.set` with the owner before and after.
- Adding a finding to a case now checks that the finding exists and is in the user's cluster scope, links it by id, and refuses a finding from another cluster than the case's. Before, any label and link were accepted, and the case's snapshot could copy a finding from a cluster the user cannot read.
- The findings export (CSV and print-ready HTML) now applies the user's cluster scope; a cluster-scoped user could previously export findings from every cluster. CSV cells that start with a formula character are prefixed with `'`.
- Notifications, agent status and the counts in `/metrics/system` follow the user's cluster scope. Marking notifications read only affects notifications in scope.
- Revoking another user's session needs `sessions.revoke_all` instead of `users.read`, so a User admin can no longer sign an Admin out.
- A User admin can no longer change, disable or delete Cluster admin accounts, matching the rule that they cannot create them.
- Every list endpoint has a default and a maximum `limit` (31 routes were unbounded, including users, resources, investigations, graphs and policy rules), and reports `truncated` when rows are cut. `limit=-1` on rule matches no longer returns every row.

### Core

- `GET /inventory/pods` takes `level` (`critical`, `high`, `medium`, `low` or `unscored`, from the pod's latest risk score with the finding bands) and `serviceAccount`, and returns `levelCounts` per level for the other filters when called with `withLevelCounts=1`. An unknown `level` returns 400.
- `GET /risk/insights` and the findings export take `resourceUid`, the findings on one workload. The pod risk report (`/risk/pods/:uid/report`) adds `insightLevels`, each finding's risk level as on the Findings list. The attack-path summary counts paths with the path bands (9+ critical, 7+ high, 4+ medium) and adds `lowPaths`; before, every path under 7.0 was counted as medium.
- Findings have an owner. `PUT /risk/insights/:id/assignee` with `{"userId": <id>|null}` sets or clears it (closed findings return 409), and `GET /risk/insights/:id/assignees` lists the people who can be assigned that finding. `GET /risk/insights` and the export take `assignee=me|none`, and `status=open` returns findings that need triage or are in review. Migration 153 adds `assignee_user_id`, `assignee_username` and `assigned_at` to `insights`; it runs on startup and only adds nullable or defaulted columns.
- `GET /investigations/:id/findings` returns a case's linked findings with their current status and risk level; findings outside the caller's clusters are counted in `hidden`, not returned. `GET /investigations?findingId=<id>` lists the cases that link a finding.
- `GET /risk/insights` sorts server-side: `sort` (score, severity, detected, updated, title, type, resource, namespace, status) with `order=asc|desc`, applied across all pages and part of the list cache key. Unknown values fall back to newest-detected-first, and every ordering ends with the finding id so pages are stable on ties.
- Statistics endpoints aggregate in SQL instead of loading every row: `/risk/trends` and `/pod-capabilities/trends` group per UTC day, `/investigations/stats` counts open cases in SQL and only reads cases whose remediation JSON mentions a due date, `/risk/scores` filters, counts, sorts and pages in the database (with the shared `pageSize` default of 50 and maximum of 500), and the access review counts users and reads only accounts that can raise a signal. Responses, cluster scope and permissions are unchanged.

### Dashboard

- Inventory is rebuilt around three views instead of seven tabs and a separate Clusters page. **Workloads** ranks pods by risk, with level chips that show the counts and filter the list, and each row shows its findings, its attack paths (and whether it is an entry point or pivot) and the service account it runs as, each a link. **Identities** lists service accounts, roles and bindings together with a kind filter. **Clusters** lists every cluster; choosing one scopes the app to it. Clicking a row opens a side panel instead of an inspector below the table: a workload's top open findings and attack paths, or what a service account can do, which bindings grant it and which workloads use it. Filters, the view and the page are in the URL; old `?tab=` links and `/resources/clusters` still work. Pod detail opens with links to the pod's findings, attack paths, network flows and service account in place of the facts grid, keeps its tab in the URL, has an **Attack paths** tab, and drops the Related navigation footer and the agent-sync hint. Node detail links to its workloads ranked by risk in Inventory.
- Pages that show the same workload now link to each other and keep its cluster. Pod detail links to the pod's findings (only that pod, instead of a name search that also matched same-named pods), its attack paths and its network flows, and its attack-path count opens Attack Paths. A pod selected on the Network map links to its findings and attack paths. A case's affected workloads link to their findings, and its Attack paths link keeps the cluster. Findings shows a **Workload** chip when it is narrowed to one workload. On Attack Paths, **Findings on the entry pod** replaces **View related findings**, which opened every finding in every cluster, and an alert's link (`?path=`) opens that path's scenario. Cluster detail's **View findings in this cluster** keeps the cluster.
- Attack Paths uses one set of levels: a path card's level now matches the counts above it and the alerts (a path at 8.2/10 is high, not critical), and the counts include **Low**. Findings on pod detail show their risk level instead of "No score".
- A finding can be owned by one person. The detail panel and the finding page show the owner, with **Take it** to own it in one click and **Assign…** to pick a teammate who can triage it. Findings has an **Assigned to me** view (your findings that need triage or are in review), queue rows show the owner, and Home shows how many findings are assigned to you. Only roles that can acknowledge findings see these controls.
- Capability exposure moved from Findings to Rules (`/rules/exposure`), next to the Capability catalog it is the other half of; `/risks/pce` redirects. Findings keeps the triage queue and Runtime evidence.
- Audit has three views instead of seven: **Activity** (security activity, now filterable by domain, which replaces the Investigation timeline tab that showed the same events), **Platform log** (the audit summary above the full log) and **Access** (access review, correlation signals and permissions by role on one page). Old `?tab=` values open the view that holds them. Removed the admin-only Audit workspace strip.
- Platform is one page that answers whether Fortuna is collecting complete, fresh data from every cluster: a verdict line with the first problem and a link to fix it, clusters and agents grouped by cluster (agents up, last heartbeat, agent version), the pipeline stages from inventory and runtime events to risk scores and the vulnerability feed with when each last produced data, certificates expiring within 30 days, errors in the last 24 hours grouped by message, and the operational log. It replaces the Pipeline overview, Runtime monitor, Catalog detail, Agents & certificates and Alerts / anomalies cards. Notifications moved out of Platform to `/notifications` (the bell's **View all**); `/monitoring/notifications` redirects.
- **Setup** (`/setup`, admins) is a first-run checklist: an agent reporting, the first scan, the first finding triaged, the team invited, and optionally another cluster. Each step checks itself from data Fortuna already has, nothing is stored, and the sidebar item shows `done/5` until the required steps are done, then leaves the sidebar.
- Investigations is now **Cases**. The list has Open, Mine, Closed and All views, and a case page shows its lifecycle (with the moves the server allows), the linked findings with their current status and risk level, affected assets, the timeline with note text, remediation tasks and owner. Closing a case asks for a reason and can resolve its open findings in the same step; if any of them fails to resolve the case stays open. **Add to case** replaces **Pin to case** on findings, pods and attack paths: it lets you pick an open case or start one instead of adding to a hidden active case, and the finding panel lists the cases that already link the finding. Title, owner and task edits save when you leave the field instead of on every keystroke. Removed the presence, shared cognition, handoff, incident command, live annotation, decision log and graph panels from the case page.
- Findings is a triage queue. `/risks` opens it instead of a Summary tab, whose charts and counts are on Home. Views for Needs triage, In review, Resolved, Dismissed and All replace the status pills, and the filters fit on one row. The detail panel and the finding page share one set of actions, so both show only what the finding's state and the user's role allow, and resolving from the finding page now asks for a reason like the panel does. J and K move through the queue, O opens the full page, and after an action the panel moves to the next finding.
- Alerts and the findings they open now show the same level. Finding notifications used the finding's rule severity (for example critical) and opened a search of the pod's findings, which shows each finding's risk level from the pod's score (often low or medium). A finding alert is now raised only when the risk level is high or critical, carries that level, opens the finding itself, and is updated or removed when the finding is rescored or closed; alerts stored earlier are corrected on the next read. The dashboard's critical count, the Executive brief and the findings export use the same risk level, the export adds `risk_level` and `risk_score` columns, the finding detail page uses the same score row as the list, and attack-path alerts use the 0-10 scale the Attack Paths page uses (they never fired before). CVE and malware alerts open the pod's SBOM tab. In the Risk Center the level badge is shown under every sort, the sort by the stored severity is labelled "Rule severity", and an unscored finding shows "no score" instead of its rule severity.
- One Home page at `/` for every role, replacing the separate Platform Integrity / Active Response / My Exposure home, the Operations Dashboard and Reports. It opens with the role's next step, then the risk overview, entry points and cluster health, and an Executive brief section with the report window, posture cards and findings exports. The brief loads only when expanded. Home reuses the dashboard's data instead of fetching the same stats a second time. `/dashboard` and `/reports` redirect to Home (`/reports` opens the brief), and admins now land on `/`. The brief's severity bars show each level's share of active findings instead of treating the count as a percentage.
- The Risk Center findings queue sorts on the server, so the chosen sort applies to every page instead of only the visible one, and changing it returns to page 1. The findings queue and the Exposure tab each have a "Reset filters" button.
- Platform Health (formerly Pipeline & Runtime Health) has Certificates and Notifications as tabs (`/monitoring/certificates`, `/monitoring/notifications`). Rules & Catalog (formerly Policy Rules) has Risk scoring rules, moved out of Settings, and the Capability catalog (`/rules/risk-scoring`, `/rules/catalog`). Operators and cluster admins, who hold `rules.write` but not `users.read`, can now edit risk scoring rules. Old links to `/certificates`, `/notifications` and `/capabilities` redirect.
- Audit (formerly Audit & Governance, `/governance`) is the one place for audit data: Security activity, the Platform audit log (moved from Platform Health, now filterable by resource and action), the Audit summary by resource and action (moved from Reports), the investigation timeline and access analytics, each on its own `?tab=`. Settings no longer queries security activity in the background for a hidden panel. Old `/monitoring?section=audit` links open the Audit log.
- Kubernetes Inventory has Clusters as a tab (`/resources/clusters`); `/clusters` redirects. Changing the header cluster clears a namespace filter that belonged to the previous cluster, on Inventory and Runtime Network.
- Runtime Network accepts `clusterId`, `namespace`, `podUid`, `q` and `tab` in the URL, and Pod detail links to it with the pod pre-selected.
- A shared "Reset filters" button clears search, filters and sort on Clusters, Inventory, Rules, Capability catalog, Platform Health error logs, Security activity and the Platform audit log.
- Certificates: a rotation history error shows as an error instead of "no records", and a failed rotation shows its error.
- Findings can be reopened (`PATCH /risk/insights/:id` with `status: active`, permission `findings.reopen`) from the finding detail page; the Risk Center status filter includes Dismissed.
- Reports: export failures show an error instead of failing silently, the Audit activity card shows audit data instead of investigation counts, and unavailable investigation stats show `n/a` instead of 0.
- The Risk Center ignores responses from a superseded request, so a slow earlier response can no longer overwrite the current filter's results.
- All downloads share one helper, and dashboard-generated CSV files neutralize formula cells.
- Removed unused API client methods, imports and dead state, and enabled `noUnusedLocals` so the typecheck rejects new dead code.
- Notification read state is per user: one person reading or clearing the bell no longer clears it for everyone. The bell links to the Notifications page, which has an Unread filter, counts and Load more.
- Filters, search and cluster changes refetch immediately on Capabilities, Resources, Dashboard and Attack Paths, reset paging and selection, and ignore responses from superseded requests. API errors on findings, audit logs, capability metadata and policy templates/instances show an error state instead of an empty list.
- Removed 21 unused dashboard files. Fixed the admin dashboard's link to a missing `/policies` page, hid Certificates links from users who cannot open it, added Platform Health to the operator nav, and a viewer deep link to a hidden Risk Center tab opens the default tab.

### Docs

- Removed internal working notes (audit plan, remediation status, dated integration and performance records, public-release checklist). The reference docs now describe current behavior only.
- The local CI guide moved to `docs/development/LOCAL_CI.md` and also covers the two-cluster integration test and the populated-migration rehearsal.
- Merged the component catalog into the architecture guide and the use cases into the user guide, and removed the per-folder index pages; `docs/README.md` is the single index.

### Scripts

- Removed the lab pipeline (`full-clean-database-rebuild-deploy.sh`), `deploy-fortuna-robust.sh` and the helpers only it used (Flannel install and VXLAN repair, control-plane labelling, DNS and prerequisite checks), the shell E2E suite, and the clean, monitor and one-off verify scripts. Install with Helm or the plain manifests; build from source with `build-and-load-containerd.sh` and deploy with `helm install --set image.tag=...`.
- `verify-k8s-e2e.sh` is now `scripts/verify/verify-scenarios.sh`, and `test-webhook-bootstrap.py` moved to `scripts/verify/`.

## v1.0.0 (refreshed 2026-10-05)

The `v1.0.0` tag was moved from the July 2026 build (`895d73d`) to the current `main`, and its images were rebuilt. Installs that pinned `sha-895d73df322c` keep the original build. The original release notes are kept in the GitHub release history.

### Install

- Helm chart in `deploy/helm/fortuna`. It generates the application secrets and a private CA with the Core, Agent and webhook certificates, keeps them on upgrade, and never stores the CA key.
- `deploy/*.yaml` is rendered from the chart and checked in CI. Workload images are `ghcr.io/shino-337/fortuna-community/fortuna-*:latest` instead of local build tags.
- One PostgreSQL manifest: `postgresql-with-age.yaml` was merged into `infrastructure/postgresql.yaml`.
- The dashboard nginx ConfigMap was removed. The dashboard image renders its own config and reads the cluster DNS server from the pod, instead of a hardcoded `10.96.0.10`. If you apply the new image with an old manifest, remove the `nginx-config` volume mount.
- The dashboard Service is now `ClusterIP`. Use `kubectl port-forward`, an Ingress, or the Helm value `dashboard.service.type=LoadBalancer`.
- The install scripts no longer edit tracked manifests. `kubectl set image` failures stop the run. Failed pre-deployment checks stop `deploy-fortuna-robust.sh` unless `SKIP_PREDEPLOY_CHECKS=true`. Loading CVE data during a deploy is opt-in (`AUTO_LOAD_CVE_ON_DEPLOY=true`).
- `install-falco-fortuna.sh` requires Helm to be installed already. It no longer downloads an installer and pipes it to bash.

### Security

- Agent (#58, #63):
  - No `pods/exec` and no `nodes/proxy`. CPU and memory come from `metrics.k8s.io`, and process and socket data from the read-only host `/proc`.
  - No host namespaces and no capabilities. The root filesystem is read-only.
  - The containerd socket is mounted only in a sidecar that has no token.
  - CI enforces all of this with an allowlist guard.
- Pod Detail encryption:
  - The install scripts and the chart generate a key.
  - An invalid key stops Core instead of being ignored.
  - `POD_DETAIL_ENCRYPTION_KEY_PREVIOUS` keeps older rows readable after a rotation.
  - Values that cannot be decrypted show a placeholder instead of ciphertext.
- The Agent redacts credential-looking command-line arguments and URL passwords. Process-diff events keep only the executable path.
- mTLS:
  - The CA is now valid for 10 years. Certificates are still valid for 365 days.
  - `rotate_mtls_secret.sh` now issues new certificates from the existing CA. Before, it did nothing once the secrets existed.
  - See "Certificate expiry" in the production guide.
- Seccomp `RuntimeDefault`, no ServiceAccount token, and no privilege escalation for the dashboard, PostgreSQL, NATS, the Core init containers and the risk CronJob. The CronJob's curl image is pinned.

### Since the July build

- Audit remediation #23 to #57: cluster-scoped identity and inventory, per-Agent HTTP and gRPC credentials, runtime evidence and source health, reviewed ServiceAccount revocation, and availability fixes.
- Documentation consolidated (#59 to #62). New configuration reference in `docs/reference/CONFIGURATION.md`, checked against the manifests and source in CI.

### CI

- `gofmt`, `shellcheck --severity=error`, `helm lint`, a drift check for rendered manifests, the Agent privilege guard on a Helm install, and a docs, manifest and source sync check.
