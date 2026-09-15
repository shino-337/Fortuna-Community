# Audit remediation status — September 2026

These changes follow merged PR #28. Review and merge the pull requests manually.
A passing test suite confirms the tested source behavior, not live deployment
coverage or a guarantee that the repository has no further defects.

| Finding | Change | Review |
| --- | --- | --- |
| Wrong RBAC role kind, namespace loss, duplicate/incorrect subject matches | Kind-aware resolution, scoped grants, standard SA subjects/groups, batch reads, explicit failures | [#29](https://github.com/shino-337/Fortuna-Community/pull/29) |
| Bulk operations falsely claimed Kubernetes disable/delete | Real UID-guarded deletion with prevalidation and per-item results; unsupported disable returns 501 without changes | [#30](https://github.com/shino-337/Fortuna-Community/pull/30) |
| Deletion could affect a replacement object and leave inconsistent audit | UID precondition, bounded requests, transactional inventory/audit and retry reconciliation | [#30](https://github.com/shino-337/Fortuna-Community/pull/30) |
| Deployment/ReplicaSet and capability list/aggregate scope gaps | Scope before reads/counts; aliases, pagination validation, UTC trend boundaries | [#31](https://github.com/shino-337/Fortuna-Community/pull/31) |
| Agent/node-name correlation could mix clusters | Explicit agent cluster identity, additive migration, atomic assignment and no node-name Ping fallback | [#32](https://github.com/shino-337/Fortuna-Community/pull/32) |
| Legacy graph traversal could expose other clusters | Scoped relational main graph; legacy unscoped operations restricted to unrestricted callers | [#33](https://github.com/shino-337/Fortuna-Community/pull/33) |
| Graph cache mutation/collision and wrong-cluster network enrichment | Copy path results; separate global keys; target-cluster service lookup and credential-aware cache | [#33](https://github.com/shino-337/Fortuna-Community/pull/33) |
| Risk evaluation failures could look like remediation | Resource-kind rule targeting, error propagation, configured evaluator, preserve disabled/missing detectors, transactional resolution audit | [#34](https://github.com/shino-337/Fortuna-Community/pull/34) |
| Identity UI confused errors with empty/missing data | Preserve API failures, display grant scope, retry and discard obsolete requests; browser regression fixtures | #29 and accompanying UI handoff PR |

## Merge and deployment order

PRs #29–34 branch from the main commit containing #28. Their source changes can be
reviewed separately. The UI handoff PR contains #29 as an ancestor: merge #29 first,
then review its remaining diff. Suggested order is #29, #30, #31, #32, #33, #34,
then UI handoff. The combined #29–34 source tree was checked for merge conflicts
locally; no GitHub pull request was merged automatically.

Before deployment, apply migration 150 and retain its additive column on rollback.
Allow successful HTTP inventory sync to assign old agents to clusters. Do not
infer missing agent ownership from a node name. Configure the YAML rule directory
and verify catalog initialization before running historical evaluation.

Behavior changes worth reviewing:

- Bulk delete now reaches Kubernetes and requires target kubeconfig plus both
  inventory.bulk and inventory.delete. HTTP 207 identifies partial failures.
- Disable and disable-inactive intentionally return 501. Inventory disappearance
  is not credential revocation. A real revocation workflow remains unimplemented.
- Legacy AGE traversal with limited cluster scope is intentionally denied. Scoped
  relational graph/attack-path views remain available; scoped AGE support remains
  unimplemented.
- Existing agents without cluster assignment temporarily disappear from cluster
  counts until HTTP sync. This change does not add per-cluster mTLS attestation.
- Risk reconciliation stops on invalid catalogs/evaluation errors. Missing or
  disabled detectors preserve findings; operators must inspect reported failures.

## Lab validation still required

1. Migrate a populated PostgreSQL database; verify legacy agents and rollback.
2. Use two clusters with repeated node, namespace and Service IP values; verify
   scoped accounts across Inventory, Agent, graph and network views.
3. Test Kubernetes deletion success, unavailable credentials, RBAC denial, UID
   replacement, and retry after database/audit persistence failure.
4. Validate live Service discovery and optional-name behavior during API failures.
5. Exercise risk reconciliation during concurrent ingestion/manual finding actions;
   verify audit records and incomplete-batch reporting.
6. Test actual login/navigation and the end-to-end demo flow in the lab. Browser
   fixtures mock API responses and do not replace this test.

The earlier broad backlog also includes runtime enrichment completeness, source
freshness, per-cluster agent credentials, scoped AGE implementation and load tests.
Those are not declared complete by this source audit. The documents below record
specific behavior and remaining boundaries, rather than treating a mitigation as
completion of the underlying feature.

## Contracts

- [RBAC grants](FINDING_RUNTIME_CONTRACT.md)
- [ServiceAccount mutations](SERVICEACCOUNT_MUTATIONS.md)
- [Workload and capability scope](INVENTORY_SCOPE.md)
- [Agent identity and migration](AGENT_CLUSTER_IDENTITY.md)
- [Graph/runtime boundaries](GRAPH_RUNTIME_BOUNDARIES.md)
- [Risk reconciliation](RISK_RECONCILIATION.md)
