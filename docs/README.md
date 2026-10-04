# Fortuna documentation

## Use Fortuna

| Goal | Guide |
|---|---|
| Try it locally in one command | [Local demo](01-getting-started/DEMO.md) |
| Investigate your first RBAC attack path | [First investigation](01-getting-started/FIRST_FINDING.md) |
| Learn the dashboard workspaces | [User guide](04-user-guide/README.md) and [use cases](04-user-guide/USE_CASES.md) |

## Install and operate

| Goal | Guide |
|---|---|
| Check requirements | [Environment requirements](01-getting-started/ENVIRONMENT_REQUIREMENTS.md) |
| Install on a cluster from published images | [Install on a cluster](01-getting-started/QUICKSTART.md) |
| Harden a long-lived installation | [Production deployment](05-operations/PRODUCTION_DEPLOYMENT.md) |
| Build and deploy from source | [Local containerd build and deploy](05-operations/DEPLOYMENT_CONTAINERD.md) |
| Optional features | [Admission webhook](05-operations/WEBHOOK.md), [runtime source health](05-operations/RUNTIME_SOURCE_HEALTH.md), [ServiceAccount revocation](05-operations/SERVICEACCOUNT_MUTATIONS.md), [scoped AGE graph](05-operations/SCOPED_AGE.md) |
| Manifest reference | [deploy/README.md](../deploy/README.md) |
| Helper scripts | [scripts/README.md](../scripts/README.md) |

## Understand how it works

| Goal | Guide |
|---|---|
| Architecture and data flows | [Architecture](02-architecture/ARCHITECTURE.md) and [API conventions](02-architecture/API_STANDARD.md) |
| Components and detection model | [Components](03-components/README.md) |
| Security model and credentials | [Security](06-reference/SECURITY.md) |
| Exact behavior contracts | [Reference](06-reference/README.md) |

## Contribute

| Goal | Guide |
|---|---|
| Contribution workflow | [CONTRIBUTING.md](../CONTRIBUTING.md) |
| Run CI checks locally | [Local CI](05-operations/LOCAL_CI.md) |
| Audit plans, remediation status, release checklist | [Maintainer records](maintainers/README.md) |

Dashboard screenshots live in [assets/screenshots](assets/screenshots/). Do not add generated reports, credentials, kubeconfigs or private environment captures to the documentation.
