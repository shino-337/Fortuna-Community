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
	Type  string                 `json:"type"` // serviceaccount, namespace, cluster
	Data  map[string]interface{} `json:"data,omitempty"`
}

// GraphEdge represents an edge in the graph
type GraphEdge struct {
	ID     string                 `json:"id"`
	Source string                 `json:"source"`
	Target string                 `json:"target"`
	Type   string                 `json:"type"` // belongs_to
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

// BuildGraph builds a simplified graph showing ServiceAccounts, Namespaces, and Clusters
func (s *GraphService) BuildGraph(clusterID, namespace string) (*GraphData, error) {
	graph := &GraphData{
		Nodes: []GraphNode{},
		Edges: []GraphEdge{},
	}

	// Build query
	saQuery := s.db.Model(&models.ServiceAccount{})

	if clusterID != "" {
		saQuery = saQuery.Where("cluster_id = ?", clusterID)
	}

	if namespace != "" {
		saQuery = saQuery.Where("namespace = ?", namespace)
	}

	// Get ServiceAccounts
	var sas []models.ServiceAccount
	if err := saQuery.Find(&sas).Error; err != nil {
		return nil, err
	}

	// Track unique clusters and namespaces
	clusterMap := make(map[string]bool)
	namespaceMap := make(map[string]map[string]bool) // cluster -> namespace -> exists
	edgeMap := make(map[string]bool)

	// Add ServiceAccount nodes and track clusters/namespaces
	for _, sa := range sas {
		saID := "sa:" + sa.ClusterID + ":" + sa.Namespace + ":" + sa.Name
		
		// Parse labels
		var labels map[string]string
		if sa.Labels != "" && sa.Labels != "null" {
			json.Unmarshal([]byte(sa.Labels), &labels)
		}

		graph.Nodes = append(graph.Nodes, GraphNode{
			ID:    saID,
			Label: sa.Name,
			Type:  "serviceaccount",
			Data: map[string]interface{}{
				"namespace": sa.Namespace,
				"cluster":   sa.ClusterID,
				"labels":    labels,
			},
		})

		// Track cluster
		clusterMap[sa.ClusterID] = true

		// Track namespace
		if namespaceMap[sa.ClusterID] == nil {
			namespaceMap[sa.ClusterID] = make(map[string]bool)
		}
		namespaceMap[sa.ClusterID][sa.Namespace] = true

		// Add edge from SA to namespace
		nsID := "ns:" + sa.ClusterID + ":" + sa.Namespace
		saNsEdgeID := saID + "->" + nsID
		if !edgeMap[saNsEdgeID] {
			graph.Edges = append(graph.Edges, GraphEdge{
				ID:     saNsEdgeID,
				Source: saID,
				Target: nsID,
				Type:   "belongs_to",
			})
			edgeMap[saNsEdgeID] = true
		}
	}

	// Add cluster nodes
	for clusterID := range clusterMap {
		graph.Nodes = append(graph.Nodes, GraphNode{
			ID:    "cluster:" + clusterID,
			Label: clusterID,
			Type:  "cluster",
		})
	}

	// Add namespace nodes and edges to clusters
	for clusterID, namespaces := range namespaceMap {
		for ns := range namespaces {
			nsID := "ns:" + clusterID + ":" + ns
			graph.Nodes = append(graph.Nodes, GraphNode{
				ID:    nsID,
				Label: ns,
				Type:  "namespace",
				Data: map[string]interface{}{
					"cluster": clusterID,
				},
			})

			// Add edge from namespace to cluster
			nsClusterID := "cluster:" + clusterID
			nsClusterEdgeID := nsID + "->" + nsClusterID
			if !edgeMap[nsClusterEdgeID] {
				graph.Edges = append(graph.Edges, GraphEdge{
					ID:     nsClusterEdgeID,
					Source: nsID,
					Target: nsClusterID,
					Type:   "belongs_to",
				})
				edgeMap[nsClusterEdgeID] = true
			}
		}
	}

	return graph, nil
}
