# Fortuna Agent → Core Proto Contract Specification

**Version**: 1.0  
**Date**: December 23, 2024  
**Status**: Draft for Phase 1  
**Purpose**: Define communication contract between Agent and Core

---

## 📋 Overview

This document defines the Protocol Buffer (protobuf) contract for **Agent → Core** communication in Fortuna.

**Design Principles**:
1. **Minimal Payload**: Send findings, not raw data
2. **Versioned**: Support backward compatibility
3. **Typed**: Strongly typed for validation
4. **Efficient**: Binary serialization (protobuf)

---

## 📁 File Structure

```
KSAM/api/proto/agent/
├─ sbom.proto           # SBOM finding messages
├─ cve.proto            # CVE finding messages (Phase 2)
├─ service.proto        # gRPC service definitions
└─ common.proto         # Shared types
```

---

## 📝 Proto Definitions

### 1. SBOM Finding Message

**File**: `KSAM/api/proto/agent/sbom.proto`

```protobuf
syntax = "proto3";

package fortuna.agent.sbom;

option go_package = "fortuna/api/proto/agent/sbom";

import "google/protobuf/timestamp.proto";

// SBOMFinding represents SBOM data extracted by Agent
message SBOMFinding {
  // Version for schema evolution
  int32 schema_version = 1;
  
  // Pod identity (required)
  string pod_uid = 2;
  string pod_name = 3;
  string namespace = 4;
  string container_name = 5;
  
  // Image info (required)
  string image_name = 6;
  string image_digest = 7;  // sha256:abc123...
  string image_tag = 8;
  
  // Operating System (optional)
  OSInfo os_info = 9;
  
  // Packages (required)
  repeated Package packages = 10;
  
  // Metadata (required)
  google.protobuf.Timestamp generated_at = 11;
  string agent_id = 12;
  string node_id = 13;
  
  // Additional context (optional)
  map<string, string> labels = 14;
  map<string, string> annotations = 15;
}

// OSInfo describes the container OS
message OSInfo {
  string name = 1;        // e.g., "debian", "alpine", "ubuntu"
  string version = 2;     // e.g., "11", "3.18", "22.04"
  string variant = 3;     // e.g., "slim", "musl"
  string architecture = 4; // e.g., "amd64", "arm64"
}

// Package represents a software package
message Package {
  string name = 1;         // e.g., "nginx"
  string version = 2;      // e.g., "1.21.6"
  PackageType type = 3;    // Enum of package types
  string architecture = 4; // e.g., "amd64", "all"
  repeated string licenses = 5;
  string source = 6;       // e.g., source package name
  
  // Optional metadata
  string description = 7;
  string homepage = 8;
  string maintainer = 9;
}

// PackageType enum
enum PackageType {
  PACKAGE_TYPE_UNKNOWN = 0;
  PACKAGE_TYPE_DEB = 1;      // Debian/Ubuntu
  PACKAGE_TYPE_RPM = 2;      // Red Hat/CentOS
  PACKAGE_TYPE_APK = 3;      // Alpine
  PACKAGE_TYPE_NPM = 4;      // Node.js
  PACKAGE_TYPE_PYPI = 5;     // Python
  PACKAGE_TYPE_GEM = 6;      // Ruby
  PACKAGE_TYPE_GO_MOD = 7;   // Go modules
  PACKAGE_TYPE_MAVEN = 8;    // Java Maven
  PACKAGE_TYPE_CARGO = 9;    // Rust
}
```

---

### 2. CVE Finding Message (Phase 2)

**File**: `KSAM/api/proto/agent/cve.proto`

