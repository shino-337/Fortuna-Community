package graph

import (
	"encoding/json"

	"gorm.io/gorm"

	"github.com/ksam/core/pkg/models"
)

// GraphNode represents a node in the graph
type GraphNode struct {
	ID    string                 `json:"id"`
	Label string                 `json:"label"`
	Type  string                 `json:"type"` // serviceaccount, role, namespace, cluster
	Data  map[string]interface{} `json:"data,omitempty"`
}

// GraphEdge represents an edge in the graph
type GraphEdge struct {
	ID     string                 `json:"id"`
	Source string                 `json:"source"`
	Target string                 `json:"target"`
	Type   string                 `json:"type"` // binding, usage, belongs_to
	Data   map[string]interface{} `json:"data,omitempty"`
}

// GraphData represents the complete graph
type GraphData struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GraphService handles graph generation
type GraphService struct {
	db *gorm.DB
}

// NewGraphService creates a new graph service
func NewGraphService(db *gorm.DB) *GraphService {
	return &GraphService{db: db}
}

// BuildGraph builds a graph from database data
func (s *GraphService) BuildGraph(clusterID, namespace string) (*GraphData, error) {
	graph := &GraphData{
		Nodes: []GraphNode{},
		Edges: []GraphEdge{},
	}

	// Build query
	saQuery := s.db.Model(&models.ServiceAccount{})
	rbQuery := s.db.Model(&models.RoleBinding{})
	crbQuery := s.db.Model(&models.ClusterRoleBinding{})

	if clusterID != "" {
		saQuery = saQuery.Where("cluster_id = ?", clusterID)
		rbQuery = rbQuery.Where("cluster_id = ?", clusterID)
		crbQuery = crbQuery.Where("cluster_id = ?", clusterID)
	}

	if namespace != "" {
		saQuery = saQuery.Where("namespace = ?", namespace)
		rbQuery = rbQuery.Where("namespace = ?", namespace)
	}

	// Get ServiceAccounts
	var sas []models.ServiceAccount
	if err := saQuery.Find(&sas).Error; err != nil {
		return nil, err
	}

	// Add ServiceAccount nodes
	saNodes := make(map[string]bool)
	nsNodes := make(map[string]bool)
	clusterNodes := make(map[string]bool)

	// Track edges to avoid duplicates
	edgeMap := make(map[string]bool)

	for _, sa := range sas {
		saID := "sa:" + sa.ClusterID + ":" + sa.Namespace + ":" + sa.Name
		
		// Add ServiceAccount node if not exists
		if !saNodes[saID] {
			graph.Nodes = append(graph.Nodes, GraphNode{
				ID:    saID,
				Label: sa.Name,
				Type:  "serviceaccount",
				Data: map[string]interface{}{
					"id":        sa.ID,
					"cluster":   sa.ClusterID,
					"namespace": sa.Namespace,
					"name":      sa.Name,
					"uid":       sa.UID,
				},
			})
			saNodes[saID] = true
		}

		// Add namespace node if not exists
		nsID := "ns:" + sa.ClusterID + ":" + sa.Namespace
		if !nsNodes[nsID] {
			graph.Nodes = append(graph.Nodes, GraphNode{
				ID:    nsID,
				Label: sa.Namespace,
				Type:  "namespace",
				Data: map[string]interface{}{
					"cluster": sa.ClusterID,
				},
			})
			nsNodes[nsID] = true
		}

		// Edge: ServiceAccount -> Namespace (create for every SA, not just when namespace is new)
		edgeID := "sa-ns:" + saID
		if !edgeMap[edgeID] {
			graph.Edges = append(graph.Edges, GraphEdge{
				ID:     edgeID,
				Source: saID,
				Target: nsID,
				Type:   "belongs_to",
			})
			edgeMap[edgeID] = true
		}

		// Add cluster node if not exists
		clusterID := "cluster:" + sa.ClusterID
		if !clusterNodes[clusterID] {
			graph.Nodes = append(graph.Nodes, GraphNode{
				ID:    clusterID,
				Label: sa.ClusterID,
				Type:  "cluster",
			})
			clusterNodes[clusterID] = true
		}

		// Edge: Namespace -> Cluster (create only once per namespace)
		nsClusterEdgeID := "ns-cluster:" + nsID
		if !edgeMap[nsClusterEdgeID] {
			graph.Edges = append(graph.Edges, GraphEdge{
				ID:     nsClusterEdgeID,
				Source: nsID,
				Target: clusterID,
				Type:   "belongs_to",
			})
			edgeMap[nsClusterEdgeID] = true
		}
	}

	// Get RoleBindings
	var rbs []models.RoleBinding
	if err := rbQuery.Find(&rbs).Error; err != nil {
		return nil, err
	}

	// Process RoleBindings to create edges
	for _, rb := range rbs {
		// Parse subjects
		var subjects []map[string]interface{}
		if err := json.Unmarshal([]byte(rb.Subjects), &subjects); err == nil {
			for _, subject := range subjects {
				if kind, ok := subject["kind"].(string); ok && kind == "ServiceAccount" {
					if name, ok := subject["name"].(string); ok {
						saID := "sa:" + rb.ClusterID + ":" + rb.Namespace + ":" + name

						// Parse roleRef
						var roleRef map[string]interface{}
						if err := json.Unmarshal([]byte(rb.RoleRef), &roleRef); err == nil {
							if roleName, ok := roleRef["name"].(string); ok {
								roleID := "role:" + rb.ClusterID + ":" + rb.Namespace + ":" + roleName

								// Add role node if not exists
								roleExists := false
								for _, node := range graph.Nodes {
									if node.ID == roleID {
										roleExists = true
										break
									}
								}
								if !roleExists {
									graph.Nodes = append(graph.Nodes, GraphNode{
										ID:    roleID,
										Label: roleName,
										Type:  "role",
										Data: map[string]interface{}{
											"cluster":   rb.ClusterID,
											"namespace": rb.Namespace,
										},
									})
								}

								// Edge: ServiceAccount -> Role (via RoleBinding)
								// Only create edge if ServiceAccount node exists
								if saNodes[saID] {
									edgeID := "sa-role:" + saID + "-" + roleID
									if !edgeMap[edgeID] {
										graph.Edges = append(graph.Edges, GraphEdge{
											ID:     edgeID,
											Source: saID,
											Target: roleID,
											Type:   "binding",
											Data: map[string]interface{}{
												"roleBinding": rb.Name,
											},
										})
										edgeMap[edgeID] = true
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Get ClusterRoleBindings
	var crbs []models.ClusterRoleBinding
	if err := crbQuery.Find(&crbs).Error; err != nil {
		return nil, err
	}

	// Process ClusterRoleBindings
	// When namespace filter is applied, only include ClusterRoleBindings that reference ServiceAccounts in that namespace
	clusterRoleNodesAdded := make(map[string]bool) // Track which ClusterRoles we've added
	for _, crb := range crbs {
		// Parse subjects
		var subjects []map[string]interface{}
		if err := json.Unmarshal([]byte(crb.Subjects), &subjects); err == nil {
			for _, subject := range subjects {
				if kind, ok := subject["kind"].(string); ok && kind == "ServiceAccount" {
					if name, ok := subject["name"].(string); ok {
						ns := ""
						if nsVal, ok := subject["namespace"].(string); ok {
							ns = nsVal
						}
						
						// If namespace filter is applied, skip ClusterRoleBindings that reference ServiceAccounts in other namespaces
						if namespace != "" && ns != namespace {
							continue
						}
						
						saID := "sa:" + crb.ClusterID + ":" + ns + ":" + name

						// Only process if ServiceAccount node exists (meaning it's in the filtered namespace)
						if !saNodes[saID] {
							continue
						}

						// Parse roleRef
						var roleRef map[string]interface{}
						if err := json.Unmarshal([]byte(crb.RoleRef), &roleRef); err == nil {
							if roleName, ok := roleRef["name"].(string); ok {
								roleID := "clusterrole:" + crb.ClusterID + ":" + roleName

								// Add cluster role node if not exists
								if !clusterRoleNodesAdded[roleID] {
									graph.Nodes = append(graph.Nodes, GraphNode{
										ID:    roleID,
										Label: roleName,
										Type:  "clusterrole",
										Data: map[string]interface{}{
											"cluster": crb.ClusterID,
										},
									})
									clusterRoleNodesAdded[roleID] = true
								}

								// Edge: ServiceAccount -> ClusterRole (via ClusterRoleBinding)
								edgeID := "sa-clusterrole:" + saID + "-" + roleID
								if !edgeMap[edgeID] {
									graph.Edges = append(graph.Edges, GraphEdge{
										ID:     edgeID,
										Source: saID,
										Target: roleID,
										Type:   "binding",
										Data: map[string]interface{}{
											"clusterRoleBinding": crb.Name,
										},
									})
									edgeMap[edgeID] = true
								}
							}
						}
					}
				}
			}
		}
	}

	// Validate edges: remove edges with non-existent source or target nodes
	nodeIDMap := make(map[string]bool)
	for _, node := range graph.Nodes {
		nodeIDMap[node.ID] = true
	}

	validEdges := []GraphEdge{}
	for _, edge := range graph.Edges {
		if nodeIDMap[edge.Source] && nodeIDMap[edge.Target] {
			validEdges = append(validEdges, edge)
		}
	}
	graph.Edges = validEdges

	return graph, nil
}

