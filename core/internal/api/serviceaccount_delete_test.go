package api

import (
	"testing"

	"github.com/fortuna/core/pkg/models"
)

func TestServiceAccountDeleteFailuresPreserveInventory(t *testing.T) {
	db, request := serviceAccountScopeFixture(t)
	for _, tc := range []struct {
		method, path, body, who string
		code                    int
	}{
		{"DELETE", "/inventory/serviceaccounts/sa-b", "", "a", 403},
		{"DELETE", "/inventory/serviceaccounts/sa-a", "", "viewer", 403},
		{"DELETE", "/inventory/serviceaccounts/sa-a", "", "missing", 401},
		{"DELETE", "/inventory/serviceaccounts/sa-a", "", "a", 503},
	} {
		w := request(tc.method, tc.path, tc.who, tc.body)
		if w.Code != tc.code {
			t.Errorf("%+v: %d %s", tc, w.Code, w.Body)
		}
	}
	var count int64
	if err := db.Model(&models.ServiceAccount{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("records changed: %d %v", count, err)
	}
}

func TestServiceAccountUpdateUsesSelectedClusterRow(t *testing.T) {
	db, request := serviceAccountScopeFixture(t)
	// The same Kubernetes UID observed in both clusters; cluster "b" sorts after "a",
	// so the cluster "a" caller must update its own row, never the foreign one.
	if err := db.Model(&models.ServiceAccount{}).Where("cluster_id = ?", "b").Update("uid", "sa-a").Error; err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path, who string
		code      int
	}{
		{"/inventory/serviceaccounts/sa-a", "admin", 409},
		{"/inventory/serviceaccounts/sa-a?clusterId=b", "a", 403},
		{"/inventory/serviceaccounts/sa-a", "a", 200},
	} {
		w := request("PUT", tc.path, tc.who, `{"labels":{"team":"x"}}`)
		if w.Code != tc.code {
			t.Errorf("%+v: %d %s", tc, w.Code, w.Body)
		}
	}
	var rows []models.ServiceAccount
	if err := db.Order("cluster_id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows[0].Labels != `{"team":"x"}` || rows[1].Labels != `{"keep":"yes"}` {
		t.Fatalf("labels: a=%s b=%s", rows[0].Labels, rows[1].Labels)
	}
}