```protobuf
syntax = "proto3";

package fortuna.agent.cve;

option go_package = "fortuna/api/proto/agent/cve";

import "google/protobuf/timestamp.proto";

// CVEFinding represents vulnerabilities found by Agent
message CVEFinding {
  // Version
  int32 schema_version = 1;
  
  // Reference to SBOM
  string sbom_id = 2;      // Sent in SBOMFinding response
  string pod_uid = 3;
  string container_name = 4;
  string image_digest = 5;
  
  // Vulnerabilities
  repeated Vulnerability vulnerabilities = 6;
  
  // Metadata
  google.protobuf.Timestamp scanned_at = 7;
  string agent_id = 8;
  string node_id = 9;
  
  // Scanner info
  ScannerInfo scanner = 10;
}

// Vulnerability represents a CVE
message Vulnerability {
  string cve_id = 1;           // e.g., "CVE-2024-1234"
  Severity severity = 2;
  float cvss_score = 3;        // e.g., 7.5
  string cvss_vector = 4;      // e.g., "CVSS:3.1/AV:N/AC:L/..."
  
  // Affected package
  string package_name = 5;
  string package_version = 6;
  
  // Fix info
  string fixed_version = 7;
  bool fix_available = 8;
  
  // Additional info
  string title = 9;
  string description = 10;
  repeated string references = 11;
  repeated string cwe_ids = 12;
  
  // Context
  google.protobuf.Timestamp published_at = 13;
  google.protobuf.Timestamp modified_at = 14;
}

// Severity enum
enum Severity {
  SEVERITY_UNKNOWN = 0;
  SEVERITY_LOW = 1;
  SEVERITY_MEDIUM = 2;
  SEVERITY_HIGH = 3;
  SEVERITY_CRITICAL = 4;
}

// ScannerInfo describes the CVE scanner
message ScannerInfo {
  string name = 1;      // e.g., "trivy", "grype"
  string version = 2;   // e.g., "0.48.0"
  string db_version = 3; // e.g., "2024-12-23"
}
```

---

### 3. Service Definitions

**File**: `KSAM/api/proto/agent/service.proto`

```protobuf
syntax = "proto3";

package fortuna.agent;

option go_package = "fortuna/api/proto/agent";

import "sbom.proto";
import "cve.proto";

// AgentService defines the gRPC service for Agent→Core communication
service AgentService {
  // Phase 1: SBOM
  rpc SendSBOMFinding(sbom.SBOMFinding) returns (SBOMResponse);
  rpc BatchSendSBOMFindings(BatchSBOMRequest) returns (BatchSBOMResponse);
  
  // Phase 2: CVE
  rpc SendCVEFinding(cve.CVEFinding) returns (CVEResponse);
  rpc BatchSendCVEFindings(BatchCVERequest) returns (BatchCVEResponse);
  
  // Health check
  rpc Ping(PingRequest) returns (PingResponse);
}

// Responses

message SBOMResponse {
  bool success = 1;
  string message = 2;
  string sbom_id = 3;  // Used to reference in CVEFinding
}

message BatchSBOMRequest {
  repeated sbom.SBOMFinding findings = 1;
}

message BatchSBOMResponse {
  int32 total = 1;
  int32 success_count = 2;
  int32 error_count = 3;
  repeated string sbom_ids = 4;
  repeated string errors = 5;
}

message CVEResponse {
  bool success = 1;
  string message = 2;
  int32 vulnerabilities_count = 3;
}

message BatchCVERequest {
  repeated cve.CVEFinding findings = 1;
}

message BatchCVEResponse {
  int32 total = 1;
  int32 success_count = 2;
  int32 error_count = 3;
  repeated string errors = 4;
}

// Health check
message PingRequest {
  string agent_id = 1;
  string node_id = 2;
}

message PingResponse {
  bool healthy = 1;
  string version = 2;
  int64 timestamp = 3;
}
```

---

### 4. Common Types

**File**: `KSAM/api/proto/agent/common.proto`

```protobuf
syntax = "proto3";

package fortuna.agent.common;

option go_package = "fortuna/api/proto/agent/common";

// NodeInfo describes the Kubernetes node
message NodeInfo {
  string node_id = 1;
  string node_name = 2;
  string node_ip = 3;
  string zone = 4;
  string region = 5;
  map<string, string> labels = 6;
}

// AgentInfo describes the agent
message AgentInfo {
  string agent_id = 1;
  string version = 2;
  string build_commit = 3;
  google.protobuf.Timestamp started_at = 4;
}
```

---

## 📊 Message Size Examples

### SBOMFinding (Typical)

```
Pod UID:         36 bytes
Pod Name:        20 bytes
Namespace:       10 bytes
Container Name:  15 bytes
Image Name:      50 bytes
Image Digest:    64 bytes
Image Tag:       10 bytes
OS Info:         30 bytes
Packages (50):   ~40KB (50 * 800 bytes)
Metadata:        100 bytes
Total:           ~41KB
```

**Network Savings**: 4GB image → 41KB finding = **99.9% reduction**

### CVEFinding (Typical)

```
SBOM ID:         10 bytes
Pod UID:         36 bytes
Vulnerabilities (10): ~5KB (10 * 500 bytes)
Metadata:        100 bytes
Scanner Info:    50 bytes
Total:           ~5KB
```

