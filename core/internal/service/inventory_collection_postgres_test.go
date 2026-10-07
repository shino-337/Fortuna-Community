package service

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fortuna/api/collection"
	"github.com/fortuna/core/migrations"
	"github.com/fortuna/core/pkg/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestInventoryCollectionPostgres(t *testing.T) {
	dsn := os.Getenv("FORTUNA_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FORTUNA_TEST_POSTGRES_URL is not configured")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlAdmin, err := admin.DB()
	require.NoError(t, err)
	defer sqlAdmin.Close()
	schema := fmt.Sprintf("collection_%d", time.Now().UnixNano())
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
	pool, err := db.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(8)
	defer pool.Close()
	require.NoError(t, db.AutoMigrate(&models.Cluster{}, &models.Agent{}, &models.User{}, &models.AuditLog{}, &models.Role{}))
	require.NoError(t, migrations.EnsureInventoryCollection(db))
	require.NoError(t, migrations.EnsureInventoryCollection(db))
	require.NoError(t, db.Create(&models.User{Username: "system", Email: "system@test", Password: "unused", Role: models.RoleSystem, Active: true}).Error)
	data := map[string]interface{}{"isFullSync": true, "isDeltaSync": false}
	counts := map[string]int{}
	for _, kind := range collection.InventoryKinds {
		data[kind] = []interface{}{}
		counts[kind] = 0
	}
	data["roles"] = []interface{}{map[string]interface{}{"uid": "role-a", "name": "role", "namespace": "ns", "rules": []interface{}{}}}
	counts["roles"] = 1
	now := time.Now().UTC()
	kindStartedAt := map[string]time.Time{}
	for _, kind := range collection.InventoryKinds {
		kindStartedAt[kind] = now.Add(-time.Second)
	}
	c := collection.Inventory{Version: 1, ID: "postgres-attempt-0001", Namespace: "ns", Status: "complete", StartedAt: now.Add(-time.Minute), ObservedAt: now, KindStartedAt: kindStartedAt, Counts: counts}
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := NewAgentService(db).WithAgentRecord(&models.Agent{ClusterID: "cluster-a", AgentID: "agent-a", NodeName: "node-a", Version: "v50"}).SyncObservedData(context.Background(), "cluster-a", "A", "auto", "", "", "agent-a", data, "", c)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var n int64
	require.NoError(t, db.Model(&models.Role{}).Count(&n).Error)
	require.EqualValues(t, 1, n)
	var acceptedAgent models.Agent
	require.NoError(t, db.Where("cluster_id = ? AND agent_id = ?", "cluster-a", "agent-a").First(&acceptedAgent).Error)
	require.Equal(t, "node-a", acceptedAgent.NodeName)
	require.NotNil(t, acceptedAgent.LastSeenAt)

	// DaemonSet topology is intentionally multi-writer. A newer failed Agent may
	// make evidence unavailable, but it must not mutate the accepted projection;
	// an older writer must not roll the receipt back, and a later complete writer
	// must recover deterministically.
	multi := c
	multi.ID = "postgres-agent-b-fail"
	multi.Status = "failed"
	multi.StartedAt = c.StartedAt.Add(2 * time.Second)
	failedReceipt, err := NewAgentService(db).
		WithAgentRecord(&models.Agent{ClusterID: "cluster-a", AgentID: "agent-b", NodeName: "node-b", Version: "v50"}).
		SyncObservedData(context.Background(), "cluster-a", "A", "auto", "", "", "agent-b", nil, "", multi)
	require.NoError(t, err)
	require.Equal(t, "failed", failedReceipt.Status)
	var unchanged models.Role
	require.NoError(t, db.Where("cluster_id = ? AND uid = ?", "cluster-a", "role-a").First(&unchanged).Error)
	require.Equal(t, "role", unchanged.Name)

	older := c
	older.ID = "postgres-agent-a-older"
	older.StartedAt = c.StartedAt.Add(time.Second)
	_, err = NewAgentService(db).
		WithAgentRecord(&models.Agent{ClusterID: "cluster-a", AgentID: "agent-a", NodeName: "node-a", Version: "v50"}).
		SyncObservedData(context.Background(), "cluster-a", "A", "auto", "", "", "agent-a", data, "", older)
	require.ErrorIs(t, err, ErrCollectionConflict)

	recovered := c
	recovered.ID = "postgres-agent-a-recover"
	recovered.StartedAt = c.StartedAt.Add(3 * time.Second)
	recoveredReceipt, err := NewAgentService(db).
		WithAgentRecord(&models.Agent{ClusterID: "cluster-a", AgentID: "agent-a", NodeName: "node-a", Version: "v50"}).
		SyncObservedData(context.Background(), "cluster-a", "A", "auto", "", "", "agent-a", data, "", recovered)
	require.NoError(t, err)
	require.Equal(t, "complete", recoveredReceipt.Status)
	require.Equal(t, "agent-a", recoveredReceipt.AgentID)

	// Continue failure/rollback checks from the latest accepted ordering watermark.
	c = recovered
	beforeFailureSeen := *acceptedAgent.LastSeenAt
	require.NoError(t, db.Where("cluster_id = ? AND agent_id = ?", "cluster-a", "agent-a").First(&acceptedAgent).Error)
	beforeFailureSeen = *acceptedAgent.LastSeenAt
	// Real PostgreSQL error aborts the transaction; neither projection nor receipt
	// can report success. Persist the failed attempt only after rollback.
	require.NoError(t, db.Exec(`CREATE FUNCTION deny_role_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected failure'; END $$`).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER deny_role_write BEFORE UPDATE ON roles FOR EACH ROW EXECUTE FUNCTION deny_role_write()`).Error)
	data["roles"] = []interface{}{map[string]interface{}{"uid": "role-a", "name": "changed", "namespace": "ns", "rules": []interface{}{}}}
	c.ID = "postgres-attempt-0002"
	c.StartedAt = c.StartedAt.Add(time.Second)
	_, err = NewAgentService(db).WithAgentRecord(&models.Agent{ClusterID: "cluster-a", AgentID: "agent-a", NodeName: "node-b", Version: "v51"}).SyncObservedData(context.Background(), "cluster-a", "Changed", "auto", "", "", "agent-a", data, "", c)
	require.Error(t, err)
	var role models.Role
	require.NoError(t, db.First(&role).Error)
	require.Equal(t, "role", role.Name)
	var cl models.Cluster
	require.NoError(t, db.First(&cl, "id = ?", "cluster-a").Error)
	require.Equal(t, "A", cl.Name)
	var rolledBackAgent models.Agent
	require.NoError(t, db.Where("cluster_id = ? AND agent_id = ?", "cluster-a", "agent-a").First(&rolledBackAgent).Error)
	require.Equal(t, "node-a", rolledBackAgent.NodeName)
	require.Equal(t, "v50", rolledBackAgent.Version)
	require.True(t, rolledBackAgent.LastSeenAt.Equal(beforeFailureSeen))
	var state models.InventoryCollection
	require.NoError(t, db.First(&state).Error)
	require.Equal(t, "failed", state.Status)
	require.NoError(t, db.Exec(`DROP TRIGGER deny_role_write ON roles`).Error)
	c.ID = "postgres-attempt-0003"
	c.StartedAt = c.StartedAt.Add(time.Second)
	_, err = NewAgentService(db).WithAgentRecord(&models.Agent{ClusterID: "cluster-a", AgentID: "agent-a", NodeName: "node-b", Version: "v51"}).SyncObservedData(context.Background(), "cluster-a", "Changed", "auto", "", "", "agent-a", data, "", c)
	require.NoError(t, err)
	require.NoError(t, db.First(&state).Error)
	require.Equal(t, "complete", state.Status)
	var recoveredAgent models.Agent
	require.NoError(t, db.Where("cluster_id = ? AND agent_id = ?", "cluster-a", "agent-a").First(&recoveredAgent).Error)
	require.Equal(t, "node-b", recoveredAgent.NodeName)
	require.Equal(t, "v51", recoveredAgent.Version)
	// Startup reruns must preserve the accepted receipt on a populated table.
	require.NoError(t, migrations.EnsureInventoryCollection(db))
	var afterRestart models.InventoryCollection
	require.NoError(t, db.First(&afterRestart).Error)
	require.Equal(t, state, afterRestart)
}
