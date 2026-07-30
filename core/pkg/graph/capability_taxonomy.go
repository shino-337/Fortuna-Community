package graph

// CanonicalCapability labels unify legacy graph capability strings over time.
// Legacy strings (CONTAINER_ACCESS, SA_TOKEN, …) remain valid in APIs;
// canonical keys are optional metadata for future strict state machines.
const (
	CapExecContainer      = "execution.container_shell"
	CapIdentitySAToken      = "identity.sa_token"
	CapAccessK8sAPI       = "access.k8s_api"
	CapAccessNodeShell    = "access.node_shell"
	CapInfraNetworkReach  = "infrastructure.network_reach"
	CapPrivilegeClusterAdmin = "privilege.cluster_admin"
)

// LegacyCapabilityAliases maps legacy caps used in technique_registry / chains to canonical IDs (Wave 0).
var LegacyCapabilityAliases = map[string]string{
	"CONTAINER_ACCESS":    CapExecContainer,
	"SA_TOKEN":            CapIdentitySAToken,
	"NETWORK_ACCESS":      CapInfraNetworkReach,
	"NODE_SHELL_ACCESS":   CapAccessNodeShell,
	"CLUSTER_ADMIN":       CapPrivilegeClusterAdmin,
	"ROLE":                "privilege.role_effective",
	"KUBELET_API_ACCESS":  "access.kubelet_api",
	"CONTAINER_RUNTIME_ACCESS": "access.container_runtime",
}

// CanonicalCapability returns the taxonomy label for a legacy cap, or the input if unknown.
func CanonicalCapability(legacy string) string {
	if c, ok := LegacyCapabilityAliases[legacy]; ok {
		return c
	}
	return legacy
}
