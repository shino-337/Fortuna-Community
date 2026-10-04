# Reference

Detailed contracts for operators and contributors who need to know exactly how Fortuna behaves. For installation and everyday use, start with the [documentation index](../README.md).

## Security

- [Security](SECURITY.md): credentials, JWT, mTLS, image pulls, runtime sensors and repository hygiene.
- [Security invariants](SECURITY_INVARIANTS.md): properties every change must preserve.

## Identity and data ownership

- [Agent identity](AGENT_IDENTITY.md): per-Agent HTTP and gRPC credentials, and how Agents are bound to one cluster.
- [Inventory](INVENTORY.md): cluster scope for inventory reads, when a collection is complete enough to resolve findings, and how SBOMs, CVEs and malware matches are tied to workloads.

## Findings, graph and runtime

- [Findings and risk](FINDINGS_AND_RISK.md): scope rules for finding actions and runtime signal lists, and how findings are evaluated, resolved and re-scored.
- [Attack graph](GRAPH.md): what the attack graph does and does not claim, and the cluster-scoped internal AGE graph.
- [Runtime evidence](RUNTIME_EVIDENCE.md): how runtime producer coverage is reported, and the signed source-health protocol.
- [ServiceAccount mutations](../05-operations/SERVICEACCOUNT_MUTATIONS.md): previewed, reviewed revocation.

Planning, audit history and the dated [integration acceptance](../maintainers/INTEGRATION_ACCEPTANCE_20260929.md) record live in [Maintainer records](../maintainers/README.md).
