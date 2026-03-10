# Capability IDs Reference

**Last Updated**: 2026-01-27  
**Version**: Phase 1.5 (Standardized)

---

## Overview

This document provides a complete reference for all standardized Pod Capability IDs used in the Pod Capability Engine (PCE). All capability IDs follow the naming convention:

```
<DOMAIN>_<VECTOR>_<SCOPE>[_<QUALIFIER>]
```

Where:
- **DOMAIN**: Primary attack domain (ESC, NET, ID, API, CTRL, FS)
- **VECTOR**: Attack vector or mechanism
- **SCOPE**: Affected scope (POD, NODE, CLUSTER)
- **QUALIFIER**: Optional additional context

---

## Capability ID List

### Escape (ESC) Domain

#### ESC_PRIV_POD
- **Description**: Pod có container với privileged mode enabled
- **Severity**: CRITICAL
- **Confidence Base**: 0.7
- **Preconditions**: None
- **Attack Steps**: `NODE_FS_WRITE`, `NODE_KERNEL_ACCESS`
- **Runtime Promotion**: Yes
- **Old ID**: `ESC_PRIVILEGED`

#### ESC_HOSTPID_POD
- **Description**: Pod có hostPID enabled, có thể truy cập process namespace của host
- **Severity**: HIGH
- **Confidence Base**: 0.6
- **Preconditions**: None
- **Attack Steps**: `PROC_NAMESPACE_ACCESS`
- **Runtime Promotion**: Yes
- **Old ID**: `ESC_KERNEL` (split)

#### ESC_HOSTIPC_POD
- **Description**: Pod có hostIPC enabled, có thể truy cập IPC namespace của host
- **Severity**: HIGH
- **Confidence Base**: 0.6
- **Preconditions**: None
- **Attack Steps**: `IPC_NAMESPACE_ACCESS`
- **Runtime Promotion**: Yes
- **Old ID**: `ESC_KERNEL` (split)

#### ESC_HOSTPATH_NODE
- **Description**: Pod có hostPath mount, có thể truy cập filesystem của node
- **Severity**: CRITICAL
- **Confidence Base**: 0.8
- **Preconditions**: `ESC_PRIV_POD`
- **Attack Steps**: `NODE_FS_WRITE`, `NODE_CRED_DUMP`
- **Runtime Promotion**: Yes
- **Old ID**: `FS_HOST_RW`

#### ESC_RUNTIME_PROC_ROOT
- **Description**: Runtime signal xác nhận truy cập /proc/1/root (proc root pivot)
- **Severity**: CRITICAL
- **Confidence Base**: 0.9
- **Preconditions**: `ESC_HOSTPATH_NODE`, `ESC_PRIV_POD`
- **Attack Steps**: `NODE_FS_WRITE`, `NODE_PERSISTENCE`
- **Runtime Promotion**: No (already runtime-confirmed)
- **Old ID**: New capability

#### ESC_RUNTIME_PROBE
- **Description**: Static risk + runtime signal cho thấy khả năng escape
- **Severity**: HIGH
- **Confidence Base**: 0.5
- **Preconditions**: `ESC_HOSTPID_POD`, `ESC_HOSTIPC_POD`, `ESC_HOSTPATH_NODE`
- **Attack Steps**: `PROC_ROOT_PIVOT`
- **Runtime Promotion**: Yes
- **Old ID**: Unchanged

#### ESC_RUNTIME_ACTIVE
- **Description**: Active runtime escape được xác nhận (high confidence)
- **Severity**: CRITICAL
- **Confidence Base**: 0.95
- **Preconditions**: `ESC_RUNTIME_PROBE`
- **Attack Steps**: `NODE_FS_WRITE`, `NODE_PERSISTENCE`, `KUBELET_CRED_ACCESS`
- **Runtime Promotion**: No (already runtime-confirmed)
- **Old ID**: Unchanged

---

### Network (NET) Domain

#### NET_HOSTNETWORK
- **Description**: Pod sử dụng hostNetwork, có thể truy cập network của host
- **Severity**: MEDIUM
- **Confidence Base**: 0.4
- **Preconditions**: None
- **Attack Steps**: `NETWORK_SNIFFING`
- **Runtime Promotion**: Yes
- **Old ID**: `NET_HOST_NETWORK`

---

### Identity (ID) Domain

#### ID_TOKEN_POD
- **Description**: Pod có thể steal ServiceAccount token
- **Severity**: MEDIUM
- **Confidence Base**: 0.5
- **Preconditions**: None
- **Attack Steps**: `RBAC_ABUSE`
- **Runtime Promotion**: Yes
- **Old ID**: `ID_TOKEN_STEAL`

