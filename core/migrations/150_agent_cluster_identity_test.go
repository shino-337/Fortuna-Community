package migrations

import (
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestAgentClusterIdentityMigration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.Exec("CREATE TABLE agents (id INTEGER PRIMARY KEY, agent_id TEXT, node_name TEXT)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO agents (agent_id,node_name) VALUES ('legacy','shared-node')").Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := Migration150_AgentClusterIdentity(db); err != nil {
			t.Fatal(err)
		}
	}
	var clusterID string
	if err := db.Table("agents").Select("cluster_id").Scan(&clusterID).Error; err != nil || clusterID != "" {
		t.Fatalf("must not guess legacy ownership: %q %v", clusterID, err)
	}
}
