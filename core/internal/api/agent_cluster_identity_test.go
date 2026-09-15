package api

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/fortuna/core/pkg/models"
	"testing"
	"time"
)

func TestAgentClusterIdentityIsolation(t *testing.T) {
	db, request := serviceAccountScopeFixture(t)
	if err := db.AutoMigrate(&models.Agent{}); err != nil {
		t.Fatal(err)
	}
	for _, cl := range []string{"a", "b"} {
		if err := upsertAgentClusterIdentity(context.Background(), db, cl, &AgentPayload{AgentID: "agent-" + cl, NodeName: "same-node"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := upsertAgentClusterIdentity(context.Background(), db, "b", &AgentPayload{AgentID: "agent-a", NodeName: "hijack"}); !errors.Is(err, errAgentClusterConflict) {
		t.Fatalf("cluster reassignment: %v", err)
	}
	old := time.Now().Add(-20 * time.Minute)
	if err := db.Model(&models.Agent{}).Where("agent_id = ?", "agent-a").Update("last_seen_at", old).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Agent{AgentID: "legacy", NodeName: "same-node", Status: "ready"}).Error; err != nil {
		t.Fatal(err)
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
	if w.Code != 200 || out.Total != 1 || out.Agents[0].AgentID != "agent-a" || out.Agents[0].NodeName != "same-node" || out.Agents[0].Status != "disconnected" {
		t.Fatalf("cluster isolation/health: %d %s", w.Code, w.Body)
	}
	if err := upsertAgentClusterIdentity(context.Background(), db, "a", &AgentPayload{AgentID: "legacy", NodeName: "same-node"}); err != nil {
		t.Fatal(err)
	}
	var row models.Agent
	if err := db.Where("agent_id = ?", "legacy").First(&row).Error; err != nil || row.ClusterID != "a" {
		t.Fatalf("legacy assignment: %+v %v", row, err)
	}
}
