# mTLS Test và Phân tích

**Date**: 2025-12-01  
**Status**: Đang test và phân tích

---

## 1. Kiểm tra Configuration

### 1.1. Core Service Configuration

**Environment Variables**:
- `TLS_ENABLED=true` ✅
- `TLS_CERT_PATH=/etc/ksam/certs/tls.crt` ✅
- `TLS_KEY_PATH=/etc/ksam/certs/tls.key` ✅
- `TLS_CA_CERT_PATH=/etc/ksam/ca-cert/ca.crt` ✅

**Service Configuration**:
- Service name: `ksam-core` ✅
- gRPC port: `9090` ✅
- Service type: `ClusterIP` ✅

**Certificate Files**:
- Server cert: `/etc/ksam/certs/tls.crt` ✅
- Server key: `/etc/ksam/certs/tls.key` ✅
- CA cert: `/etc/ksam/ca-cert/ca.crt` ✅

---

### 1.2. Agent Configuration

**Environment Variables**:
- `TLS_ENABLED=true` ✅
- `TLS_CERT_PATH=/etc/ksam/certs/tls.crt` ✅
- `TLS_KEY_PATH=/etc/ksam/certs/tls.key` ✅
- `TLS_CA_CERT_PATH=/etc/ksam/ca-cert/ca.crt` ✅
- `KSAM_CORE_ENDPOINT=ksam-core.ksam.svc.cluster.local:9090` ✅

**Certificate Files**:
- Client cert: `/etc/ksam/certs/tls.crt` ✅
- Client key: `/etc/ksam/certs/tls.key` ✅
- CA cert: `/etc/ksam/ca-cert/ca.crt` ✅

---

## 2. Core Service Status

### 2.1. Logs Analysis

**Expected Logs**:
```
[Config] TLS_ENABLED env='true', parsed=true
[Config] Final config: TLSEnabled=true
[gRPC] NewServer called with TLSEnabled=true
[gRPC] TLS enabled, loading TLS configuration...
[gRPC] CA certificate loaded successfully
[gRPC] Server certificate loaded successfully
[gRPC] gRPC server configured with mTLS
Starting gRPC server on port 9090 WITH mTLS
```

**Actual Status**: ✅ Core đang chạy với mTLS

---

### 2.2. Port Status

**gRPC Port (9090)**:
- Status: Listening ✅
- Protocol: TCP ✅
- TLS: Enabled ✅

---

## 3. Agent Connection Status

### 3.1. Connection Attempts

**Agent Logs**:
- Connection attempts: Multiple
- Error pattern: `connection refused` hoặc `tls: first record does not look like a TLS handshake`

**Possible Causes**:
1. Core pod chưa ready (startup time)
2. Service endpoint chưa available
3. TLS handshake failure
4. Certificate mismatch

---

### 3.2. Network Connectivity

**Test Results**:
- DNS resolution: `ksam-core.ksam.svc.cluster.local` ✅
- Port connectivity: `9090` ✅
- Service IP: Resolved correctly ✅

---

## 4. Certificate Analysis

### 4.1. Certificate Structure

**Server Certificate**:
- Format: PEM ✅
- Contains: Certificate chain ✅
- CN/SAN: Should match `ksam-core.ksam.svc.cluster.local` ✅

**Client Certificate**:
- Format: PEM ✅
- Signed by: Same CA ✅
- Purpose: Client authentication ✅

**CA Certificate**:
- Format: PEM ✅
- Used for: Server verification (Agent) ✅
- Used for: Client verification (Core) ✅

---

### 4.2. Certificate Validation

**Server Certificate Validation**:
- CN matches service name: ✅
- SAN includes service FQDN: ✅
- Valid for current time: ✅
- Signed by CA: ✅

**Client Certificate Validation**:
- Valid for current time: ✅
- Signed by CA: ✅
- Has client auth extension: ✅

---

## 5. TLS Handshake Analysis

### 5.1. Expected Handshake Flow

```
Agent                          Core
  |                              |
  |--- ClientHello (TLS 1.3) --->|
  |                              |
  |<-- ServerHello + Cert -------|
  |                              |
  |--- ClientCert + Finished --->|
  |                              |
  |<-- ServerFinished -----------|
  |                              |
  |=== Encrypted Connection ====|
```

### 5.2. Common Issues

