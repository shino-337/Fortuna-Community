# Enable the optional admission webhook

The bundled webhook supports namespace `fortuna`. Install Core and its TLS secrets first. Use `scripts/deploy/enable-webhook.sh` instead of applying `webhook-config.yaml` directly. The helper validates the certificate against the installed CA and the Kubernetes service DNS name, then injects that CA into the webhook configuration. It does not require cert-manager.

```bash
./scripts/deploy/enable-webhook.sh
kubectl get endpoints fortuna-webhook -n fortuna
```

Endpoints must contain a Core pod address. An empty list means the webhook is not connected. Confirm Core readiness before proceeding.

## Existing certificates

Older generated certificates may lack `fortuna-webhook.fortuna.svc`. If verification fails, plan a rotation using `MTLS_REGEN=1 ./scripts/utils/rotate_mtls_secret.sh` after reviewing that script's impact on Core and Agents. Rotation changes the CA and can interrupt Agent connections; remote clusters need updated trust material too. Do not regenerate certificates merely to fix a Service selector.

After any CA rotation, rerun `enable-webhook.sh` to update `caBundle`. Certificate rotation and webhook CA synchronization are not automatic.

## Verify behavior in a disposable namespace

The configuration selects only namespaces labeled `fortuna.io/policy-enabled=true`. It initially uses `failurePolicy: Ignore`, while Core defaults to audit mode. Successful pod creation alone does not prove the webhook was called or enforcement worked.

In a disposable lab namespace:

1. Apply the opt-in namespace label.
2. Submit a benign pod using `kubectl apply --dry-run=server`.
3. Inspect API server/admission errors and Core logs to confirm the request reached the webhook.
4. Configure a known test policy and validate its expected allow/deny behavior before enabling enforcement.

Do not change `failurePolicy` to `Fail` before certificate, endpoint, availability, and policy checks pass. This guide is a validation procedure, not a record of a successful live-cluster test.

## Disable

```bash
kubectl delete validatingwebhookconfiguration fortuna-policy-webhook
```

This disables admission calls without deleting Core or the TLS secrets.
