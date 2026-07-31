# Public Release Checklist

Use this checklist before making the repository public or publishing a new release from a formerly private working tree.

## Required Before Public

- Run a full secret scan against the complete Git history, not only the current tree.
- Rewrite history for any real secrets, private keys, kubeconfigs, database dumps, node credentials, or local certificates that were ever committed.
- Rotate every credential that appeared in history, even if the file has since been deleted.
- Confirm generated/local artifacts are ignored and untracked: `.certs/`, `*.key`, `*.pem`, `.env*`, kubeconfigs, `cve-data/`, database dumps, root-level binaries, archives, logs, and `node_modules/`.
- Verify public docs reference only example values, generated-at-install secrets, or placeholder values.
- Verify release image references use `ghcr.io/shino-337/fortuna-community`.
- Ensure GitHub private vulnerability reporting is enabled for the public repository.
- Ensure branch protection requires CI, secret scanning, and review before publishing release tags.

## Current Repository Notes

This repository currently has no tracked `.certs/` files in the working tree, but the local working tree may still contain ignored `.certs/*.key` files. Delete local `.certs/` before running a current-tree release scan or creating any source archive.

Git history contains deleted `.certs/*.key` and certificate files in commits `6dbbf64f` and `6945742`. Before public launch, remove those objects from history with a history rewrite tool and force-push the sanitized branch, then rotate/regenerate the mTLS certificate authority and client/server certificates.

Gitleaks also reports historical findings in deleted docs/test artifacts and reset scripts, including archived JWT examples and kubeadm discovery token CA hashes. Treat these as rewrite candidates unless each value is proven synthetic and acceptable for public history.

The root-level `cmd` binary and `postgres_fortuna_backup.sql` are local untracked artifacts. They are ignored and should be deleted from any release staging directory before publishing.

## Suggested Local Checks

```bash
git status --short
git log --all --name-status -- .certs cmd postgres_fortuna_backup.sql
rg --hidden -g '!.git/*' -g '!node_modules/*' -g '!dashboard/node_modules/*' -i \
  'BEGIN (RSA |OPENSSH |EC |DSA |)PRIVATE KEY|aws_secret_access_key|github_pat|ghp_|client_secret|api[_-]?key\s*[:=]|password\s*[:=]|token\s*[:=]|secret\s*:='
```

If available, run Gitleaks across all history:

```bash
gitleaks detect --source . --redact --verbose
```

For history rewrite, prefer a fresh protected backup plus `git filter-repo` or BFG Repo-Cleaner. After rewriting, re-run the full scan before pushing to the public remote.
