package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fortuna/api/collection"
	"github.com/fortuna/core/migrations"
	"github.com/fortuna/core/pkg/models"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func collectionFixture(t *testing.T) (*gorm.DB, *AgentService, map[string]interface{}, collection.Inventory) {
	t.Helper()
	db := openTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.Cluster{}, &models.Agent{}, &models.User{}, &models.AuditLog{}, &models.Role{}, &models.ClusterRole{}, &models.RoleBinding{}, &models.ClusterRoleBinding{}, &models.ServiceAccount{}, &models.Pod{}, &models.Deployment{}, &models.ReplicaSet{}))
	require.NoError(t, migrations.EnsureInventoryCollection(db))
	require.NoError(t, migrations.EnsureInventoryCollection(db))
	// Keep the receipt transaction independent of password bootstrap.
	require.NoError(t, db.Create(&models.User{Username: "system", Email: "system@test", Password: "unused", Role: models.RoleAdmin, Active: true}).Error)
	data := map[string]interface{}{"isFullSync": true, "isDeltaSync": false}
	counts := map[string]int{}
	for _, kind := range collection.InventoryKinds {
		data[kind] = []interface{}{}
		counts[kind] = 0
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	kindStartedAt := map[string]time.Time{}
	for _, kind := range collection.InventoryKinds {
		kindStartedAt[kind] = now.Add(-time.Second)
	}
	c := collection.Inventory{Version: 1, ID: "attempt-0000000001", Status: "complete", StartedAt: now.Add(-time.Minute), ObservedAt: now, KindStartedAt: kindStartedAt, Counts: counts}
	return db, NewAgentService(db), data, c
}

func TestInventoryCollectionCommitAndFailure(t *testing.T) {
	db, s, data, c := collectionFixture(t)
	s.WithAgentRecord(&models.Agent{ClusterID: "cluster-a", AgentID: "agent-a", NodeName: "node-a", Version: "v50"})
	data["roles"] = []interface{}{map[string]interface{}{"uid": "role-a", "name": "role", "namespace": "ns", "rules": []interface{}{}}}
	c.Counts["roles"] = 1
	receipt, err := s.SyncObservedData(context.Background(), "cluster-a", "A", "auto", "", "", "agent-a", data, "", c)
	require.NoError(t, err)
	require.Equal(t, "complete", receipt.Status)
	var acceptedAgent models.Agent
	require.NoError(t, db.Where("cluster_id = ? AND agent_id = ?", "cluster-a", "agent-a").First(&acceptedAgent).Error)
	require.Equal(t, "node-a", acceptedAgent.NodeName)
	require.Equal(t, "v50", acceptedAgent.Version)
	// Replay is idempotent for inventory evidence. The accepted request may still
	// refresh Agent liveness, so capture the rollback baseline after the replay.
	replay, err := s.SyncObservedData(context.Background(), "cluster-a", "A", "auto", "", "", "agent-a", data, "", c)
	require.NoError(t, err)
	require.Equal(t, receipt.ReceivedAt, replay.ReceivedAt)
	var beforeFailureAgent models.Agent
	require.NoError(t, db.Where("cluster_id = ? AND agent_id = ?", "cluster-a", "agent-a").First(&beforeFailureAgent).Error)
	require.NotNil(t, beforeFailureAgent.LastSeenAt)
	beforeFailureSeen := *beforeFailureAgent.LastSeenAt
	// A storage failure rolls back all inventory changes and records failed status.
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("test:role-write-failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "roles" {
			tx.AddError(errors.New("injected write failure"))
		}
	}))
	data["roles"] = []interface{}{map[string]interface{}{"uid": "new-role", "name": "new", "namespace": "ns", "rules": []interface{}{}}}
	c.ID = "attempt-0000000002"
	c.StartedAt = c.StartedAt.Add(time.Second)
	s.WithAgentRecord(&models.Agent{ClusterID: "cluster-a", AgentID: "agent-a", NodeName: "node-b", Version: "v51"})
	_, err = s.SyncObservedData(context.Background(), "cluster-a", "B", "auto", "", "", "agent-a", data, "", c)
	require.Error(t, err)
	var state models.InventoryCollection
	require.NoError(t, db.First(&state, "cluster_id = ?", "cluster-a").Error)
	require.Equal(t, "failed", state.Status)
	var role models.Role
	require.NoError(t, db.Where("cluster_id = ? AND uid = ?", "cluster-a", "role-a").First(&role).Error)
	var cluster models.Cluster
	require.NoError(t, db.First(&cluster, "id = ?", "cluster-a").Error)
	require.Equal(t, "A", cluster.Name)
	var rolledBackAgent models.Agent
	require.NoError(t, db.Where("cluster_id = ? AND agent_id = ?", "cluster-a", "agent-a").First(&rolledBackAgent).Error)
	require.Equal(t, "node-a", rolledBackAgent.NodeName)
	require.Equal(t, "v50", rolledBackAgent.Version)
	require.True(t, rolledBackAgent.LastSeenAt.Equal(beforeFailureSeen), "failed inventory must not advance Agent liveness")
	var count int64
	require.NoError(t, db.Model(&models.Role{}).Where("uid = ?", "new-role").Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Callback().Create().Remove("test:role-write-failure"))
	c.ID = "attempt-0000000003"
	c.StartedAt = c.StartedAt.Add(time.Second)
	s.WithAgentRecord(&models.Agent{ClusterID: "cluster-a", AgentID: "agent-a", NodeName: "node-b", Version: "v51"})
	_, err = s.SyncObservedData(context.Background(), "cluster-a", "B", "auto", "", "", "agent-a", data, "", c)
	require.NoError(t, err)
	require.NoError(t, db.First(&state, "cluster_id = ?", "cluster-a").Error)
	require.Equal(t, "complete", state.Status)
	var recoveredAgent models.Agent
	require.NoError(t, db.Where("cluster_id = ? AND agent_id = ?", "cluster-a", "agent-a").First(&recoveredAgent).Error)
	require.Equal(t, "node-b", recoveredAgent.NodeName)
	require.Equal(t, "v51", recoveredAgent.Version)
	require.Equal(t, "stale", state.EffectiveStatus(state.ObservedAt.Add(collection.MaxAge+time.Second)))
}

