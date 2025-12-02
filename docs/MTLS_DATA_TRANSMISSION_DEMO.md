# mTLS Data Transmission Demo

**Date**: 2025-12-01  
**Purpose**: Demonstrate Agent to Core data transmission with mTLS encryption and decryption

---

## Overview

This demo shows the complete flow of data transmission from Agent to Core, including:
1. **Encrypted Transmission**: Data encrypted with mTLS (TLS 1.3)
2. **Decryption at Core**: Data decrypted using mTLS certificates
3. **Data Visibility**: Show decrypted data in logs and database

---

## Demo Script: `demo_mtls_data_transmission.sh`

### Purpose
Demonstrates end-to-end data transmission with mTLS, showing:
- mTLS configuration and certificates
- Encrypted traffic information
- Decrypted data in Core logs
- Final data stored in database

### Usage

```bash
./scripts/demo_mtls_data_transmission.sh
```

---

## Demo Flow

### Step 1: Prepare for Demo
- Clears previous logs to show only new traffic
- Identifies Agent and Core pods

### Step 2: Verify mTLS Configuration
- Shows Agent and Core certificates (CN)
- Verifies TLS is enabled on both sides
- Displays certificate details

### Step 3: Check mTLS Connection Status
- Shows active connection from Agent to Core
- Displays connection details (IP addresses, ports)
- Confirms mTLS connection is established

### Step 4: Create Test Resource
- Creates a test Pod to trigger Agent streaming
- Agent detects the new resource via Kubernetes Watch API

### Step 5: Monitor Agent Streaming (Encrypted)
- Shows Agent logs indicating data streaming
- Data is encrypted with mTLS before transmission
- Logs show: "Successfully streamed X inventory items to core"

### Step 6: Show Encrypted Traffic Information
- Displays traffic characteristics:
  - Protocol: TLS 1.3
  - Cipher: AES-128-GCM-SHA256
  - Authentication: Mutual TLS (mTLS)
  - Source/Destination IPs
- **Note**: Actual packet contents are encrypted

### Step 7: Show Decrypted Data Received by Core
- Shows Core logs with decrypted data
- Logs include:
  - `[IngestAPI] Starting inventory stream`
  - `[IngestAPI] Published inventory item: kind=Pod, name=<name>/<namespace>`
  - `[AgentService] StreamInventory started`
  - `[AgentService] Processed X inventory items`

### Step 8: Show Data in Database (Final State)
- Queries PostgreSQL for received data
- Shows complete pod information:
  - Name, Namespace, UID
  - ServiceAccount
  - ClusterID
  - Timestamps
  - Container details

### Step 9: Show Normalized Data in NATS
- Checks NATS stream for normalized data
- Shows stream statistics
- Confirms data processing pipeline

### Step 10: Summary
- Complete flow summary
- Security confirmation
- Data flow visualization

---

## Expected Output

### mTLS Configuration
```
✅ Agent certificate: CN=ksam-agent
✅ Core certificate: CN=ksam-core.ksam.svc.cluster.local
Agent TLS: TLS_ENABLED=true
Core TLS: TLS_ENABLED=true
```

### Connection Status
```
✅ Active mTLS connection established
   tcp  ESTABLISHED 10.244.3.130:53372 -> 10.102.242.116:9090
```

### Encrypted Traffic
```
Traffic between Agent and Core is encrypted with mTLS:
   Protocol: TLS 1.3
   Cipher: AES-128-GCM-SHA256
   Authentication: Mutual TLS (mTLS)
   Source: Agent (10.244.3.130) -> Core (10.102.242.116:9090)
```

### Decrypted Data (Core Logs)
```
📥 Data received by Core (decrypted from mTLS stream):
   🔄 [IngestAPI] Starting inventory stream
   ✅ [IngestAPI] Published inventory item: kind=Pod, name=ksam-demo-1234567890/ksam
   📡 [AgentService] StreamInventory started
   📡 [AgentService] Processed 1 inventory items
```

### Database Data
```
✅ Data found in database (decrypted, normalized, and stored):
   Name: ksam-demo-1234567890
   Namespace: ksam
   UID: abc123-def456-...
   ServiceAccount: default
   ClusterID: minikube
   Created: 2025-12-01 08:00:00
   Updated: 2025-12-01 08:00:05
```

---

## Security Demonstration

### Encryption Verification

1. **Traffic Encryption**:
   - All data transmitted over TLS 1.3
   - Strong cipher suite (AES-128-GCM-SHA256)
   - Mutual authentication (mTLS)

2. **Certificate Validation**:
   - Agent certificate verified by Core
   - Core certificate verified by Agent
   - Certificate chain validated

3. **Data Protection**:
   - Data encrypted in transit
   - Cannot be read without private keys
   - Decrypted only at authorized endpoints

### Decryption Process

1. **Agent Side**:
   - Data collected from Kubernetes API
   - Encrypted with Agent's private key
   - Sent over mTLS connection

2. **Core Side**:
   - Receives encrypted data
   - Decrypts using Core's private key
   - Validates Agent certificate
   - Processes decrypted data

3. **Storage**:
   - Decrypted data normalized
   - Stored in PostgreSQL
   - Available for processing

---

## Data Flow Diagram

```
┌─────────┐
│  Agent  │
│         │
│ 1. Collect │
│    Data    │
└────┬────┘
     │
     │ 2. Encrypt with mTLS
     │    (TLS 1.3)
     ▼
┌─────────────────┐
│  Network        │
│  (Encrypted)    │
│  🔒 🔒 🔒      │
└────────┬────────┘
         │
         │ 3. Transmit encrypted
         ▼
┌─────────┐
│  Core   │
│         │
│ 4. Decrypt │
│    with mTLS│
│ 5. Process │
│ 6. Store  │
└──────────┘
```

---

## Key Points

1. **Encryption**: All data encrypted with mTLS (TLS 1.3)
2. **Decryption**: Data decrypted at Core using certificates
3. **Visibility**: Decrypted data visible in logs and database
4. **Security**: Traffic cannot be read without private keys
5. **Verification**: Certificates validated on both sides

---

## Troubleshooting

### Issue: No data in Core logs

**Solution**:
- Check Agent is streaming: `kubectl logs -n ksam -l app=ksam-agent | grep "Successfully streamed"`
- Verify mTLS connection: `kubectl exec -n ksam <agent-pod> -- ss -tnp | grep 9090`
- Check Core logs: `kubectl logs -n ksam -l app=ksam-core | grep IngestAPI`

### Issue: Data not in database

**Solution**:
- Wait a few seconds for processing
- Check worker logs: `kubectl logs -n ksam -l app=ksam-core | grep Worker`
- Verify NATS: `kubectl logs -n ksam -l app=nats`

### Issue: mTLS connection failed

**Solution**:
- Verify certificates: `kubectl exec -n ksam <pod> -- ls -la /etc/ksam/certs/`
- Check TLS enabled: `kubectl exec -n ksam <pod> -- env | grep TLS_ENABLED`
- Review connection logs: `kubectl logs -n ksam -l app=ksam-agent | grep -i tls`

---

## Related Scripts

- `test_mtls_connection.sh`: Test mTLS connection
- `test_mtls_traffic_with_debug_pod.sh`: Capture and analyze traffic
- `generate_test_traffic.sh`: Generate test data

---

**Status**: Ready for demonstration