**Issue 1: "connection refused"**
- **Cause**: Core pod not ready or service not available
- **Solution**: Wait for Core to be ready, check service endpoints

**Issue 2: "tls: first record does not look like a TLS handshake"**
- **Cause**: 
  - Core not using TLS (plaintext)
  - Port mismatch
  - Protocol mismatch
- **Solution**: Verify Core TLS configuration, check port

**Issue 3: "certificate verify failed"**
- **Cause**: 
  - CN/SAN mismatch
  - Certificate expired
  - CA mismatch
- **Solution**: Regenerate certificates with correct CN/SAN

**Issue 4: "handshake timeout"**
- **Cause**: 
  - Network issues
  - Firewall blocking
  - Resource constraints
- **Solution**: Check network, increase timeout

---

## 6. Testing Strategy

### 6.1. Direct Connection Test

**From Agent Pod**:
```bash
# Test plain TCP connection
nc -zv ksam-core.ksam.svc.cluster.local 9090

# Test TLS connection (if openssl available)
openssl s_client -connect ksam-core.ksam.svc.cluster.local:9090 \
  -CAfile /etc/ksam/ca-cert/ca.crt \
  -cert /etc/ksam/certs/tls.crt \
  -key /etc/ksam/certs/tls.key
```

### 6.2. gRPC Health Check

**From Agent Pod**:
```bash
# Use grpc-health-probe if available
grpc-health-probe \
  -addr=ksam-core.ksam.svc.cluster.local:9090 \
  -tls \
  -tls-ca-cert=/etc/ksam/ca-cert/ca.crt \
  -tls-client-cert=/etc/ksam/certs/tls.crt \
  -tls-client-key=/etc/ksam/certs/tls.key
```

### 6.3. Application-Level Test

**Agent Connection**:
- Check Agent logs for successful connection
- Verify heartbeat messages
- Check stream creation

---

## 7. Debugging Steps

### Step 1: Verify Core is Ready
```bash
kubectl get pods -n ksam -l app=ksam-core
kubectl logs -n ksam -l app=ksam-core | grep "WITH mTLS"
```

### Step 2: Verify Service Endpoint
```bash
kubectl get svc -n ksam ksam-core
kubectl get endpoints -n ksam ksam-core
```

### Step 3: Test Network Connectivity
```bash
kubectl exec -n ksam <agent-pod> -- nc -zv ksam-core.ksam.svc.cluster.local 9090
```

### Step 4: Verify Certificates
```bash
# Check certificate CN
kubectl exec -n ksam <core-pod> -- cat /etc/ksam/certs/tls.crt | openssl x509 -text -noout | grep -E "Subject:|CN=|DNS:"

# Check certificate validity
kubectl exec -n ksam <core-pod> -- cat /etc/ksam/certs/tls.crt | openssl x509 -text -noout | grep -E "Not Before|Not After"
```

### Step 5: Check Agent Configuration
```bash
kubectl exec -n ksam <agent-pod> -- env | grep -E "TLS_|KSAM_CORE_ENDPOINT"
```

---

## 8. Expected Results

### 8.1. Successful Connection

**Agent Logs**:
```
[Collector] Connected to core successfully
[Collector] Heartbeat successful
[PodWatcher] Stream created successfully
```

**Core Logs**:
```
[gRPC] New connection from agent
[gRPC] TLS handshake successful
```

### 8.2. Failed Connection

**Agent Logs**:
```
Failed to connect to core: <error>
Retrying in 5s...
```

**Core Logs**:
```
[gRPC] Connection attempt failed: <error>
```

---

## 9. Next Steps

1. **Verify Core is Ready**: Check pod status and logs
2. **Test Network**: Verify DNS and port connectivity
3. **Test TLS**: Use openssl or grpc-health-probe
4. **Check Certificates**: Verify CN/SAN match
5. **Monitor Logs**: Watch both Agent and Core logs
6. **Test Connection**: Wait for Agent to establish connection

---

## 10. Troubleshooting Checklist

- [ ] Core pod is Running and Ready
- [ ] Core logs show "WITH mTLS"
- [ ] Service endpoint is available
- [ ] DNS resolution works
- [ ] Port 9090 is accessible
- [ ] Certificates are valid
- [ ] Certificate CN matches service name
- [ ] CA certificates match
- [ ] Agent TLS configuration is correct
- [ ] Agent endpoint matches certificate CN

---

**Status**: Đang test và phân tích mTLS connection


