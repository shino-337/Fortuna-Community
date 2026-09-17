package api

import (
	"context"
	"encoding/json"
	"github.com/fortuna/core/pkg/models"
	"testing"
	"time"
)

func TestAgentClusterIdentityIsolation(t *testing.T) {
	db, request := serviceAccountScopeFixture(t)
	if err := db.AutoMigrate(&models.Agent{}); err != nil {
		t.Fatal(err)
	}

	// The same external AgentID is valid in independent clusters. Identity is the
	// composite {cluster_id,agent_id}, not agent_id alone.
	for _, cl := range []string{"a", "b"} {
		if err := upsertAgentClusterIdentity(context.Background(), db, cl, &AgentPayload{AgentID: "shared-agent", NodeName: "same-node", Version: "1"}); err != nil {
			t.Fatalf("upsert %s: %v", cl, err)
		}
	}
	var shared []models.Agent
	if err := db.Where("agent_id = ?", "shared-agent").Order("cluster_id").Find(&shared).Error; err != nil {
		t.Fatal(err)
	}
	if len(shared) != 2 || shared[0].ClusterID != "a" || shared[1].ClusterID != "b" {
		t.Fatalf("composite identity rows: %+v", shared)
	}

	// Updating cluster b must not mutate the independently-owned cluster-a row.
	if err := upsertAgentClusterIdentity(context.Background(), db, "b", &AgentPayload{AgentID: "shared-agent", NodeName: "node-b-updated", Version: "2"}); err != nil {
		t.Fatal(err)
	}
	var rowA, rowB models.Agent
	if err := db.Where("cluster_id = ? AND agent_id = ?", "a", "shared-agent").First(&rowA).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Where("cluster_id = ? AND agent_id = ?", "b", "shared-agent").First(&rowB).Error; err != nil {
		t.Fatal(err)
	}
	if rowA.NodeName != "same-node" || rowB.NodeName != "node-b-updated" {
		t.Fatalf("cross-cluster agent mutation: a=%+v b=%+v", rowA, rowB)
	}

	old := time.Now().Add(-20 * time.Minute)
	if err := db.Model(&models.Agent{}).Where("cluster_id = ? AND agent_id = ?", "a", "shared-agent").Update("last_seen_at", old).Error; err != nil {
		t.Fatal(err)
	}

	// Legacy empty-cluster rows are claimed once by the authenticated/synced
	// cluster and become a normal composite identity.
	if err := db.Create(&models.Agent{AgentID: "legacy", NodeName: "same-node", Status: "ready"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := upsertAgentClusterIdentity(context.Background(), db, "a", &AgentPayload{AgentID: "legacy", NodeName: "same-node"}); err != nil {
		t.Fatal(err)
	}
	var legacy models.Agent
	if err := db.Where("agent_id = ?", "legacy").First(&legacy).Error; err != nil || legacy.ClusterID != "a" {
		t.Fatalf("legacy assignment: %+v %v", legacy, err)
	}

	w := request("GET", "/inventory/clusters/a/agents", "a", "")
	var out struct {
		Agents []struct {
			AgentID  string `json:"agentId"`
			NodeName string `json:"nodeName"`
			Status   string
		}
		Total int
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || out.Total != 2 {
		t.Fatalf("cluster isolation/health: %d %s", w.Code, w.Body)
	}
	seenShared := false
	seenLegacy := false
	for _, a := range out.Agents {
		switch a.AgentID {
		case "shared-agent":
			seenShared = true
			if a.Status != "disconnected" {
				t.Fatalf("cluster-a shared-agent status=%s, want disconnected", a.Status)
			}
		case "legacy":
			seenLegacy = true
		}
	}
	if !seenShared || !seenLegacy {
		t.Fatalf("cluster-a inventory missing agents: %s", w.Body)
	}
}
