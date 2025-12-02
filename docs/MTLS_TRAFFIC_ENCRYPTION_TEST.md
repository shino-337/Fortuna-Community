# mTLS Traffic Encryption Test

**Date**: 2025-12-01  
**Purpose**: Verify that traffic between Agent and Core is actually encrypted with mTLS

---

## Test Overview

This test verifies that:
1. Traffic between Agent and Core is encrypted
2. TLS handshake is performed
3. No plaintext data is visible in network traffic
4. Certificates are being used correctly

---

## Test Methods

### Method 1: Packet Capture (tcpdump)

**Objective**: Capture and analyze network packets to verify encryption

**Steps**:
1. Start tcpdump on Core pod to capture port 9090
2. Trigger Agent to send data
3. Analyze captured packets:
   - Check for TLS handshake (ClientHello, ServerHello)
   - Verify no plaintext data visible
   - Confirm encrypted payload

**Expected Results**:
- ✅ TLS handshake packets detected
- ✅ No plaintext inventory data visible
- ✅ Encrypted payload confirmed

**Limitations**:
- Requires tcpdump/tshark in pod
- May require root privileges
- Capture file size considerations

---

### Method 2: TLS Connection Test (openssl)

**Objective**: Test TLS connection directly using openssl

**Steps**:
1. Use openssl s_client to connect to Core
2. Verify certificate chain
3. Check TLS protocol version
4. Verify connection establishment

**Expected Results**:
- ✅ TLS connection established
- ✅ Certificate verified (return code: 0)
- ✅ TLS protocol version shown

**Command**:
```bash
openssl s_client -connect ksam-core.ksam.svc.cluster.local:9090 \
  -CAfile /etc/ksam/ca-cert/ca.crt \
  -cert /etc/ksam/certs/tls.crt \
  -key /etc/ksam/certs/tls.key
```

---

### Method 3: Configuration Verification

**Objective**: Verify TLS configuration is correct

**Steps**:
1. Check environment variables
2. Verify certificate files exist
3. Check certificate validity
4. Verify connection status

**Expected Results**:
- ✅ TLS_ENABLED=true
- ✅ Certificates present and valid
- ✅ Connections established

---

### Method 4: Log Analysis

**Objective**: Verify TLS handshake in application logs

**Steps**:
1. Check Agent logs for connection establishment
2. Check Core logs for TLS configuration
3. Look for TLS-related messages

**Expected Results**:
- ✅ Agent: Connection established
- ✅ Core: "gRPC server configured with mTLS"
- ✅ Core: "Starting gRPC server on port 9090 WITH mTLS"

---

## Test Execution

### Automated Test

Run the automated test script:
```bash
./scripts/test_mtls_traffic_encryption.sh
```

### Manual Test

#### Step 1: Capture Traffic
```bash
# On Core pod
kubectl exec -n ksam <core-pod> -- tcpdump -i any -w /tmp/capture.pcap port 9090

# In another terminal, trigger Agent
kubectl exec -n ksam <agent-pod> -- sh -c "echo 'trigger'"

# Stop capture after 10 seconds
```

#### Step 2: Analyze Capture
```bash
# Check for TLS handshake
kubectl exec -n ksam <core-pod> -- tcpdump -r /tmp/capture.pcap -A | grep -i "handshake"

# Check for plaintext (should be none)
kubectl exec -n ksam <core-pod> -- tcpdump -r /tmp/capture.pcap -A | grep -i "inventory\|pod"
```

#### Step 3: Test with openssl
```bash
# From Agent pod
kubectl exec -n ksam <agent-pod> -- openssl s_client \
  -connect ksam-core.ksam.svc.cluster.local:9090 \
  -CAfile /etc/ksam/ca-cert/ca.crt \
  -cert /etc/ksam/certs/tls.crt \
  -key /etc/ksam/certs/tls.key
```

---

## Expected Results

### Successful Test

**Packet Capture**:
- TLS handshake detected: ✅
- Plaintext data: ❌ (none found)
- Encrypted payload: ✅

**openssl Test**:
- Connection: ✅ Established
- Certificate: ✅ Verified
- TLS Version: ✅ TLS 1.3

**Configuration**:
- TLS Enabled: ✅
- Certificates: ✅ Present
- Connection: ✅ Active

---

## Troubleshooting

### Issue: tcpdump not available

**Solution**: Use alternative methods:
- openssl s_client test
- Configuration verification
- Log analysis

### Issue: No packets captured

**Possible Causes**:
- No traffic during capture window
- Port mismatch
- Network policy blocking

**Solution**:
- Increase capture duration
- Verify Agent is streaming
- Check network policies

### Issue: Plaintext detected

**Critical**: This indicates TLS is not working!

**Investigation**:
1. Check TLS configuration
2. Verify certificates
3. Check gRPC server TLS setup
4. Review connection logs

---

## Security Implications

### If Traffic is Encrypted ✅
- Data confidentiality: Protected
- Data integrity: Protected
- Authentication: Mutual (mTLS)

### If Traffic is NOT Encrypted ❌
- **CRITICAL SECURITY ISSUE**
- Data can be intercepted
- Credentials exposed
- Compliance violation

---

## Test Frequency

- **Initial Setup**: Required
- **After Configuration Changes**: Required
- **Regular Verification**: Monthly
- **Security Audits**: Required

---

## Test Results Template

```
Test Date: YYYY-MM-DD
Test Status: PASSED / FAILED

Packet Capture:
  - TLS Handshake: ✅ / ❌
  - Plaintext Data: ✅ (none) / ❌ (found)
  - Encrypted Payload: ✅ / ❌

openssl Test:
  - Connection: ✅ / ❌
  - Certificate: ✅ / ❌
  - TLS Version: TLS 1.3

Configuration:
  - TLS Enabled: ✅ / ❌
  - Certificates: ✅ / ❌
  - Active Connections: ✅ / ❌

Conclusion: Traffic is encrypted / NOT encrypted
```

---

**Status**: Ready for execution


