package api

import (
	"github.com/fortuna/core/pkg/models"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestMutationHTTPPermissionAndScope(t *testing.T) {
	db, request := serviceAccountScopeFixture(t)
	for _, job := range []models.ServiceAccountMutation{{ID: "operation-a", ClusterID: "a", UID: "sa-a", ActorID: 0, Action: "revoke", Plan: `{"version":1,"namespace":"shared","name":"shared","steps":[],"limitations":[]}`, Digest: "digest", Status: "preview", ExpiresAt: time.Now().Add(time.Minute)}, {ID: "operation-b", ClusterID: "b", UID: "sa-b", ActorID: 0, Action: "revoke", Plan: `{}`, Digest: "digest", Status: "preview", ExpiresAt: time.Now().Add(time.Minute)}, {ID: "operation-other-actor", ClusterID: "a", UID: "sa-a", ActorID: 999, Action: "revoke", Plan: `{}`, Digest: "digest", Status: "preview", ExpiresAt: time.Now().Add(time.Minute)}} {
		require.NoError(t, db.Create(&job).Error)
	}
	for _, tc := range []struct {
		method, path, who, body string
		code                    int
	}{{"GET", "/inventory/serviceaccount-mutations/operation-a", "a", "", 200}, {"GET", "/inventory/serviceaccount-mutations/operation-b", "a", "", 404}, {"GET", "/inventory/serviceaccount-mutations/operation-other-actor", "a", "", 404}, {"POST", "/inventory/serviceaccount-mutations/operation-a/execute", "viewer", `{"digest":"digest"}`, 403}, {"POST", "/inventory/serviceaccount-mutations/operation-a/execute", "a", `{"digest":"changed"}`, 409}, {"POST", "/inventory/serviceaccounts/sa-a/mutations/preview", "viewer", `{"action":"revoke"}`, 403}} {
		w := request(tc.method, tc.path, tc.who, tc.body)
		require.Equal(t, tc.code, w.Code, w.Body.String())
	}
	var job models.ServiceAccountMutation
	require.NoError(t, db.First(&job, "id = ?", "operation-a").Error)
	require.Equal(t, "preview", job.Status)
}
