#!/usr/bin/env bash
# Create/update fortuna-secrets from environment variables.
#
# Required:
#   FORTUNA_DATABASE_URL       postgres connection string used by Core
#
# Optional:
#   FORTUNA_ADMIN_PASSWORD     operator-provided bootstrap admin password
#   FORTUNA_DEFAULT_ADMIN_PASSWORD
#                               first-login default when FORTUNA_ADMIN_PASSWORD is omitted
#   FORTUNA_JWT_SECRET         generated with openssl when omitted
#   FORTUNA_INGEST_TOKEN       generated with openssl when omitted; required for Core/Agent ingest
#   POD_DETAIL_ENCRYPTION_KEY  encryption key for pod detail payloads
#   FORTUNA_POSTGRES_PASSWORD  creates postgres-credentials for bundled PostgreSQL manifests
#   FORTUNA_SECRET_NAME        defaults to fortuna-secrets
#
# Usage:
#   export FORTUNA_DATABASE_URL='postgres://<user>:<password>@postgres.fortuna.svc.cluster.local:5432/fortuna?sslmode=disable'
#   export FORTUNA_ADMIN_PASSWORD='<strong-password>'  # recommended for production
#   ./scripts/utils/ensure-fortuna-secrets.sh [namespace]

set -euo pipefail

NAMESPACE="${1:-${NAMESPACE:-fortuna}}"
SECRET_NAME="${FORTUNA_SECRET_NAME:-fortuna-secrets}"

require_env() {
  local name="$1"
  if [ -z "${!name:-}" ]; then
    echo "ERROR: $name is required. Export it before running this script." >&2
    exit 1
  fi
}

require_env FORTUNA_DATABASE_URL

if ! kubectl get namespace "$NAMESPACE" &>/dev/null; then
  kubectl create namespace "$NAMESPACE"
  echo "Created namespace $NAMESPACE"
fi

secret_value() {
  local key="$1"
  kubectl -n "$NAMESPACE" get secret "$SECRET_NAME" -o "jsonpath={.data.${key}}" 2>/dev/null | base64 -d 2>/dev/null || true
}

existing_admin_password="$(secret_value admin-password)"
existing_bootstrap_default_credential="$(secret_value bootstrap-default-credential)"
existing_jwt_secret="$(secret_value jwt-secret)"
existing_ingest_token="$(secret_value ingest-token)"
existing_pod_detail_encryption_key="$(secret_value pod-detail-encryption-key)"

BOOTSTRAP_DEFAULT_CREDENTIAL="${FORTUNA_BOOTSTRAP_DEFAULT_CREDENTIAL:-false}"
if [ -z "${FORTUNA_ADMIN_PASSWORD:-}" ]; then
  if [ -n "$existing_admin_password" ]; then
    FORTUNA_ADMIN_PASSWORD="$existing_admin_password"
    if [ -n "$existing_bootstrap_default_credential" ]; then
      BOOTSTRAP_DEFAULT_CREDENTIAL="$existing_bootstrap_default_credential"
    fi
    echo "Reusing existing admin-password from $SECRET_NAME. Set FORTUNA_ADMIN_PASSWORD to rotate it."
  else
    FORTUNA_ADMIN_PASSWORD="${FORTUNA_DEFAULT_ADMIN_PASSWORD:-Fortuna_ChangeMe_123!}"
    BOOTSTRAP_DEFAULT_CREDENTIAL="true"
    echo "WARNING: FORTUNA_ADMIN_PASSWORD is not set. Using default first-login bootstrap password; Fortuna will require a password change after login." >&2
  fi
fi

if [ -z "${FORTUNA_JWT_SECRET:-}" ]; then
  if [ -n "$existing_jwt_secret" ]; then
    FORTUNA_JWT_SECRET="$existing_jwt_secret"
    echo "Reusing existing jwt-secret from $SECRET_NAME. Set FORTUNA_JWT_SECRET to rotate it."
  else
    if ! command -v openssl >/dev/null 2>&1; then
      echo "ERROR: FORTUNA_JWT_SECRET is not set and openssl is unavailable." >&2
      exit 1
    fi
    FORTUNA_JWT_SECRET="$(openssl rand -base64 32)"
    echo "Generated FORTUNA_JWT_SECRET for this secret update."
  fi
fi

if [ -z "${FORTUNA_INGEST_TOKEN:-}" ]; then
  if [ -n "$existing_ingest_token" ]; then
    FORTUNA_INGEST_TOKEN="$existing_ingest_token"
    echo "Reusing existing ingest-token from $SECRET_NAME. Set FORTUNA_INGEST_TOKEN to rotate it."
  else
    if ! command -v openssl >/dev/null 2>&1; then
      echo "ERROR: FORTUNA_INGEST_TOKEN is not set and openssl is unavailable." >&2
      exit 1
    fi
    FORTUNA_INGEST_TOKEN="$(openssl rand -base64 32)"
    echo "Generated FORTUNA_INGEST_TOKEN for this secret update."
  fi
fi

secret_args=(
  "--from-literal=database-url=${FORTUNA_DATABASE_URL}"
  "--from-literal=jwt-secret=${FORTUNA_JWT_SECRET}"
  "--from-literal=ingest-token=${FORTUNA_INGEST_TOKEN}"
  "--from-literal=admin-password=${FORTUNA_ADMIN_PASSWORD}"
  "--from-literal=bootstrap-default-credential=${BOOTSTRAP_DEFAULT_CREDENTIAL}"
)

if [ -z "${POD_DETAIL_ENCRYPTION_KEY:-}" ] && [ -n "$existing_pod_detail_encryption_key" ]; then
  POD_DETAIL_ENCRYPTION_KEY="$existing_pod_detail_encryption_key"
  echo "Reusing existing pod-detail-encryption-key from $SECRET_NAME. Set POD_DETAIL_ENCRYPTION_KEY to rotate it."
fi

if [ -n "${POD_DETAIL_ENCRYPTION_KEY:-}" ]; then
  secret_args+=("--from-literal=pod-detail-encryption-key=${POD_DETAIL_ENCRYPTION_KEY}")
fi

kubectl -n "$NAMESPACE" create secret generic "$SECRET_NAME" \
  "${secret_args[@]}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "Applied $SECRET_NAME in $NAMESPACE"

if [ -n "${FORTUNA_POSTGRES_PASSWORD:-}" ]; then
  kubectl -n "$NAMESPACE" create secret generic postgres-credentials \
    "--from-literal=POSTGRES_PASSWORD=${FORTUNA_POSTGRES_PASSWORD}" \
    --dry-run=client -o yaml | kubectl apply -f -
  echo "Applied postgres-credentials in $NAMESPACE"
fi
