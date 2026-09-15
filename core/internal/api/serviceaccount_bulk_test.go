package api

import (
	"github.com/fortuna/core/pkg/models"
	"testing"
)

func TestServiceAccountBulkFailuresPreserveInventory(t *testing.T) {
	db, request := serviceAccountScopeFixture(t)
	for _, tc := range []struct {
		path, body, who string
		code            int
	}{
		{"/bulk/delete", `{"ids":[1,2]}`, "a", 403},
		{"/bulk/delete", `{"ids":[1]}`, "bulk-only", 403},
		{"/bulk/delete", `{"ids":[1,1]}`, "a", 207},
		{"/bulk/delete", `{"ids":[999]}`, "a", 403},
		{"/bulk/delete", `{"ids":[0]}`, "a", 400},
		{"/bulk/delete", `{"ids":[1]}`, "missing", 401},
		{"/bulk/disable", `{"ids":[1]}`, "a", 501},
		{"/disable-inactive", "", "a", 501},
	} {
		w := request("POST", "/inventory/serviceaccounts"+tc.path, tc.who, tc.body)
		if w.Code != tc.code {
			t.Errorf("%+v: %d %s", tc, w.Code, w.Body)
		}
	}
	var count int64
	if err := db.Model(&models.ServiceAccount{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("records changed: %d %v", count, err)
	}
}
