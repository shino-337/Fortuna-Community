# Security Reference

This reference covers runtime secret handling and repository hygiene for Fortuna deployments.

## Runtime Secrets

- Store `FORTUNA_JWT_SECRET`, `FORTUNA_ADMIN_PASSWORD`, `FORTUNA_INGEST_TOKEN`, registry credentials, and PostgreSQL credentials in Kubernetes Secrets or an external secret manager.
- Generate deployment secrets per environment. Do not reuse local development credentials in production.
- Rotate mTLS certificate authorities and client/server certificates after any suspected disclosure or history rewrite.
- Use short-lived tokens where possible and avoid putting bearer tokens in logs, issue reports, screenshots, or shell history.

## Repository Hygiene

- Do not commit real private keys, certificates, kubeconfigs, database dumps, `.env` files, node credentials, package vulnerability mirrors, generated reports, or local binaries.
- Keep examples under `deploy/samples/` and `*.example` files generic.
- Run the full-history secret scan before public releases and after large documentation imports.
- Treat deleted files as still public until Git history has been rewritten and the sanitized branch has been force-pushed.

## Deployment Notes

- Prefer immutable image tags such as release tags or `sha-<commit>` tags for production.
- Enable authentication for user-facing Core API routes.
- Use mTLS for agent-to-core gRPC traffic.
- Review dashboard and API logs before sharing bug reports.
