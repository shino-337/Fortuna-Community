package graph

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"gorm.io/gorm"
)

// AgeGraphEngine provides graph operations using Apache AGE
type AgeGraphEngine struct {
	db     *gorm.DB
	sqlDB  *sql.DB
	enabled bool
}

// NewAgeGraphEngine creates a new AGE graph engine
func NewAgeGraphEngine(db *gorm.DB) (*AgeGraphEngine, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	engine := &AgeGraphEngine{
		db:     db,
		sqlDB:  sqlDB,
		enabled: false,
	}

	// Check if AGE is available
	if err := engine.checkAGE(); err != nil {
		log.Printf("[AgeGraphEngine] AGE not available: %v. Graph features will be disabled.", err)
		return engine, nil // Return engine but disabled
	}

	engine.enabled = true
	log.Printf("[AgeGraphEngine] AGE graph engine initialized successfully")
	return engine, nil
}

// checkAGE checks if AGE extension is available
func (e *AgeGraphEngine) checkAGE() error {
	var exists bool
	err := e.sqlDB.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM pg_extension WHERE extname = 'age'
		)
	`).Scan(&exists)

	if err != nil {
		return fmt.Errorf("failed to check AGE extension: %w", err)
	}

	if !exists {
		return fmt.Errorf("AGE extension not installed")
	}

	return nil
}

// IsEnabled returns whether AGE is enabled
func (e *AgeGraphEngine) IsEnabled() bool {
	return e.enabled
}

// CreateVertex creates a vertex in the graph
func (e *AgeGraphEngine) CreateVertex(ctx context.Context, label string, properties map[string]interface{}) (string, error) {
	if !e.enabled {
		return "", fmt.Errorf("AGE not enabled")
	}

	// Build properties JSON
	propsJSON := buildPropertiesJSON(properties)

	query := fmt.Sprintf(`
		SELECT * FROM cypher('ksam_graph', $$
			CREATE (v:%s %s)
			RETURN id(v)
		$$) as (id agtype)
	`, label, propsJSON)

	var vertexID string
	err := e.sqlDB.QueryRowContext(ctx, query).Scan(&vertexID)
	if err != nil {
		return "", fmt.Errorf("failed to create vertex: %w", err)
	}

	return vertexID, nil
}

// CreateEdge creates an edge between two vertices
func (e *AgeGraphEngine) CreateEdge(ctx context.Context, fromID, toID, label string, properties map[string]interface{}) (string, error) {
	if !e.enabled {
		return "", fmt.Errorf("AGE not enabled")
	}

	propsJSON := buildPropertiesJSON(properties)

	query := fmt.Sprintf(`
		SELECT * FROM cypher('ksam_graph', $$
			MATCH (a), (b)
			WHERE id(a) = %s AND id(b) = %s
			CREATE (a)-[e:%s %s]->(b)
			RETURN id(e)
		$$) as (id agtype)
	`, fromID, toID, label, propsJSON)

	var edgeID string
	err := e.sqlDB.QueryRowContext(ctx, query).Scan(&edgeID)
	if err != nil {
		return "", fmt.Errorf("failed to create edge: %w", err)
	}

	return edgeID, nil
}

// GetAccessibleSecrets returns all secrets accessible by a ServiceAccount
func (e *AgeGraphEngine) GetAccessibleSecrets(ctx context.Context, saID string) ([]string, error) {
	if !e.enabled {
		return []string{}, nil
	}

	query := `
		SELECT * FROM cypher('ksam_graph', $$
			MATCH (sa:ServiceAccount {id: $saID})-[:USES]->(r:Role|ClusterRole)-[:GRANTS]->(s:Secret)
			RETURN s.name
		$$, $1) as (name agtype)
	`

	rows, err := e.sqlDB.QueryContext(ctx, query, saID)
	if err != nil {
		return nil, fmt.Errorf("failed to query accessible secrets: %w", err)
	}
	defer rows.Close()

	var secrets []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		secrets = append(secrets, name)
	}

	return secrets, nil
}

// ShortestPath finds shortest path between two resources
func (e *AgeGraphEngine) ShortestPath(ctx context.Context, fromID, toID string) ([]string, error) {
	if !e.enabled {
		return []string{}, nil
	}

	query := fmt.Sprintf(`
		SELECT * FROM cypher('ksam_graph', $$
			MATCH path = shortestPath((a)-[*]-(b))
			WHERE id(a) = %s AND id(b) = %s
			RETURN [node in nodes(path) | id(node)]
		$$) as (path agtype)
	`, fromID, toID)

	var pathStr string
	err := e.sqlDB.QueryRowContext(ctx, query).Scan(&pathStr)
	if err != nil {
		return nil, fmt.Errorf("failed to find shortest path: %w", err)
	}

	// Parse path (simplified - would need proper parsing in production)
	return []string{pathStr}, nil
}

// GetBlastRadius returns all resources reachable from a resource within maxDepth
func (e *AgeGraphEngine) GetBlastRadius(ctx context.Context, resourceID string, maxDepth int) ([]string, error) {
	if !e.enabled {
		return []string{}, nil
	}

	query := fmt.Sprintf(`
		SELECT * FROM cypher('ksam_graph', $$
			MATCH (start {id: $resourceID})-[*1..%d]-(connected)
			RETURN DISTINCT id(connected)
		$$, $1) as (id agtype)
	`, maxDepth)

	rows, err := e.sqlDB.QueryContext(ctx, query, resourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get blast radius: %w", err)
	}
	defer rows.Close()

	var resources []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		resources = append(resources, id)
	}

	return resources, nil
}

// GetNeighborhood returns neighbors of a resource within depth
func (e *AgeGraphEngine) GetNeighborhood(ctx context.Context, resourceID string, depth int) ([]string, error) {
	if !e.enabled {
		return []string{}, nil
	}

	return e.GetBlastRadius(ctx, resourceID, depth)
}

// buildPropertiesJSON builds JSON string for properties
func buildPropertiesJSON(properties map[string]interface{}) string {
	if len(properties) == 0 {
		return "{}"
	}

	// Use proper JSON marshaling
	jsonBytes, err := json.Marshal(properties)
	if err != nil {
		log.Printf("[AgeGraphEngine] Failed to marshal properties: %v", err)
		return "{}"
	}

	return string(jsonBytes)
}

// ExecuteCypher executes a Cypher query
func (e *AgeGraphEngine) ExecuteCypher(ctx context.Context, query string, params map[string]interface{}) ([]map[string]interface{}, error) {
	if !e.enabled {
		return []map[string]interface{}{}, fmt.Errorf("AGE not enabled")
	}

	// Build parameterized query
	// Note: AGE Cypher queries use $1, $2, etc. for parameters
	var args []interface{}
	argIndex := 1
	for k, v := range params {
		query = strings.ReplaceAll(query, "$"+k, fmt.Sprintf("$%d", argIndex))
		args = append(args, v)
		argIndex++
	}

	rows, err := e.sqlDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute Cypher query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		result := make(map[string]interface{})
		for i, col := range columns {
			result[col] = values[i]
		}
		results = append(results, result)
	}

	return results, nil
}

