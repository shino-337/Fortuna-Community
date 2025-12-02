# mTLS Traffic Encryption Test Case

**Date**: 2025-12-01  
**Purpose**: Verify that traffic between Agent and Core is actually encrypted with mTLS

---

## Test Case Overview

### Objective
Verify that network traffic between Agent and Core is encrypted using mTLS, not plaintext.

### Test Methods
1. **Packet Capture Analysis** - Capture and analyze network packets
2. **TLS Connection Test** - Test TLS handshake with openssl
3. **Plaintext Detection** - Verify no plaintext data in traffic
4. **Certificate Verification** - Verify certificates are being used

---

## Test Case 1: Packet Capture and Analysis

### Prerequisites
- Debug pod with tcpdump available
- Network access to Core service
- Agent actively streaming data

### Test Steps

1. **Create debug pod**:
   ```bash
   kubectl run ksam-mtls-debug --image=nicolaka/netshoot:latest \
     --namespace=ksam --rm -it -- /bin/sh
   ```

2. **Start packet capture**:
   ```bash
   tcpdump -i any -w /tmp/capture.pcap host <core-ip> and port 9090
   ```

3. **Trigger Agent activity** (in another terminal):
   ```bash
   kubectl exec -n ksam <agent-pod> -- sh -c "echo 'trigger'"
   ```

4. **Stop capture** after 10-15 seconds

5. **Analyze capture**:
   ```bash
   # Check for TLS handshake
   tcpdump -r /tmp/capture.pcap -A | grep -i "handshake\|client hello\|server hello"
   
   # Check for plaintext (should be NONE)
   tcpdump -r /tmp/capture.pcap -A | grep -i "inventory\|pod\|serviceaccount"
   ```

### Expected Results

**✅ PASS Criteria**:
- TLS handshake packets detected (ClientHello, ServerHello)
- No plaintext inventory data visible
- Encrypted payload confirmed
- Packet sizes consistent with encrypted traffic

**❌ FAIL Criteria**:
- No TLS handshake detected
- Plaintext data visible in packets
- Unencrypted payload

### Evidence

**Successful Test**:
```
# TLS Handshake
ClientHello detected
ServerHello detected
ChangeCipherSpec detected

# Plaintext Check
No occurrences of "inventory", "pod", "serviceaccount" in packet payloads
```

---

## Test Case 2: TLS Connection Test with openssl

### Prerequisites
- openssl available in test environment
- CA certificate accessible
- Network access to Core service

### Test Steps

1. **Test TLS connection**:
   ```bash
   openssl s_client -connect ksam-core.ksam.svc.cluster.local:9090 \
     -CAfile /etc/ksam/ca-cert/ca.crt \
     -verify_return_error
   ```

2. **Verify output**:
   - Check for "Verify return code: 0"
   - Check TLS protocol version
   - Check cipher suite

### Expected Results

**✅ PASS Criteria**:
- Connection established
- Certificate verified (return code: 0)
- TLS protocol version shown (TLS 1.3 preferred)
- Cipher suite shown

**Example Output**:
```
CONNECTED(00000003)
depth=1 CN = KSAM CA
verify return code: 0 (ok)
---
Protocol  : TLSv1.3
Cipher    : TLS_AES_256_GCM_SHA384
```

---

## Test Case 3: Plaintext Detection Test

### Prerequisites
- Packet capture from Test Case 1
- Or ability to monitor network traffic

### Test Steps

1. **Capture traffic** during Agent streaming

2. **Search for plaintext keywords**:
   - "inventory"
   - "pod"
   - "serviceaccount"
   - "namespace"
   - Any Kubernetes resource names

3. **Analyze results**

### Expected Results

**✅ PASS Criteria**:
- Zero occurrences of plaintext keywords in packet payloads
- All data appears as encrypted binary

**❌ FAIL Criteria**:
- Plaintext keywords found in packets
- Readable JSON/YAML in packet data

---

## Test Case 4: Certificate Verification

