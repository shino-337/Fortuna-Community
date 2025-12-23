# Admission Webhook - Deployment Instructions

**Date**: 2025-12-09  
**Status**: ✅ **READY FOR DEPLOYMENT**

---

## Quick Start

Run the automated deployment and test script:

```bash
./KSAM/scripts/deploy-and-test-webhook.sh
```

---

## Manual Deployment Steps

### Step 1: Apply Secret

```bash
kubectl apply -f /tmp/ksam-webhook-tls-secret.yaml
```

**Expected Output**:
```
secret/ksam-webhook-tls created
```

---

### Step 2: Apply Deployment

```bash
kubectl apply -f KSAM/deploy/core-deployment.yaml
```

**Expected Output**:
```
deployment.apps/ksam-core configured
```

---

### Step 3: Apply Webhook Service

```bash
kubectl apply -f KSAM/deploy/webhook-service.yaml
```

**Expected Output**:
```
service/ksam-webhook created
```

---

### Step 4: Apply Webhook Configuration

```bash
kubectl apply -f KSAM/deploy/webhook-config.yaml
```

**Expected Output**:
```
validatingwebhookconfiguration.admissionregistration.k8s.io/ksam-policy-webhook created
```

---

### Step 5: Wait for Pods

```bash
kubectl get pods -n ksam -l app=ksam-core
```

**Expected**: Pods should be in `Running` state

---

## Verification Steps

### 1. Check Webhook Server Logs

```bash
kubectl logs -n ksam -l app=ksam-core | grep -E "Webhook|HTTPS server|8443"
```

**Expected Logs**:
```
[Webhook] Setting up dedicated HTTPS server for admission webhook...
[Webhook] Starting webhook HTTPS server on :8443
[Webhook] Certificate: /etc/webhook/certs/tls.crt
[Webhook] Key: /etc/webhook/certs/tls.key
[Webhook] ✅ Webhook HTTPS server started on :8443
```

---

### 2. Check Service Endpoints

```bash
kubectl get endpoints -n ksam ksam-webhook
```

**Expected**: Should show pod IP with port 8443

---

### 3. Check Certificate Mount

```bash
POD_NAME=$(kubectl get pod -n ksam -l app=ksam-core -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n ksam $POD_NAME -- ls -la /etc/webhook/certs/
```

**Expected Files**:
- `tls.crt`
- `tls.key`

---

### 4. Test Health Endpoint (Port Forward)

```bash
# Terminal 1: Port forward
kubectl port-forward -n ksam deployment/ksam-core 8443:8443

# Terminal 2: Test endpoint
curl -k https://localhost:8443/admission/health
```

**Expected Response**:
```json
{"status":"ok"}
```

---

### 5. Test Policy Enforcement

```bash
# Label namespace
kubectl label namespace default ksam.io/policy-enabled=true --overwrite

# Create test pod
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-pod-webhook
  namespace: default
spec:
  containers:
    - name: nginx
      image: nginx:latest
      securityContext:
        runAsNonRoot: false
EOF

# Check webhook logs
kubectl logs -n ksam -l app=ksam-core | grep -E "AdmissionReview|admission/validate"
```

**Expected**: Webhook should receive AdmissionReview request and evaluate policies

---

## Troubleshooting

### Issue: Webhook server not starting

**Check**:
1. Certificates mounted:
   ```bash
   kubectl exec -n ksam <pod> -- ls -la /etc/webhook/certs/
   ```

2. TLS enabled:
   ```bash
   kubectl logs -n ksam -l app=ksam-core | grep "TLS_ENABLED"
   ```

3. Port conflict:
   ```bash
   kubectl logs -n ksam -l app=ksam-core | grep "8443"
   ```

---

### Issue: Webhook not receiving requests

**Check**:
1. Webhook config:
   ```bash
   kubectl get validatingwebhookconfiguration ksam-policy-webhook -o yaml
   ```

2. Service endpoints:
   ```bash
   kubectl get endpoints -n ksam ksam-webhook
   ```

3. Namespace label:
   ```bash
   kubectl get namespace default -o yaml | grep "ksam.io/policy-enabled"
   ```

---

### Issue: Certificate errors

**Check**:
1. Secret exists:
   ```bash
   kubectl get secret -n ksam ksam-webhook-tls
   ```

2. CA bundle in config:
   ```bash
   kubectl get validatingwebhookconfiguration ksam-policy-webhook -o jsonpath='{.webhooks[0].clientConfig.caBundle}' | wc -c
   ```
   Should be > 100 characters

---

## Files Created

1. ✅ `KSAM/core/cmd/main.go` - HTTPS webhook server implementation
2. ✅ `KSAM/deploy/core-deployment.yaml` - Updated deployment
3. ✅ `KSAM/deploy/webhook-service.yaml` - Webhook service
4. ✅ `KSAM/deploy/webhook-config.yaml` - Webhook configuration
5. ✅ `KSAM/scripts/generate-webhook-certs.sh` - Certificate generation
6. ✅ `KSAM/scripts/deploy-and-test-webhook.sh` - Deployment script
7. ✅ `/tmp/ksam-webhook-tls-secret.yaml` - TLS secret

---

## Summary

All code and manifests are ready for deployment. Run:

```bash
./KSAM/scripts/deploy-and-test-webhook.sh
```

Or follow the manual steps above.

---

**Status**: ✅ **READY FOR DEPLOYMENT**