func TestInventoryCollectionEmptyMissingReplayAndScope(t *testing.T) {
	for _, tc := range []string{"empty", "missing", "null", "count", "duplicate", "scope", "stale", "failed", "replay-change", "out-of-order", "namespace-prune", "legacy-invalidate"} {
		t.Run(tc, func(t *testing.T) {
			db, s, data, c := collectionFixture(t)
			require.NoError(t, db.Create(&models.Cluster{ID: "cluster-a", Name: "A"}).Error)
			require.NoError(t, db.Create(&models.Role{ClusterID: "cluster-a", UID: "foreign-ns", Name: "foreign", Namespace: "other", Rules: "[]"}).Error)
			wantErr := false
			switch tc {
			case "missing":
				delete(data, "pods")
				wantErr = true
			case "null":
				data["pods"] = nil
				wantErr = true
			case "count":
				c.Counts["roles"] = 1
				wantErr = true
			case "duplicate":
				data["roles"] = []interface{}{map[string]interface{}{"uid": "r", "name": "r", "namespace": "ns", "rules": []interface{}{}}, map[string]interface{}{"uid": "r", "name": "r", "namespace": "ns", "rules": []interface{}{}}}
				c.Counts["roles"] = 2
				wantErr = true
			case "scope":
				c.Namespace = "ns"
				data["roles"] = []interface{}{map[string]interface{}{"uid": "r", "name": "r", "namespace": "other", "rules": []interface{}{}}}
				c.Counts["roles"] = 1
				wantErr = true
			case "stale":
				c.StartedAt = time.Now().Add(-time.Hour)
				wantErr = true
			case "failed":
				c.Status = "failed"
				data = nil
			case "namespace-prune":
				c.Namespace = "ns"
				require.NoError(t, db.Create(&models.ServiceAccount{ClusterID: "cluster-a", UID: "sa-foreign", Name: "sa-foreign", Namespace: "other"}).Error)
				require.NoError(t, db.Create(&models.RoleBinding{ClusterID: "cluster-a", UID: "rb-foreign", Name: "rb-foreign", Namespace: "other", RoleRef: "{}", Subjects: "[]"}).Error)
				require.NoError(t, db.Create(&models.Deployment{ClusterID: "cluster-a", UID: "dep-foreign", Name: "dep-foreign", Namespace: "other"}).Error)
				require.NoError(t, db.Create(&models.ReplicaSet{ClusterID: "cluster-a", UID: "rs-foreign", Name: "rs-foreign", Namespace: "other"}).Error)
				data["serviceAccounts"] = []interface{}{map[string]interface{}{"uid": "sa-local", "name": "sa-local", "namespace": "ns"}}
				data["roles"] = []interface{}{map[string]interface{}{"uid": "r", "name": "r", "namespace": "ns", "rules": []interface{}{}}}
				data["roleBindings"] = []interface{}{map[string]interface{}{"uid": "rb-local", "name": "rb-local", "namespace": "ns", "roleRef": map[string]interface{}{"apiGroup": "rbac.authorization.k8s.io", "kind": "Role", "name": "r"}, "subjects": []interface{}{}}}
				data["deployments"] = []interface{}{map[string]interface{}{"uid": "dep-local", "name": "dep-local", "namespace": "ns"}}
				data["replicasets"] = []interface{}{map[string]interface{}{"uid": "rs-local", "name": "rs-local", "namespace": "ns"}}
				for _, kind := range []string{"serviceAccounts", "roles", "roleBindings", "deployments", "replicasets"} {
					c.Counts[kind] = 1
				}
			}
			receipt, err := s.SyncObservedData(context.Background(), "cluster-a", "A", "auto", "", "", "agent-a", data, "", c)
			if wantErr {
				require.ErrorIs(t, err, ErrInvalidCollection)
				return
			}
			require.NoError(t, err)
			switch tc {
			case "replay-change":
				data["extra"] = "changed"
				_, err = s.SyncObservedData(context.Background(), "cluster-a", "A", "auto", "", "", "agent-a", data, "", c)
				require.ErrorIs(t, err, ErrCollectionConflict)
			case "out-of-order":
				c.ID = "older-00000000001"
				c.StartedAt = c.StartedAt.Add(-time.Second)
				_, err = s.SyncObservedData(context.Background(), "cluster-a", "A", "auto", "", "", "agent-a", data, "", c)
				require.ErrorIs(t, err, ErrCollectionConflict)
			case "failed":
				require.Equal(t, "failed", receipt.Status)
			case "legacy-invalidate":
				require.NoError(t, s.SyncData("cluster-a", "A", "auto", "", "", map[string]interface{}{"isDeltaSync": true}, ""))
				var state models.InventoryCollection
				require.NoError(t, db.First(&state, "cluster_id = ?", "cluster-a").Error)
				require.Equal(t, "unknown", state.Status)
			case "namespace-prune":
				require.Equal(t, "complete", receipt.Status)
				for _, check := range []struct {
					model interface{}
					uid   string
				}{
					{&models.ServiceAccount{}, "sa-foreign"},
					{&models.RoleBinding{}, "rb-foreign"},
					{&models.Deployment{}, "dep-foreign"},
					{&models.ReplicaSet{}, "rs-foreign"},
				} {
					var n int64
					require.NoError(t, db.Model(check.model).Where("cluster_id = ? AND uid = ? AND deleted_at IS NULL", "cluster-a", check.uid).Count(&n).Error)
					require.EqualValues(t, 1, n, "namespace-scoped sync pruned foreign namespace resource %s", check.uid)
				}
			default:
				require.Equal(t, "complete", receipt.Status)
			}
			var foreign models.Role
			require.NoError(t, db.Where("uid = ?", "foreign-ns").First(&foreign).Error)
		})
	}
}


