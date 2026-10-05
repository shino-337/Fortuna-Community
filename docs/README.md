# Fortuna documentation

## Use Fortuna

| Goal | Guide |
|---|---|
| Try it locally in one command | [Local demo](getting-started/DEMO.md) |
| Investigate your first RBAC attack path | [First investigation](getting-started/FIRST_FINDING.md) |
| Learn the dashboard workspaces | [User guide](user-guide/README.md), including the investigation workflow |

## Install and operate

| Goal | Guide |
|---|---|
| Check requirements | [Environment requirements](getting-started/ENVIRONMENT_REQUIREMENTS.md) |
| Install on a cluster (Helm or plain manifests), add remote clusters | [Install on a cluster](getting-started/QUICKSTART.md) |
| Harden a long-lived installation | [Production deployment](operations/PRODUCTION_DEPLOYMENT.md) |
| Build and deploy from source | [Build and deploy from source](operations/DEPLOYMENT_CONTAINERD.md) |
| Optional features | [Runtime sensors (Falco, source health)](operations/RUNTIME_SENSORS.md), [admission webhook](operations/WEBHOOK.md), [ServiceAccount revocation](operations/SERVICEACCOUNT_MUTATIONS.md) |
| Back up or reset data | [Backup and reset](operations/BACKUP_AND_RESET.md) |
| Manifests, Helm chart and per-Agent credentials | [deploy/README.md](../deploy/README.md) |
| Helper scripts | [scripts/README.md](../scripts/README.md) |

## Understand how it works

| Goal | Guide |
|---|---|
| Components, data flows and dashboard states | [Architecture](architecture/ARCHITECTURE.md) |
| REST route groups and conventions | [API conventions](architecture/API_STANDARD.md) |

## Reference

Exact behavior, for operators and contributors who need to know precisely what Fortuna does.

| Topic | Contents |
|---|---|
| [Configuration](reference/CONFIGURATION.md) | Every environment variable Core and the Agent read, with defaults |
| [Security](reference/SECURITY.md) | Agent privileges and their blast radius, the CI guardrails on them, credential handling |
| [Security invariants](reference/SECURITY_INVARIANTS.md) | Properties every change must preserve, and the tests that enforce them |
| [Agent identity](reference/AGENT_IDENTITY.md) | Per-Agent HTTP and gRPC credentials and how Agents are bound to one cluster |
| [Inventory](reference/INVENTORY.md) | Cluster scope for inventory reads, collection receipts, SBOM ownership |
| [Findings and risk](reference/FINDINGS_AND_RISK.md) | Cluster scope on the risk APIs, finding actions, evaluation, auto-resolution, scoring, ServiceAccount permissions |
| [Attack graph](reference/GRAPH.md) | What the attack graph claims and the cluster-scoped AGE graph |
| [Runtime evidence](reference/RUNTIME_EVIDENCE.md) | Runtime producer coverage and the signed source-health protocol |

## Contribute

| Goal | Guide |
|---|---|
| Contribution workflow | [CONTRIBUTING.md](../CONTRIBUTING.md) |
| Run CI checks locally, the two-cluster test and a migration rehearsal | [Local CI](development/LOCAL_CI.md) |
| Release history | [CHANGELOG.md](../CHANGELOG.md) |

Dashboard screenshots live in [assets/screenshots](assets/screenshots/). Do not add generated reports, credentials, kubeconfigs or private environment captures to the documentation.
