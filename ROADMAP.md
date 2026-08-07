# Fortuna Roadmap

Fortuna is being developed around one core goal:

> **Understand the Kubernetes attack path, not just the vulnerability.**

The roadmap is organized around security capabilities rather than internal implementation batches. Items may move as attack techniques, validation results, and community feedback change priorities.

## Kubernetes Exposure

- [x] Kubernetes workload and cluster inventory
- [x] ServiceAccount inventory and RBAC mapping
- [x] Role / ClusterRole / binding analysis
- [x] Pod capability and security-context analysis
- [x] Dangerous permission and wildcard detection
- [ ] Network exposure and service-to-workload reachability correlation
- [ ] External exposure and ingress-to-workload path modeling

## Attack Paths

- [x] RBAC privilege-escalation paths
- [x] ServiceAccount token abuse scenarios
- [x] HostPath and dangerous host-access scenarios
- [x] Lateral-movement scenarios
- [x] Broken / incomplete attack-chain validation
- [ ] Cross-namespace attack-path correlation
- [ ] Node compromise and cluster-wide blast-radius paths
- [ ] MITRE ATT&CK technique mapping for attack-path steps
- [ ] Community-contributed detection scenarios

## Runtime Security

- [x] Falco event ingestion
- [x] eBPF syscall telemetry
- [x] Process snapshot and diffing
- [x] Network connection tracking
- [x] Runtime evidence promotion and correlation
- [ ] Runtime-to-static attack-path enrichment
- [ ] Expanded runtime detection coverage

## SBOM & Vulnerability Intelligence

- [x] Workload-level SBOM collection
- [x] Package URL normalization
- [x] CVE / OSV correlation
- [x] Explainable vulnerability evidence
- [ ] Exploitability-aware prioritization improvements
- [ ] Broader vulnerability and package intelligence sources

## Unified Risk

- [x] Multi-factor workload risk scoring
- [x] Attack-path risk contribution
- [x] RBAC and capability risk contribution
- [x] Runtime evidence contribution
- [x] Contributing-factor explainability
- [ ] Asset / business-context weighting
- [ ] Risk trend and historical prioritization improvements
- [ ] Benchmark dataset for repeatable risk-model evaluation

## Platform & Operations

- [x] Multi-cluster agent architecture
- [x] Authenticated Agent-to-Core communication
- [x] Dashboard security workspaces
- [x] Reproducible deployment and verification scripts
- [ ] Simplified one-command demonstration environment
- [ ] Improved upgrade and migration experience
- [ ] Expanded operational telemetry and health reporting

## Community

- [ ] Security research scenario library
- [ ] Good-first-issue contributor path
- [ ] Detection-rule contribution guide
- [ ] Community Show & Tell examples
- [ ] Public benchmark / evaluation methodology
- [ ] Regular release notes and security research updates

## How to Contribute

Not every roadmap item needs to be implemented by the core maintainers. Contributions are especially welcome for attack-path scenarios, Kubernetes security research, runtime detections, vulnerability correlation, documentation, and tests.

For substantial changes, open an issue first so the proposed behavior and verification approach can be discussed before implementation.