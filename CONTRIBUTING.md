# Contributing to Fortuna

Fortuna is a Kubernetes security and risk management platform. Public contributions should keep the repository runnable from the documented release packages and from source.

## Before Opening a Change

- Read [README.md](README.md) and the setup docs under [docs/01-getting-started](docs/01-getting-started).
- Keep examples generic. Do not commit real kubeconfigs, credentials, database dumps, private screenshots, generated reports, or local test output.
- Prefer release image tags such as `v1.0.0` in user-facing docs. Use local tags only in clearly marked development commands.

## Development

```bash
# Core
cd core
go test ./...

# Agent
cd ../agent
go test ./...

# Dashboard
cd ../dashboard
npm install
npm run build
```

Use the scripts under [scripts](scripts) for local cluster workflows and image distribution. See [deploy/README.md](deploy/README.md) for Kubernetes manifests.

## Pull Requests

- Keep changes focused and explain the operational impact.
- Include tests or a clear verification note for behavior changes.
- Update README/docs/deploy examples when configuration or installation behavior changes.
- Do not include generated binaries, packaged archives, local CVE datasets, or node-specific files.

## Security Issues

Do not open a public issue with exploit details or secrets. Follow [SECURITY.md](SECURITY.md).
