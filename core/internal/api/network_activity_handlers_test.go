package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/fortuna/core/internal/middleware"
	"github.com/fortuna/core/pkg/models"
)

func TestGetNetworkActivityConnections_EmptyItemsArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.Pod{}, &models.PodNetworkConnection{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", &models.User{Role: models.RoleAdmin})
		c.Set(middleware.CtxNormalizedRole, models.RoleAdmin)
		c.Next()
	})
	r.GET("/runtime/network-activity", GetNetworkActivity(db))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/runtime/network-activity?cluster=cluster-a&view=connections&sinceMinutes=15", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}

	var resp struct {
		Items json.RawMessage `json:"items"`
		Total int64           `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Total != 0 {
		t.Fatalf("expected total 0, got %d", resp.Total)
	}
	if string(resp.Items) != "[]" {
		t.Fatalf("expected items [] not null, got %s", string(resp.Items))
	}
}

func TestGetNetworkActivityEdges_FiltersHostProbeSocketNoise(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&models.Pod{}, &models.PodNetworkConnection{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now().UTC()
	podUID := "falco-pod-uid"
	coreUID := "core-pod-uid"
	postgresUID := "postgres-pod-uid"
	pods := []models.Pod{
		{
			ClusterID: "cluster-a",
			UID:       podUID,
			Name:      "falco-node",
			Namespace: "fortuna",
			PodIP:     "10.244.0.151",
		},
		{
			ClusterID: "cluster-a",
			UID:       coreUID,
			Name:      "fortuna-core",
			Namespace: "fortuna",
			PodIP:     "10.244.0.207",
		},
		{
			ClusterID: "cluster-a",
			UID:       postgresUID,
			Name:      "postgres",
			Namespace: "fortuna",
			PodIP:     "10.244.0.157",
		},
	}
	if err := db.Create(&pods).Error; err != nil {
		t.Fatalf("seed pods: %v", err)
	}
	rows := []models.PodNetworkConnection{
		{
			PodUID:        podUID,
			ClusterID:     "cluster-a",
			Namespace:     "fortuna",
			ContainerName: "falco",
			SourceIP:      "10.244.0.151",
			SourcePort:    8765,
			DestIP:        "10.244.0.1",
			DestPort:      57044,
			Protocol:      "tcp",
			State:         "TIME_WAIT",
			RuntimeSource: "host",
			ObservedAt:    now,
			Bucket5m:      now,
		},
		{
			PodUID:        postgresUID,
			ClusterID:     "cluster-a",
			Namespace:     "fortuna",
			ContainerName: "postgres",
			SourceIP:      "10.244.0.157",
			SourcePort:    5432,
			DestIP:        "10.244.0.207",
			DestPort:      50324,
			Protocol:      "tcp",
			State:         "ESTABLISHED",
			RuntimeSource: "host",
			ObservedAt:    now,
			Bucket5m:      now,
		},
		{
			PodUID:        coreUID,
			ClusterID:     "cluster-a",
			Namespace:     "fortuna",
			ContainerName: "core",
			SourceIP:      "10.244.0.207",
			SourcePort:    50324,
			DestIP:        "10.109.73.28",
			DestPort:      5432,
			Protocol:      "tcp",
			State:         "ESTABLISHED",
			RuntimeSource: "host",
			ObservedAt:    now,
			Bucket5m:      now,
		},
		{
			PodUID:        podUID,
			ClusterID:     "cluster-a",
			Namespace:     "fortuna",
			ContainerName: "falco",
			SourceIP:      "0.0.0.0",
			SourcePort:    8765,
			DestIP:        "0.0.0.0",
			DestPort:      0,
			Protocol:      "tcp",
			State:         "LISTEN",
			RuntimeSource: "host",
			ObservedAt:    now,
			Bucket5m:      now,
		},
		{
			PodUID:        podUID,
			ClusterID:     "cluster-a",
			Namespace:     "fortuna",
			ContainerName: "falcoctl",
			SourceIP:      "10.244.0.151",
			SourcePort:    46112,
			DestIP:        "185.199.109.153",
			DestPort:      443,
			Protocol:      "tcp",
			State:         "ESTABLISHED",
			RuntimeSource: "host",
			ObservedAt:    now,
			Bucket5m:      now,
		},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed connections: %v", err)
	}

	var items []struct {
		DestIP   string `gorm:"column:dest_ip"`
		DestPort int    `gorm:"column:dest_port"`
	}
	qb := joinPodsForNetworkActivity(db.Table("pod_network_connections AS n")).
		Where("n.cluster_id = ?", "cluster-a")
	qb = applyNetworkActivityTopologyFlowFilter(qb)
	if err := qb.Select("n.dest_ip, n.dest_port").
		Group("n.dest_ip, n.dest_port").
		Order("n.dest_ip").
		Scan(&items).Error; err != nil {
		t.Fatalf("query filtered topology rows: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected two topology edges, got %+v", items)
	}
	if items[0].DestIP != "10.109.73.28" || items[0].DestPort != 5432 {
		t.Fatalf("expected postgres service edge first, got %+v", items[0])
	}
	if items[1].DestIP != "185.199.109.153" || items[1].DestPort != 443 {
		t.Fatalf("expected external falcoctl edge second, got %+v", items[1])
	}
}
