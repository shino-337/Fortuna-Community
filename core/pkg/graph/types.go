package graph

// AttackPath represents an attack path from a source to a target
type AttackPath struct {
	Nodes      []PathNode `json:"nodes"`
	Edges      []PathEdge `json:"edges"`
	TotalRisk  float64    `json:"total_risk"`
	Difficulty float64    `json:"difficulty"` // 0-1, how hard to execute
	Impact     float64    `json:"impact"`      // 0-1, potential damage
	Length     int        `json:"length"`
	Description string    `json:"description"`
}

// PathNode represents a node in an attack path
type PathNode struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"` // Pod, ServiceAccount, Role, etc.
	Properties map[string]interface{} `json:"properties"`
}

// PathEdge represents an edge in an attack path
type PathEdge struct {
	Type       string `json:"type"` // USES_SERVICE_ACCOUNT, BINDS_TO, etc.
	Source     string `json:"source"`
	Target     string `json:"target"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// Permission represents permissions for a ServiceAccount (graph-based)
type Permission struct {
	RoleName  string      `json:"role_name"`
	Rules     []PolicyRule `json:"rules"`
	Namespace string      `json:"namespace"`
	RoleType  string      `json:"role_type"` // Role or ClusterRole
}

// PolicyRule represents a Kubernetes policy rule
type PolicyRule struct {
	Verbs         []string `json:"verbs"`
	Resources     []string `json:"resources"`
	APIGroups     []string `json:"api_groups"`
	ResourceNames []string `json:"resource_names,omitempty"`
}

// RiskyPod represents a pod with privilege escalation risk
type RiskyPod struct {
	UID                string `json:"uid"`
	Name               string `json:"name"`
	Namespace          string `json:"namespace"`
	ServiceAccountName string `json:"service_account_name"`
	RoleName           string `json:"role_name"`
	RiskScore          float64 `json:"risk_score"`
	RiskReason         string  `json:"risk_reason"`
}

