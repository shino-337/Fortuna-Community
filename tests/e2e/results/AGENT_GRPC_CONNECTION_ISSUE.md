# Agent gRPC Connection Issue Analysis

**Date**: 2025-12-28  
**Issue**: Agent cannot connect to Core gRPC server  
**Status**: ⏳ **INVESTIGATING**

---

## Problem

### Error Messages
```
[SBOMProcessor] Failed to process container core in pod fortuna/fortuna-core-867f95d8f6-n6jrw: 
failed to send SBOM to Core: SendSBOMFinding RPC failed: rpc error: code = Unavailable desc = 
connection error: desc = "transport: Error while dialing: dial tcp 10.110.71.133:9090: connect: connection refused"

⚠️  Heartbeat failed: Ping RPC failed: rpc error: code = Unavailable desc = 
connection error: desc = "transport: Error while dialing: dial tcp 10.110.71.133:9090: connect: connection refused"
```

### Root Cause Analysis

1. **Core Service**: ✅ Exists and has endpoints
   - ClusterIP: `10.110.71.133`
   - Endpoints: `10.244.4.145:9090,10.244.4.145:8080`
   - Ports: `8080/TCP,9090/TCP`

2. **Core Pod**: ✅ Running
   - IP: `10.244.4.145`
   - Status: `Running`
   - Ports: `8080, 9090` exposed

3. **gRPC Server**: ⚠️ **ISSUE**
   - Logs show: "Starting gRPC server on port 9090 WITH mTLS"
   - But no confirmation that server actually started listening
   - Server started in goroutine, errors may be silent

4. **Agent Config**: ✅ Correct
   - `CORE_GRPC_ENDPOINT: "fortuna-core.fortuna.svc.cluster.local:9090"`
   - TLS enabled

---

## Investigation Steps

1. ✅ Checked Core service and endpoints
2. ✅ Verified Core pod is running
3. ✅ Checked Agent configuration
4. ⏳ Verifying gRPC server actually listening
5. ⏳ Testing connection from Agent pod
6. ⏳ Checking for silent errors in gRPC goroutine

---

## Potential Issues

1. **gRPC Server Not Listening**: Server may have failed to start but error not logged
2. **TLS Handshake Failure**: mTLS configuration mismatch
3. **Network Policy**: Blocking connection
4. **Port Binding Issue**: Server not binding to correct interface

---

## Next Steps

1. Verify gRPC server is actually listening on port 9090
2. Test connection from Agent pod
3. Check for errors in gRPC server goroutine
4. Verify TLS certificates are valid
5. Test with TLS disabled temporarily

---

**Report Generated**: 2025-12-28  
**Status**: ⏳ **INVESTIGATING**

