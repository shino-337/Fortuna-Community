#!/usr/bin/env bash
# Ensure fortuna-secrets exists and optionally patch NVD API key from deploy/secrets.env or env NVD_API_KEY.
# Run from repo root. Use for deploy/rebuild so NVD key has a single place to define.
#
# Usage:
#   ./scripts/utils/ensure-fortuna-secrets.sh [namespace]
#   NVD_API_KEY=your-key ./scripts/utils/ensure-fortuna-secrets.sh fortuna
#
# To persist NVD key for future deploys: copy deploy/secrets.env.example to deploy/secrets.env,
# set NVD_API_KEY=your-key in deploy/secrets.env, then run this script before/after apply.

set -e
REPO_ROOT="${REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
NAMESPACE="${1:-fortuna}"
SECRET_NAME="fortuna-secrets"
CORE_SECRETS_YAML="${REPO_ROOT}/deploy/core-secrets.yaml"
SECRETS_ENV="${REPO_ROOT}/deploy/secrets.env"

cd "$REPO_ROOT"

if [ ! -f "$CORE_SECRETS_YAML" ]; then
  echo "ERROR: $CORE_SECRETS_YAML not found. Run from repo root." >&2
  exit 1
fi

# Ensure namespace
if ! kubectl get namespace "$NAMESPACE" &>/dev/null; then
  kubectl create namespace "$NAMESPACE"
  echo "Created namespace $NAMESPACE"
fi

# Create or update base secret (core-secrets.yaml has no nvd-api-key; we patch below if we have one)
kubectl apply -f "$CORE_SECRETS_YAML" -n "$NAMESPACE"
echo "Applied $SECRET_NAME in $NAMESPACE"

# Load NVD_API_KEY: from env, or from deploy/secrets.env
NVD_VAL="${NVD_API_KEY:-}"
if [ -z "$NVD_VAL" ] && [ -f "$SECRETS_ENV" ]; then
  set -a
  # shellcheck source=/dev/null
  source "$SECRETS_ENV"
  set +a
  NVD_VAL="${NVD_API_KEY:-}"
fi

if [ -n "$NVD_VAL" ]; then
  kubectl patch secret "$SECRET_NAME" -n "$NAMESPACE" --type=merge -p '{"stringData":{"nvd-api-key":"'"$NVD_VAL"'"}}'
  echo "Patched $SECRET_NAME with nvd-api-key (length ${#NVD_VAL} chars)"
else
  echo "NVD_API_KEY not set (no deploy/secrets.env or env). Core will run with NVD rate limit 5 req/30s."
fi
