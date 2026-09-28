# P0 RBAC Scenarios — S6 and S7

These scenarios extend the S1–S5 validation seed with two distinct RBAC abuse patterns.

## S6 — ClusterRole excessive permission

### Positive assertion

The workload identity `fortuna-test/sa-cross-namespace-reader` is bound through a
`ClusterRoleBinding` to cluster-wide `get/list/watch` access on Secrets and ConfigMaps.
The expected evidence must connect:

1. workload (`cross-namespace-reader`);
2. ServiceAccount (`sa-cross-namespace-reader`);
3. ClusterRoleBinding (`crb-cross-namespace-reader`);
4. ClusterRole (`fortuna-cross-namespace-reader`);
5. cluster-scoped permission and blast radius.

A ClusterRoleBinding alone is not sufficient evidence for a critical finding.

### Negative boundary

A namespace-local `RoleBinding` with equivalent read permissions must not be treated as
cluster-wide exposure.

## S7 — Wildcard RBAC permission

### Positive assertion

The workload identity `fortuna-test/sa-wildcard` receives wildcard API groups,
resources, and verbs through a namespace-local `Role`.

The expected evidence must connect:

1. workload (`wildcard-rbac`);
2. ServiceAccount (`sa-wildcard`);
3. RoleBinding (`rb-wildcard`);
4. wildcard rule;
5. effective namespace scope.

Wildcard permissions alone do not establish a cluster-wide attack path.

### Negative boundary

A wildcard namespace-local Role must not be promoted to a cluster-wide escalation path
unless another evidence edge establishes that reachability or privilege impact.

## Verification note

The manifests are scenario fixtures. Runtime PASS/FAIL must be determined by Fortuna's
actual evidence collector and attack-path correlation, not by the presence of these YAML
objects alone.
