# Reference

Detailed contracts for operators and contributors who need to know exactly how Fortuna behaves. For installation and everyday use, start with the [documentation index](../README.md).

## Security

- [Security](SECURITY.md): credentials, JWT, mTLS, image pulls, runtime sensors and repository hygiene.
- [Security invariants](SECURITY_INVARIANTS.md): properties every change must preserve.

## Identity and data ownership

- [Agent credential foundation](AGENT_CREDENTIAL_FOUNDATION.md): per-agent credential boundaries and their enforcement.
- [Agent cluster identity](AGENT_CLUSTER_IDENTITY.md): how agents are bound to one cluster, and migration from the shared token.
- [Inventory scope](INVENTORY_SCOPE.md): workload and capability scope rules.
- [Inventory collection evidence](INVENTORY_COLLECTION_EVIDENCE.md): when inventory is complete enough to resolve findings.
- [SBOM content ownership](SBOM_CONTENT_OWNERSHIP.md): how SBOMs, CVEs and malware matches are tied to workloads.

## Findings, graph and runtime

- [Finding actions and runtime evidence](FINDING_RUNTIME_CONTRACT.md): scope rules for finding actions and runtime signal lists.
- [Graph and runtime boundaries](GRAPH_RUNTIME_BOUNDARIES.md): what the attack graph does and does not claim.
- [Runtime coverage evidence](RUNTIME_COVERAGE_EVIDENCE.md): how runtime producer coverage is reported.
- [Risk reconciliation](RISK_RECONCILIATION.md): how findings are resolved and re-scored.
- [ServiceAccount mutations](../05-operations/SERVICEACCOUNT_MUTATIONS.md): previewed, reviewed revocation.
- [Integration acceptance](INTEGRATION_ACCEPTANCE.md): the live two-cluster acceptance gate.

Planning and audit history lives in [Maintainer records](../maintainers/README.md).
