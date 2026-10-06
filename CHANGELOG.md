# Changelog

## Unreleased

### Security

- The findings export (CSV and print-ready HTML) now applies the user's cluster scope; a cluster-scoped user could previously export findings from every cluster. CSV cells that start with a formula character are prefixed with `'`.
- Notifications, agent status and the counts in `/metrics/system` follow the user's cluster scope. Marking notifications read only affects notifications in scope.
- Revoking another user's session needs `sessions.revoke_all` instead of `users.read`, so a User admin can no longer sign an Admin out.
- A User admin can no longer change, disable or delete Cluster admin accounts, matching the rule that they cannot create them.
- Every list endpoint has a default and a maximum `limit` (31 routes were unbounded, including users, resources, investigations, graphs and policy rules), and reports `truncated` when rows are cut. `limit=-1` on rule matches no longer returns every row.

### Core

- `GET /risk/insights` sorts server-side: `sort` (score, severity, detected, updated, title, type, resource, namespace, status) with `order=asc|desc`, applied across all pages and part of the list cache key. Unknown values fall back to newest-detected-first, and every ordering ends with the finding id so pages are stable on ties.
- Statistics endpoints aggregate in SQL instead of loading every row: `/risk/trends` and `/pod-capabilities/trends` group per UTC day, `/investigations/stats` counts open cases in SQL and only reads cases whose remediation JSON mentions a due date, `/risk/scores` filters, counts, sorts and pages in the database (with the shared `pageSize` default of 50 and maximum of 500), and the access review counts users and reads only accounts that can raise a signal. Responses, cluster scope and permissions are unchanged.

### Dashboard

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