---

## 🔄 Message Flow

### Phase 1: SBOM Generation

```
1. Pod Created on Node-47
   ↓
2. Agent-47 detects (K8s watcher)
   ↓
3. Agent-47 generates SBOM (local)
   ↓
4. Agent-47 → Core: SendSBOMFinding(SBOMFinding)
   ↓
5. Core validates, stores, returns SBOMResponse{sbom_id: "123"}
   ↓
6. Agent-47 logs success
```

### Phase 2: CVE Scanning

```
1. Agent has sbom_id from Phase 1
   ↓
2. Agent runs CVE scan (local trivy/grype)
   ↓
3. Agent → Core: SendCVEFinding(CVEFinding{sbom_id: "123", vulns: [...]})
   ↓
4. Core correlates with SBOM, creates insights
   ↓
5. Core returns CVEResponse{success: true}
```

---

## 🔐 Security Considerations

### mTLS (Future)

```protobuf
// Add to service.proto for mTLS

service AgentService {
  // ... existing methods
  
  // Registration (with cert)
  rpc RegisterAgent(RegisterRequest) returns (RegisterResponse);
}

message RegisterRequest {
  AgentInfo agent_info = 1;
  bytes certificate = 2;
}

message RegisterResponse {
  bool authorized = 1;
  string token = 2;
  int64 expires_at = 3;
}
```

### Validation

**Core must validate**:
- Schema version matches
- Required fields present
- Image digest format (sha256:...)
- Package versions valid
- CVE IDs valid format

---

## 📈 Versioning Strategy

### Schema Evolution

```protobuf
message SBOMFinding {
  int32 schema_version = 1;  // Current: 1
  
  // v1 fields
  string pod_uid = 2;
  // ...
  
  // v2 fields (future)
  reserved 20 to 30;  // Reserve for v2
}
```

### Backward Compatibility

```go
// Core handler
func (h *Handler) SendSBOMFinding(ctx context.Context, req *pb.SBOMFinding) (*pb.SBOMResponse, error) {
    switch req.SchemaVersion {
    case 1:
        return h.handleSBOMv1(req)
    case 2:
        return h.handleSBOMv2(req)
    default:
        return nil, fmt.Errorf("unsupported schema version: %d", req.SchemaVersion)
    }
}
```

---

## 🧪 Testing

### Unit Tests (Proto)

```go
func TestSBOMFindingMarshal(t *testing.T) {
    finding := &pb.SBOMFinding{
        SchemaVersion: 1,
        PodUid:        "abc-123",
        // ...
    }
    
    // Marshal
    data, err := proto.Marshal(finding)
    require.NoError(t, err)
    
    // Unmarshal
    finding2 := &pb.SBOMFinding{}
    err = proto.Unmarshal(data, finding2)
    require.NoError(t, err)
    
    // Verify
    assert.Equal(t, finding.PodUid, finding2.PodUid)
}
```

### Integration Tests (gRPC)

```go
func TestSendSBOMFinding(t *testing.T) {
    // Start test gRPC server
    server := startTestServer(t)
    defer server.Stop()
    
    // Create client
    client := createTestClient(t, server.Addr())
    
    // Send finding
    finding := createTestSBOMFinding()
    resp, err := client.SendSBOMFinding(context.Background(), finding)
    
    require.NoError(t, err)
    assert.True(t, resp.Success)
    assert.NotEmpty(t, resp.SbomId)
}
```

---

## 📚 Code Generation

### Generate Go Code

```bash
# From repo root
cd KSAM/api/proto/agent

# Generate Go code
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       *.proto

# Output:
# sbom.pb.go
# cve.pb.go
# service.pb.go
# service_grpc.pb.go
# common.pb.go
```

### Makefile

```makefile
# KSAM/api/proto/Makefile

.PHONY: generate
generate:
	protoc --go_out=. --go_opt=paths=source_relative \
	       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
	       agent/*.proto
	
.PHONY: clean
clean:
	rm -f agent/*.pb.go
```

---

## ✅ Checklist

- [ ] Proto files created in `api/proto/agent/`
- [ ] Go code generated successfully
- [ ] Agent can import generated types
- [ ] Core can import generated types
- [ ] Unit tests for proto serialization pass
- [ ] Integration tests for gRPC pass
- [ ] Documentation updated

---

*Fortuna K8s Management Platform*  
*Proto Contract Specification v1.0*  
*Agent → Core Communication*

