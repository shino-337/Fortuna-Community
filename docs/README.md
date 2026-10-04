# Fortuna documentation

## Use Fortuna

| Goal | Guide |
|---|---|
| Try it locally in one command | [Local demo](getting-started/DEMO.md) |
| Investigate your first RBAC attack path | [First investigation](getting-started/FIRST_FINDING.md) |
| Learn the dashboard workspaces | [User guide](user-guide/README.md) and [use cases](user-guide/USE_CASES.md) |

## Install and operate

| Goal | Guide |
|---|---|
| Check requirements | [Environment requirements](getting-started/ENVIRONMENT_REQUIREMENTS.md) |
| Install on a cluster from published images | [Install on a cluster](getting-started/QUICKSTART.md) |
| Harden a long-lived installation | [Production deployment](operations/PRODUCTION_DEPLOYMENT.md) |
| Build and deploy from source | [Local containerd build and deploy](operations/DEPLOYMENT_CONTAINERD.md) |
| Optional features | [Runtime sensors (Falco, source health)](operations/RUNTIME_SENSORS.md), [admission webhook](operations/WEBHOOK.md), [ServiceAccount revocation](operations/SERVICEACCOUNT_MUTATIONS.md) |
| Back up or reset data | [Backup and reset](operations/BACKUP_AND_RESET.md) |
| Manifest reference | [deploy/README.md](../deploy/README.md) |
| Helper scripts | [scripts/README.md](../scripts/README.md) |

## Understand how it works

| Goal | Guide |
|---|---|
| Architecture and data flows | [Architecture](architecture/ARCHITECTURE.md) and [API conventions](architecture/API_STANDARD.md) |
| Components and detection model | [Components](components/README.md) |
| Agent privileges, security model and credentials | [Security](reference/SECURITY.md) |
| Exact behavior contracts | [Reference](reference/README.md) |

## Contribute

| Goal | Guide |
|---|---|
| Contribution workflow | [CONTRIBUTING.md](../CONTRIBUTING.md) |
| Run CI checks locally | [Local CI](maintainers/LOCAL_CI.md) |
| Audit plans, remediation status, release checklist | [Maintainer records](maintainers/README.md) |

Dashboard screenshots live in [assets/screenshots](assets/screenshots/). Do not add generated reports, credentials, kubeconfigs or private environment captures to the documentation.
