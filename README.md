# Fortuna

### Understand Kubernetes RBAC attack paths and the evidence behind them.

Fortuna helps Kubernetes security engineers, pentesters, and platform teams investigate **which workloads have dangerous permissions and how those permissions connect to an attack path**. It brings workload inventory, RBAC, vulnerability findings, and available runtime evidence into one investigation.

**Start with one question: could this pod's ServiceAccount give an attacker cluster-wide access?**

[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
[![CI](https://github.com/shino-337/Fortuna-Community/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/shino-337/Fortuna-Community/actions/workflows/ci.yml)

[Explore the screenshots](docs/04-user-guide/README.md#workspace-screenshots) · [Try the RBAC walkthrough](docs/01-getting-started/FIRST_FINDING.md) · [Install Fortuna](docs/01-getting-started/QUICKSTART.md) · [Website](https://fortunahub.dev)

![Fortuna Attack Paths workspace from a local deployment](docs/assets/screenshots/attack-analysis.png)

*Representative capture from a local deployment. Your findings and graph depend on the cluster, collected inventory, and enabled sensors. This image is not a before/after verification result.*

## Your first investigation

A pod does not need a hostPath mount to have dangerous access. A ServiceAccount bound to `cluster-admin` can already carry cluster-wide permissions.

The [RBAC walkthrough](docs/01-getting-started/FIRST_FINDING.md) uses the existing S2 lab fixture to help you:

1. Identify the pod and its ServiceAccount.
2. Follow the ClusterRoleBinding to the granted role.
3. Compare Kubernetes authorization with Fortuna's workload-specific evidence.
4. Remove the binding and check the result after inventory reconciliation.

An inferred permission path is **not proof that an attacker executed it**. Runtime confirmation requires corresponding telemetry and evidence.

## Choose how to explore

| Your goal | Start here |
|---|---|
| See the interface without installing | [Screenshot tour](docs/04-user-guide/README.md#workspace-screenshots) |
| Run Fortuna in an isolated lab | [Quickstart using released images](docs/01-getting-started/QUICKSTART.md) |
| Investigate one concrete RBAC condition | [First finding walkthrough](docs/01-getting-started/FIRST_FINDING.md) |
| Evaluate permissions and resource requirements | [Environment requirements](docs/01-getting-started/ENVIRONMENT_REQUIREMENTS.md) and [Agent manifest](deploy/fortuna-agent-daemonset.yaml) |
| Build or contribute | [Contributing](CONTRIBUTING.md) |

A hosted interactive demo and one-command demonstration environment are not available yet; see the [roadmap](ROADMAP.md).

## What you can investigate

| Area | Evidence to inspect |
|---|---|
| Workload and RBAC posture | Pods, ServiceAccounts, roles, bindings, dangerous permissions, and pod capabilities |
| Attack paths | Workload-specific relationships, granted permissions, missing edges, and path classification |
| Vulnerabilities | Workload SBOM, package matching, and vulnerability evidence |
| Runtime | Available Falco/agent telemetry and its relationship to static posture |
| Prioritization | Contributing factors behind a workload's risk score |

### Coverage and limits

- This README describes the evolving `main` branch. The latest published release is [v1.0.0](https://github.com/shino-337/Fortuna-Community/releases/tag/v1.0.0); use its versioned documentation and manifests for that release.
- Runtime coverage depends on sensor configuration. The current Agent manifest disables `EBPF_ENABLED` by default; a runtime badge or an empty view does not establish coverage.
- External ingress-to-workload modeling, network reachability correlation, and business-context weighting remain roadmap work.
- Scenario manifests define expected behavior. They are not evidence that your deployment passed those checks.
- Start in an isolated cluster. The current Agent uses host PID access, root, host mounts including the containerd socket, and additional Linux capabilities. Review the [manifest](deploy/fortuna-agent-daemonset.yaml) before installation.

## Installation

Use the [Quickstart](docs/01-getting-started/QUICKSTART.md) for published images. It covers Kubernetes/storage prerequisites, secrets, mTLS, rollout, and dashboard access. Review the [environment requirements](docs/01-getting-started/ENVIRONMENT_REQUIREMENTS.md) first.

For v1.0.0, start from a matching checkout:

```bash
git clone --branch v1.0.0 --depth 1 https://github.com/shino-337/Fortuna-Community.git
cd Fortuna-Community
```

Follow the Quickstart **inside that checkout**. Newer documentation and features on `main` may differ. Source builds and database reset workflows are developer/operations paths, not required to explore the screenshots.

## Architecture and documentation

Fortuna Agent collects Kubernetes/runtime inventory and sends it to Core over gRPC/mTLS. Core and workers correlate evidence using PostgreSQL and NATS; the dashboard exposes findings, inventory, risk factors, and attack paths.

| Need | Guide |
|---|---|
| Architecture and multi-cluster model | [Architecture](docs/02-architecture/ARCHITECTURE.md) |
| Components and detection model | [Components](docs/03-components/README.md) |
| Dashboard workflows | [User guide](docs/04-user-guide/README.md) |
| Live-cluster validation scenarios | [Scenarios](scenarios/README.md) |
| Production operations | [Production deployment](docs/05-operations/PRODUCTION_DEPLOYMENT.md) |
| Planned capabilities | [Roadmap](ROADMAP.md) |

## Feedback and contributions

Useful feedback includes your Fortuna version, Kubernetes/runtime version, the step that blocked you, and expected versus observed evidence. Remove credentials and sensitive cluster data before sharing.

See [CONTRIBUTING.md](CONTRIBUTING.md) for bug reports, documentation improvements, and detection scenarios. If Fortuna is useful to you, star the repository to help others discover it.

For vulnerabilities, follow [SECURITY.md](SECURITY.md). Fortuna is licensed under [Apache-2.0](LICENSE).