---

### API Domain

#### API_RBAC_WRITE_CLUSTER
- **Description**: Pod có RBAC write access đến Kubernetes API
- **Severity**: HIGH
- **Confidence Base**: 0.7
- **Preconditions**: `ID_TOKEN_POD`
- **Attack Steps**: `RBAC_ESCALATION`, `RESOURCE_MANIPULATION`
- **Runtime Promotion**: Yes
- **Old ID**: `API_K8S_WRITE`

---

### Control Plane (CTRL) Domain

#### CTRL_CONTROL_PLANE_POD
- **Description**: Pod chạy trong control plane namespace (kube-system)
- **Severity**: MEDIUM
- **Confidence Base**: 0.5
- **Preconditions**: None
- **Attack Steps**: `CONTROL_PLANE_ACCESS`
- **Runtime Promotion**: Yes
- **Old ID**: Unchanged

---

## Migration Guide

### Old → New ID Mapping

| Old ID | New ID | Notes |
|--------|--------|-------|
| `ESC_PRIVILEGED` | `ESC_PRIV_POD` | Renamed for consistency |
| `ESC_KERNEL` | `ESC_HOSTPID_POD`<br>`ESC_HOSTIPC_POD` | Split into two capabilities |
| `FS_HOST_RW` | `ESC_HOSTPATH_NODE` | Moved to ESC domain, renamed |
| `NET_HOST_NETWORK` | `NET_HOSTNETWORK` | Removed underscore |
| `ID_TOKEN_STEAL` | `ID_TOKEN_POD` | Renamed for consistency |
| `API_K8S_WRITE` | `API_RBAC_WRITE_CLUSTER` | More specific naming |

### Backward Compatibility

- Old capability IDs in database will continue to work
- New evaluations will use new IDs
- Migration script may be needed to update existing records

---

## Capability States

All capabilities can have the following states:

1. **detected**: Initial detection from static analysis
2. **confirmed**: Confirmed by runtime signals
3. **exploited**: Active exploitation detected
4. **chained**: Part of an attack chain

---

## Promotion Rules

Capabilities can be promoted between states based on runtime signals:

### Signal Types

- `PROC_ROOT_PIVOT`: Access to `/proc/1/root` or similar
- `FS_ESCAPE_ATTEMPT`: Filesystem escape attempts
- `NAMESPACE_ESCAPE`: Namespace escape attempts
- `CAPABILITY_MISUSE`: Misuse of Linux capabilities

### Example Promotions

- `PROC_ROOT_PIVOT` (1x) → `ESC_HOSTPATH_NODE` (confirmed)
- `PROC_ROOT_PIVOT` (3x) → `ESC_RUNTIME_ACTIVE` (exploited)
- `FS_ESCAPE_ATTEMPT` (1x) → `ESC_RUNTIME_ACTIVE` (exploited)

---

## Usage in Code

### Constants

All capability IDs are defined as constants in `core/pkg/capability/constants.go`:

```go
const (
    ESC_PRIV_POD = "ESC_PRIV_POD"
    ESC_HOSTPID_POD = "ESC_HOSTPID_POD"
    ESC_HOSTIPC_POD = "ESC_HOSTIPC_POD"
    ESC_HOSTPATH_NODE = "ESC_HOSTPATH_NODE"
    ESC_RUNTIME_PROBE = "ESC_RUNTIME_PROBE"
    ESC_RUNTIME_ACTIVE = "ESC_RUNTIME_ACTIVE"
    NET_HOSTNETWORK = "NET_HOSTNETWORK"
    ID_TOKEN_POD = "ID_TOKEN_POD"
    API_RBAC_WRITE_CLUSTER = "API_RBAC_WRITE_CLUSTER"
    CTRL_CONTROL_PLANE_POD = "CTRL_CONTROL_PLANE_POD"
)
```

### Evaluation

```go
caps, err := capability.EvaluatePod(ctx, db, pod)
// Returns capabilities with new standardized IDs
```

### State Management

```go
csc := capability.NewCapabilityStateController(db)
csc.InitializeCapability(ctx, podUID, namespace, capability.ESC_PRIV_POD, ...)
csc.PromoteCapability(ctx, podUID, capability.ESC_PRIV_POD, "PROC_ROOT_PIVOT", 0.9)
```

---

## References

- [PCE Enhancement Specification](./podCapabilityEngine_enhancement_spec.md)
- [PCE Improvement Specification](./PodCapabilityEngine_Improvement_Specification.md)
- [PCE Adjustment Document](./PodCapabilityEngine_adjustment.md)
