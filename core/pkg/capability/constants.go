package capability

// Capability ID constants following standardized format:
// <DOMAIN>_<VECTOR>_<SCOPE>[_<QUALIFIER>]
//
// DOMAIN: ESC, ID, NET, FS, API, OBS, CTRL
// VECTOR: PRIV, TOKEN, NET, PROC, API, RBAC, HOSTPID, HOSTIPC, HOSTPATH, RUNTIME
// SCOPE: POD, NODE, CLUSTER
// QUALIFIER (optional): RUNTIME, STATIC, WRITE, READ

const (
	// Escape capabilities
	ESC_PRIV_POD           = "ESC_PRIV_POD"           // Privileged container
	ESC_HOSTPID_POD       = "ESC_HOSTPID_POD"        // hostPID enabled
	ESC_HOSTIPC_POD       = "ESC_HOSTIPC_POD"        // hostIPC enabled
	ESC_HOSTPATH_NODE     = "ESC_HOSTPATH_NODE"      // hostPath mount
	ESC_RUNTIME_PROC_ROOT = "ESC_RUNTIME_PROC_ROOT"   // /proc/1/root pivot (runtime confirmed)
	ESC_RUNTIME_PROBE     = "ESC_RUNTIME_PROBE"      // Runtime probe (static risk + runtime signal)
	ESC_RUNTIME_ACTIVE    = "ESC_RUNTIME_ACTIVE"     // Active runtime escape (high confidence)

	// Identity capabilities
	ID_TOKEN_POD = "ID_TOKEN_POD" // ServiceAccount token steal

	// Network capabilities
	NET_HOSTNETWORK = "NET_HOSTNETWORK" // hostNetwork

	// API capabilities
	API_RBAC_WRITE_CLUSTER = "API_RBAC_WRITE_CLUSTER" // RBAC write access

	// Control plane capabilities
	CTRL_CONTROL_PLANE_POD = "CTRL_CONTROL_PLANE_POD" // Control plane namespace
)

// CapabilityState represents the state of a capability
type CapabilityState string

const (
	StateDetected  CapabilityState = "detected"  // Static config detected
	StateConfirmed CapabilityState = "confirmed" // Runtime signal confirmed
	StateExploited CapabilityState = "exploited" // Multiple signals corroborate
	StateChained   CapabilityState = "chained"   // Used in attack path
)

// LegacyCapabilityMapping maps old capability IDs to new standardized IDs
var LegacyCapabilityMapping = map[string]string{
	"ESC_PRIVILEGED":      ESC_PRIV_POD,
	"ESC_KERNEL":          ESC_HOSTPID_POD, // Will be split based on hostPID vs hostIPC
	"FS_HOST_RW":          ESC_HOSTPATH_NODE,
	"ESC_RUNTIME_PROBE":   ESC_RUNTIME_PROBE,
	"ESC_RUNTIME_ACTIVE":  ESC_RUNTIME_ACTIVE,
	"ID_TOKEN_STEAL":      ID_TOKEN_POD,
	"NET_HOST_NETWORK":    NET_HOSTNETWORK,
	"API_K8S_WRITE":       API_RBAC_WRITE_CLUSTER,
	"CTRL_CONTROL_PLANE_POD": CTRL_CONTROL_PLANE_POD,
}
