# Security Policy

## Supported Versions

Security fixes target the latest published release and the `main` branch.

## Reporting a Vulnerability

Do not disclose vulnerability details in a public issue. Use GitHub private vulnerability reporting or GitHub Security Advisories for this repository when available.

If private reporting is not available, open a minimal public issue asking the maintainers for a private contact channel. Do not include exploit steps, secrets, tokens, kubeconfigs, hostnames, IP addresses, or screenshots that expose sensitive data.

## Secret Handling

- Never commit real Kubernetes secrets, kubeconfigs, certificates, private keys, database dumps, tokens, `.env` files, or local node credentials.
- Use `FORTUNA_JWT_SECRET`, `FORTUNA_ADMIN_PASSWORD`, `FORTUNA_INGEST_TOKEN`, and Kubernetes Secrets for runtime values.
- Use immutable release image tags or commit SHA tags for production installs.

Additional runtime guidance is in [docs/06-reference/SECURITY.md](docs/06-reference/SECURITY.md).
