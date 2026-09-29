package graph

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestAGECanonicalScopeAndIdentifiers(t *testing.T) {
	_, err := clusterGraphName("")
	require.Error(t, err)
	_, err = clusterGraphName(" a ")
	require.Error(t, err)
	a, err := clusterGraphName("cluster-a")
	require.NoError(t, err)
	b, err := clusterGraphName("cluster-b")
	require.NoError(t, err)
	require.NotEqual(t, a, b)
	require.LessOrEqual(t, len(a), 63)
	for _, id := range []string{"1;DELETE", "-1", "01", "1.0", "9223372036854775808"} {
		_, err = ageVertexID(id)
		require.Error(t, err)
	}
	engine := &AgeGraphEngine{clusterID: "cluster-a"}
	_, err = engine.scopedProperties(map[string]any{"cluster_id": "cluster-b"})
	require.Error(t, err)
	_, err = engine.scopedProperties(map[string]any{"bad": func() {}})
	require.Error(t, err)
}
func TestAGEScopedTraversalPostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL required")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	defer pool.Close()
	suffix := fmt.Sprint(time.Now().UnixNano())
	a, err := NewAgeGraphEngine(db, "cluster-a-"+suffix)
	require.NoError(t, err)
	b, err := NewAgeGraphEngine(db, "cluster-b-"+suffix)
	require.NoError(t, err)
	ctx := context.Background()
	for _, engine := range []*AgeGraphEngine{a, b} {
		require.NoError(t, engine.Initialize(ctx))
		defer func(e *AgeGraphEngine) {
			conn, err := e.connection(ctx)
			if err == nil {
				defer conn.Close()
				_, _ = conn.ExecContext(ctx, "SELECT ag_catalog.drop_graph($1,true)", e.graphName)
			}
		}(engine)
	}
	// The same resource UID/name in both clusters produces isolated graphs.
	from, err := a.CreateVertex(ctx, "Pod", map[string]any{"uid": "duplicate-pod", "name": "same", "literal": "$fortuna$ ' \\ ::vertex"})
	require.NoError(t, err)
	to, err := a.CreateVertex(ctx, "Secret", map[string]any{"uid": "duplicate-secret"})
	require.NoError(t, err)
	_, err = a.CreateEdge(ctx, from, to, "CAN_READ", map[string]any{})
	require.NoError(t, err)
	foreign, err := b.CreateVertex(ctx, "Secret", map[string]any{"uid": "duplicate-secret", "name": "foreign"})
	require.NoError(t, err)
	// AGE numeric IDs may coincide across graphs; graph scope, not numeric IDs,
	// determines ownership. Reading A must never resolve B's properties.
	result, err := a.GetBlastRadius(ctx, from, 2)
	require.NoError(t, err)
	require.Equal(t, []string{to}, result)
	_, err = a.CreateVertex(ctx, "Pod", map[string]any{"cluster_id": b.clusterID})
	require.Error(t, err)
	_, err = a.CreateEdge(ctx, from, to, "CAN_READ", map[string]any{"cluster_id": b.clusterID})
	require.Error(t, err)
	_, err = a.CreateEdge(ctx, from, "1;DELETE", "CAN_READ", nil)
	require.Error(t, err)
	_, err = a.CreateVertex(ctx, "Pod) DELETE v", nil)
	require.Error(t, err)
	_ = foreign
	service := &QueryService{engine: a}
	paths, err := service.FindPaths(ctx, "duplicate-pod", "duplicate-secret", PathConstraint{MaxDepth: 2})
	require.NoError(t, err)
	require.Len(t, paths, 1)
	require.Len(t, paths[0].Nodes, 2)
	require.Len(t, paths[0].Edges, 1)
	for _, node := range paths[0].Nodes {
		require.Equal(t, a.clusterID, node.Properties["cluster_id"])
		require.NotEqual(t, "foreign", node.Properties["name"])
	}
	// Inject foreign legacy evidence directly into A to prove vertex AND edge
	// filters hold even when an operator imported malformed data.
	_, err = a.query(ctx, "MATCH (a),(b) WHERE id(a) = $from AND id(b) = $to CREATE (a)-[e:FOREIGN {cluster_id: $foreign}]->(b) RETURN id(e)", "id agtype", map[string]any{"from": mustAGEID(t, from), "to": mustAGEID(t, to), "foreign": b.clusterID})
	require.NoError(t, err)
	paths, err = service.FindPaths(ctx, "duplicate-pod", "duplicate-secret", PathConstraint{MaxDepth: 2})
	require.NoError(t, err)
	require.Len(t, paths, 1, "foreign edge must be excluded")
	_, err = a.query(ctx, "MATCH (v) WHERE id(v) = $id SET v.cluster_id = $foreign RETURN id(v)", "id agtype", map[string]any{"id": mustAGEID(t, to), "foreign": b.clusterID})
	require.NoError(t, err)
	paths, err = service.FindPaths(ctx, "duplicate-pod", "duplicate-secret", PathConstraint{MaxDepth: 2})
	require.NoError(t, err)
	require.Empty(t, paths, "foreign vertex must be excluded")
	_, err = service.FindPaths(ctx, "duplicate-pod", "duplicate-secret", PathConstraint{MaxDepth: 2, RequireRuntimeEvidence: true})
	require.Error(t, err)
}
func mustAGEID(t *testing.T, value string) int64 {
	t.Helper()
	id, err := ageVertexID(value)
	require.NoError(t, err)
	return id
}
