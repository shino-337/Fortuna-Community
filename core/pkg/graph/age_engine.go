package graph

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// AgeGraphEngine owns exactly one cluster graph. The caller must resolve an
// authorized cluster before constructing it; there is no unscoped query API.
type AgeGraphEngine struct {
	sqlDB                *sql.DB
	clusterID, graphName string
}

var ageLabel = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,62}$`)

func clusterGraphName(clusterID string) (string, error) {
	if clusterID == "" || len(clusterID) > 255 || strings.TrimSpace(clusterID) != clusterID {
		return "", fmt.Errorf("canonical cluster scope required")
	}
	sum := sha256.Sum256([]byte(clusterID))
	return "fortuna_" + hex.EncodeToString(sum[:24]), nil
}
func NewAgeGraphEngine(db *gorm.DB, clusterID string) (*AgeGraphEngine, error) {
	name, err := clusterGraphName(clusterID)
	if err != nil {
		return nil, err
	}
	pool, err := db.DB()
	if err != nil {
		return nil, err
	}
	var installed bool
	if err = pool.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'age')").Scan(&installed); err != nil {
		return nil, err
	}
	if !installed {
		return nil, fmt.Errorf("AGE extension unavailable")
	}
	return &AgeGraphEngine{sqlDB: pool, clusterID: clusterID, graphName: name}, nil
}
func (e *AgeGraphEngine) IsEnabled() bool { return e != nil && e.sqlDB != nil && e.clusterID != "" }
func (e *AgeGraphEngine) connection(ctx context.Context) (*sql.Conn, error) {
	if !e.IsEnabled() {
		return nil, fmt.Errorf("scoped AGE unavailable")
	}
	conn, err := e.sqlDB.Conn(ctx)
	if err != nil {
		return nil, err
	}
	for _, command := range []string{"LOAD 'age'", "SET search_path = ag_catalog, public"} {
		if _, err = conn.ExecContext(ctx, command); err != nil {
			conn.Close()
			return nil, err
		}
	}
	return conn, nil
}

// Initialize is an explicit operator/writer operation; read queries do not
// silently create graphs or convert missing extension/schema into empty data.
func (e *AgeGraphEngine) Initialize(ctx context.Context) error {
	conn, err := e.connection(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", e.graphName); err != nil {
		return err
	}
	var exists bool
	if err = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM ag_catalog.ag_graph WHERE name = $1)", e.graphName).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		if _, err = tx.ExecContext(ctx, "SELECT ag_catalog.create_graph($1)", e.graphName); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Queries and column contracts are private constants chosen by named methods.
// Only a validated label/depth and a hash-derived graph name enter SQL text;
// all data values travel in a prepared agtype parameter map on one connection.
func (e *AgeGraphEngine) query(ctx context.Context, cypher, columns string, params map[string]any) ([]string, error) {
	conn, err := e.connection(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	params["cluster"] = e.clusterID
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf("SELECT * FROM ag_catalog.cypher('%s', $fortuna$%s$fortuna$, $1) AS (%s)", e.graphName, cypher, columns)
	stmt, err := conn.PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	rows, err := stmt.QueryContext(ctx, string(raw))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := []string{}
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			return nil, err
		}
		results = append(results, value)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
func (e *AgeGraphEngine) scopedProperties(properties map[string]any) (map[string]any, error) {
	if owner, ok := properties["cluster_id"]; ok && owner != e.clusterID {
		return nil, fmt.Errorf("foreign vertex/edge cluster")
	}
	copy := make(map[string]any, len(properties)+1)
	for k, v := range properties {
		copy[k] = v
	}
	copy["cluster_id"] = e.clusterID
	if _, err := json.Marshal(copy); err != nil {
		return nil, err
	}
	return copy, nil
}
func agePropertiesLiteral(props map[string]any) (string, map[string]any, error) {
	keys := make([]string, 0, len(props))
	for key := range props {
		if !ageLabel.MatchString(key) {
			return "", nil, fmt.Errorf("invalid AGE property name")
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	params := map[string]any{}
	for i, key := range keys {
		p := fmt.Sprintf("value_%d", i)
		parts = append(parts, key+": $"+p)
		params[p] = props[key]
	}
	return "{" + strings.Join(parts, ", ") + "}", params, nil
}
func (e *AgeGraphEngine) CreateVertex(ctx context.Context, label string, properties map[string]any) (string, error) {
	if !ageLabel.MatchString(label) {
		return "", fmt.Errorf("invalid AGE label")
	}
	props, err := e.scopedProperties(properties)
	if err != nil {
		return "", err
	}
	literal, params, err := agePropertiesLiteral(props)
	if err != nil {
		return "", err
	}
	rows, err := e.query(ctx, fmt.Sprintf("CREATE (v:%s %s) RETURN id(v)", label, literal), "id agtype", params)
	if err != nil {
		return "", err
	}
	if len(rows) != 1 {
		return "", fmt.Errorf("vertex was not created")
	}
	return rows[0], nil
}
func ageVertexID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 || strconv.FormatInt(id, 10) != value {
		return 0, fmt.Errorf("invalid AGE vertex ID")
	}
	return id, nil
}
func (e *AgeGraphEngine) CreateEdge(ctx context.Context, fromID, toID, label string, properties map[string]any) (string, error) {
	if !ageLabel.MatchString(label) {
		return "", fmt.Errorf("invalid AGE label")
	}
	from, err := ageVertexID(fromID)
	if err != nil {
		return "", err
	}
	to, err := ageVertexID(toID)
	if err != nil {
		return "", err
	}
	props, err := e.scopedProperties(properties)
	if err != nil {
		return "", err
	}
	literal, params, err := agePropertiesLiteral(props)
	if err != nil {
		return "", err
	}
	params["from"] = from
	params["to"] = to
	rows, err := e.query(ctx, fmt.Sprintf("MATCH (a), (b) WHERE id(a) = $from AND id(b) = $to AND a.cluster_id = $cluster AND b.cluster_id = $cluster CREATE (a)-[e:%s %s]->(b) RETURN id(e)", label, literal), "id agtype", params)
	if err != nil {
		return "", err
	}
	if len(rows) != 1 {
		return "", fmt.Errorf("edge endpoints missing or foreign")
	}
	return rows[0], nil
}

func boundedDepth(depth int) (int, error) {
	if depth < 1 || depth > 8 {
		return 0, fmt.Errorf("AGE traversal depth must be 1..8")
	}
	return depth, nil
}
func (e *AgeGraphEngine) paths(ctx context.Context, depth int, selector string, params map[string]any) ([]AttackPath, error) {
	if _, err := boundedDepth(depth); err != nil {
		return nil, err
	}
	raw, err := e.query(ctx, fmt.Sprintf("MATCH p = (a)-[*1..%d {cluster_id: $cluster}]->(b) WHERE a.cluster_id = $cluster AND b.cluster_id = $cluster AND %s RETURN p LIMIT 1000", depth, selector), "path agtype", params)
	if err != nil {
		return nil, err
	}
	paths := make([]AttackPath, 0, len(raw))
	for _, value := range raw {
		p, err := decodeAGEPath(value, e.clusterID)
		if err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, nil
}
func (e *AgeGraphEngine) GetBlastRadius(ctx context.Context, resourceID string, maxDepth int) ([]string, error) {
	id, err := ageVertexID(resourceID)
	if err != nil {
		return nil, err
	}
	paths, err := e.paths(ctx, maxDepth, "id(a) = $id", map[string]any{"id": id})
	if err != nil {
		return nil, err
	}
	ids := []string{}
	seen := map[string]bool{}
	for _, p := range paths {
		id := p.Nodes[len(p.Nodes)-1].ID
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	sort.Strings(ids)
	return ids, nil
}
func (e *AgeGraphEngine) GetNeighborhood(ctx context.Context, resourceID string, depth int) ([]string, error) {
	return e.GetBlastRadius(ctx, resourceID, depth)
}
func (e *AgeGraphEngine) ShortestPath(ctx context.Context, fromID, toID string) ([]string, error) {
	from, err := ageVertexID(fromID)
	if err != nil {
		return nil, err
	}
	to, err := ageVertexID(toID)
	if err != nil {
		return nil, err
	}
	paths, err := e.paths(ctx, 8, "id(a) = $from AND id(b) = $to", map[string]any{"from": from, "to": to})
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return []string{}, nil
	}
	best := paths[0]
	for _, p := range paths {
		if p.Length < best.Length {
			best = p
		}
	}
	ids := make([]string, 0, len(best.Nodes))
	for _, n := range best.Nodes {
		ids = append(ids, n.ID)
	}
	return ids, nil
}
func (e *AgeGraphEngine) GetAccessibleSecrets(ctx context.Context, saID string) ([]string, error) {
	id, err := ageVertexID(saID)
	if err != nil {
		return nil, err
	}
	paths, err := e.paths(ctx, 8, "id(a) = $id", map[string]any{"id": id})
	if err != nil {
		return nil, err
	}
	result := []string{}
	seen := map[string]bool{}
	for _, p := range paths {
		n := p.Nodes[len(p.Nodes)-1]
		uid, ok := n.Properties["uid"].(string)
		if n.Type == "Secret" && ok && !seen[uid] {
			result = append(result, uid)
			seen[uid] = true
		}
	}
	sort.Strings(result)
	return result, nil
}