### Prerequisites
- Access to certificates
- openssl available

### Test Steps

1. **Retrieve server certificate**:
   ```bash
   openssl s_client -connect ksam-core.ksam.svc.cluster.local:9090 \
     -showcerts
   ```

2. **Verify certificate details**:
   - Subject (CN should match service name)
   - Issuer (should be KSAM CA)
   - Validity dates
   - Certificate chain

### Expected Results

**✅ PASS Criteria**:
- Certificate subject matches service name
- Certificate signed by KSAM CA
- Certificate valid (not expired)
- Certificate chain complete

---

## Automated Test Script

### Script: `test_mtls_traffic_with_debug_pod.sh`

**Features**:
- Creates debug pod automatically
- Performs all test cases
- Cleans up after completion
- Provides detailed results

**Usage**:
```bash
./scripts/test_mtls_traffic_with_debug_pod.sh
```

**Output**:
- Test results for each test case
- Pass/fail status
- Detailed evidence
- Summary conclusion

---

## Test Results Interpretation

### All Tests Pass ✅
**Conclusion**: Traffic is encrypted with mTLS
- TLS handshake confirmed
- No plaintext detected
- Certificates valid
- Connection secure

### Some Tests Fail ⚠️
**Action Required**:
- Review failed test details
- Check TLS configuration
- Verify certificates
- Check network policies

### Critical Failures ❌
**Immediate Action**:
- Plaintext detected → **CRITICAL SECURITY ISSUE**
- No TLS handshake → TLS not working
- Certificate errors → Certificate misconfiguration

---

## Security Implications

### If Encrypted ✅
- **Data Confidentiality**: Protected
- **Data Integrity**: Protected
- **Authentication**: Mutual (mTLS)
- **Compliance**: Meets security requirements

### If NOT Encrypted ❌
- **Data Confidentiality**: **COMPROMISED**
- **Data Integrity**: **COMPROMISED**
- **Authentication**: **WEAK**
- **Compliance**: **VIOLATION**

**Immediate Actions**:
1. Stop service if possible
2. Investigate root cause
3. Fix TLS configuration
4. Re-test immediately
5. Review security policies

---

## Test Frequency

- **Initial Deployment**: Required
- **After Configuration Changes**: Required
- **Security Audits**: Required
- **Regular Verification**: Monthly
- **After Incidents**: Required

---

## Test Documentation

### Test Record Template

```
Test Date: YYYY-MM-DD HH:MM:SS
Tester: <name>
Environment: <cluster/environment>

Test Case 1: Packet Capture
  Status: PASS / FAIL
  Packets Captured: <count>
  TLS Handshake: YES / NO
  Plaintext Detected: NO / YES (<count>)
  Evidence: <details>

Test Case 2: TLS Connection
  Status: PASS / FAIL
  Connection: ESTABLISHED / FAILED
  Certificate: VERIFIED / FAILED
  TLS Version: <version>
  Evidence: <output>

Test Case 3: Plaintext Detection
  Status: PASS / FAIL
  Plaintext Found: NO / YES
  Evidence: <details>

Test Case 4: Certificate Verification
  Status: PASS / FAIL
  Certificate Valid: YES / NO
  Subject: <CN>
  Issuer: <CA>
  Evidence: <details>

Overall Result: PASS / FAIL
Conclusion: <summary>
```

---

## Troubleshooting

### Issue: Debug pod cannot be created

**Solution**:
- Check namespace permissions
- Verify image pull policy
- Check resource limits

### Issue: No packets captured

**Possible Causes**:
- No traffic during capture
- Network policy blocking
- Wrong IP/port

**Solution**:
- Increase capture duration
- Verify Agent is streaming
- Check network policies
- Verify service IP

### Issue: Plaintext detected

**CRITICAL**: Immediate investigation required

**Steps**:
1. Verify TLS configuration
2. Check gRPC server setup
3. Review connection logs
4. Test with openssl
5. Check for configuration errors

---

**Status**: Ready for execution