func TestInventoryCollectionRejectsCrossNamespaceRows(t *testing.T) {
	cases := map[string]map[string]interface{}{
		"serviceAccounts": {"uid": "sa", "name": "sa", "namespace": "other"},
		"roles": {"uid": "role", "name": "role", "namespace": "other", "rules": []interface{}{}},
		"roleBindings": {"uid": "rb", "name": "rb", "namespace": "other", "roleRef": map[string]interface{}{"apiGroup": "rbac.authorization.k8s.io", "kind": "Role", "name": "role"}, "subjects": []interface{}{}},
		"deployments": {"uid": "dep", "name": "dep", "namespace": "other"},
		"replicasets": {"uid": "rs", "name": "rs", "namespace": "other"},
		"pods": {"uid": "pod", "name": "pod", "namespace": "other", "hostNetwork": false, "hostPID": false, "hostIPC": false, "automountServiceAccountToken": true, "containers": []interface{}{map[string]interface{}{"name": "c"}}},
	}
	for kind, row := range cases {
		t.Run(kind, func(t *testing.T) {
			_, _, data, meta := collectionFixture(t)
			meta.Namespace = "ns"
			data[kind] = []interface{}{row}
			meta.Counts[kind] = 1
			require.ErrorIs(t, ValidateInventoryPayload(meta, data, time.Now()), ErrInvalidCollection)
		})
	}
}
