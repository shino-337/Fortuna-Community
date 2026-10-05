# Backup and reset

These procedures change or delete Fortuna data. Run them only against the installation you mean to change: check `kubectl config current-context` first.

## Back up PostgreSQL

Back up regularly and before every upgrade; an image rollback does not reverse database migrations.

```bash
PG_POD=$(kubectl -n fortuna get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}')
umask 077
kubectl -n fortuna exec "$PG_POD" -- pg_dump -U postgres -d fortuna -Fc -Z 6 > fortuna-$(date -u +%Y%m%d%H%M).dump
```

Store the dump with restricted access; it contains users, findings and cluster inventory.

## Reset data

Reset cluster data while keeping schema (deletes all inventory, findings and runtime data; take a backup first):

```bash
PG_POD=$(kubectl -n fortuna get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}')
kubectl cp deploy/sql/clear_all_cluster_data.sql fortuna/$PG_POD:/tmp/
kubectl -n fortuna exec $PG_POD -- psql -U postgres -d fortuna -f /tmp/clear_all_cluster_data.sql
kubectl rollout restart daemonset/fortuna-agent -n fortuna
```

Full database reset helper (destructive: removes all application data, users,
catalogs, and migration history in the dedicated `fortuna` database):

```bash
PG_POD=$(kubectl -n fortuna get pods -l app=postgres -o jsonpath='{.items[0].metadata.name}')
set -e
umask 077
BACKUP_DIR=$(mktemp -d /tmp/fortuna-backup.XXXXXX)
kubectl -n fortuna exec "$PG_POD" -- pg_dump -U postgres -d fortuna -Fc -Z 6 > "$BACKUP_DIR/fortuna.dump"
kubectl -n fortuna cp "$BACKUP_DIR/fortuna.dump" "$PG_POD":/tmp/fortuna-check.dump
kubectl -n fortuna exec "$PG_POD" -- pg_restore -l /tmp/fortuna-check.dump >/dev/null
LOCAL_HASH=$(sha256sum "$BACKUP_DIR/fortuna.dump" | cut -d ' ' -f 1)
POD_HASH=$(kubectl -n fortuna exec "$PG_POD" -- sha256sum /tmp/fortuna-check.dump | cut -d ' ' -f 1)
test "$LOCAL_HASH" = "$POD_HASH"
kubectl -n fortuna exec "$PG_POD" -- rm /tmp/fortuna-check.dump
kubectl -n fortuna scale deployment/fortuna-core --replicas=0
kubectl -n fortuna wait --for=delete pod -l app.kubernetes.io/component=core --timeout=120s
kubectl cp deploy/sql/reset_database_full.sql fortuna/$PG_POD:/tmp/
kubectl -n fortuna exec "$PG_POD" -- psql -v ON_ERROR_STOP=1 -U postgres -d fortuna -f /tmp/reset_database_full.sql
kubectl -n fortuna scale deployment/fortuna-core --replicas=1
```

Keep the dump securely until the new rollout is verified. The reset keeps the
PostgreSQL PVC and Kubernetes Secrets; Core recreates the schema and bootstrap
admin on startup. A fresh vulnerability catalog must be loaded separately if
Core has no configured OSV source directory. Core quarantines Agent gRPC writes
until the [per-Agent mTLS registry and client certificate overlay](../../deploy/scoped-agent-credentials/MTLS.md)
is provisioned; the old shared Agent certificate is not a fallback. If the
original Fortuna CA private key is unavailable, use the dedicated Agent-client
CA procedure in that guide rather than rotating Core and webhook certificates.

Node-local Falco delivery state survives every reset above; see [preserve Falco delivery state](RUNTIME_SENSORS.md#preserve-falco-delivery-state) before replaying evidence into a fresh database.
