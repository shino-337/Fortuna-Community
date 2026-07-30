# Fortuna Documentation

This documentation set is intentionally small and user-facing. It covers how to install Fortuna, operate it, understand the architecture, and use the dashboard.

The project name is **Fortuna**. The public repository is `shino-337/Fortuna-Community`, and published GHCR images use the lowercase repository namespace `ghcr.io/shino-337/fortuna-community`.

## Start Here

| Goal | Document |
|------|----------|
| Check environment requirements | [01-getting-started/ENVIRONMENT_REQUIREMENTS.md](01-getting-started/ENVIRONMENT_REQUIREMENTS.md) |
| Install from published images | [01-getting-started/INSTALLATION.md](01-getting-started/INSTALLATION.md) |
| Fast deploy path | [01-getting-started/QUICKSTART.md](01-getting-started/QUICKSTART.md) |
| Use the dashboard | [04-user-guide/README.md](04-user-guide/README.md) |
| Main investigation workflows | [04-user-guide/USE_CASES.md](04-user-guide/USE_CASES.md) |
| Operate a deployment | [05-operations/DEPLOYMENT.md](05-operations/DEPLOYMENT.md) |
| Production deployment | [05-operations/PRODUCTION_DEPLOYMENT.md](05-operations/PRODUCTION_DEPLOYMENT.md) |
| Local containerd build/deploy | [05-operations/DEPLOYMENT_CONTAINERD.md](05-operations/DEPLOYMENT_CONTAINERD.md) |
| Troubleshoot common failures | [05-operations/DEPLOYMENT.md](05-operations/DEPLOYMENT.md) |
| Architecture overview | [02-architecture/ARCHITECTURE.md](02-architecture/ARCHITECTURE.md) |
| Component catalog | [03-components/README.md](03-components/README.md) |
| Security and credentials | [06-reference/SECURITY.md](06-reference/SECURITY.md) |

## Diagrams And Screenshots

Architecture and component diagrams are maintained as Mermaid or text diagrams in the relevant documents. User-facing dashboard screenshots live under [assets/screenshots](assets/screenshots/) and are referenced from the user guide and component catalog.

## What Is Not Included

Generated test reports, local review notes, design backlog, internal audit reports, planning backlogs, credentials, kubeconfigs, and private environment captures are not part of the public documentation. Keep them local or attach them to private issues/PRs when needed.
