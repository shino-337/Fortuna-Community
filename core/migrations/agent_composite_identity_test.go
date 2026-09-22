package migrations

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fortuna/core/pkg/agentidentity"
	"github.com/fortuna/core/pkg/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestAgentCompositeIdentityPostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL is not configured")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	adminSQL, err := admin.DB()
	require.NoError(t, err)
	defer adminSQL.Close()
	schema := fmt.Sprintf("agent_identity_%d", time.Now().UnixNano())
	require.NoError(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	defer admin.Exec("DROP SCHEMA " + schema + " CASCADE")
	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		require.NoError(t, err)
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		dsn = u.String()
	} else {
		dsn += " search_path=" + schema
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(8)

	require.NoError(t, db.AutoMigrate(&models.Agent{}))
	require.NoError(t, db.Exec("DROP INDEX idx_agents_cluster_agent").Error)
	require.NoError(t, db.Exec("CREATE UNIQUE INDEX idx_agents_agent_id ON agents(agent_id)").Error)
	legacy := models.Agent{AgentID: "same", NodeName: "legacy", Capabilities: "[]"}
	require.NoError(t, db.Create(&legacy).Error)
	for i := 0; i < 2; i++ {
		require.NoError(t, EnsureAgentCompositeIdentity(db))
	}
	var wg sync.WaitGroup
	errs := make(chan error, 24)
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- agentidentity.UpsertRecord(context.Background(), db, &models.Agent{ClusterID: fmt.Sprintf("cluster-%d", i%2), AgentID: "same", NodeName: fmt.Sprintf("node-%d", i%2), Version: "v48"})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var rows []models.Agent
	require.NoError(t, db.Order("cluster_id").Find(&rows).Error)
	require.Len(t, rows, 3)
	require.Equal(t, legacy.ID, rows[0].ID)
	require.Equal(t, "", rows[0].ClusterID)
	require.Equal(t, "legacy", rows[0].NodeName)
	require.Equal(t, "node-0", rows[1].NodeName)
	require.Equal(t, "node-1", rows[2].NodeName)
	require.NoError(t, db.Delete(&rows[1]).Error)
	require.NoError(t, agentidentity.UpsertRecord(context.Background(), db, &models.Agent{ClusterID: "cluster-0", AgentID: "same", NodeName: "restored"}))
	var restored models.Agent
	require.NoError(t, db.First(&restored, rows[1].ID).Error)
	require.Equal(t, "restored", restored.NodeName)
	require.Equal(t, "v48", restored.Version)
	require.Error(t, db.Exec("UPDATE agents SET cluster_id='foreign' WHERE id=?", rows[1].ID).Error)
	require.Error(t, db.Create(&models.Agent{AgentID: "unowned", Capabilities: "[]"}).Error)
	require.Error(t, db.Create(&models.Agent{ClusterID: "cluster-0", AgentID: "same", Capabilities: "[]"}).Error)
	require.NoError(t, EnsureAgentCompositeIdentity(db))
}
