# Security Policy

## Supported Versions

Security fixes target the latest published release and the `main` branch.

## Reporting a Vulnerability

Do not disclose vulnerability details in a public issue. Use GitHub private vulnerability reporting or GitHub Security Advisories for this repository when available.

If private reporting is not available, open a minimal public issue asking the maintainers for a private contact channel. Do not include exploit steps, secrets, tokens, kubeconfigs, hostnames, IP addresses, or screenshots that expose sensitive data.

## Handling secrets

- Contributors: see the repository rules in [CONTRIBUTING.md](CONTRIBUTING.md#repository-rules).
- Operators: see [runtime secrets and deployment notes](docs/06-reference/SECURITY.md).
- Maintainers making a formerly private repository public: run the [public release checklist](docs/maintainers/PUBLIC_RELEASE_CHECKLIST.md), including full-history secret scanning and credential rotation.
