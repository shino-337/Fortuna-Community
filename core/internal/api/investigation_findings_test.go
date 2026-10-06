package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/fortuna/core/pkg/models"
)

func investigationFindingsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := reviewDB(t, &models.Insight{}, &models.Pod{}, &models.Cluster{}, &models.RiskScore{},
		&models.InvestigationCase{}, &models.InvestigationActivityLog{}, &models.SecurityActivityLog{})
	// Insight 1 is in cluster c1, insight 2 in cluster c2.
	seedTwoClusterFindings(t, db)
	return db
}

func scopedCaseOwner(clusters ...string) *models.User {
	u := scopedViewer(clusters...)
	u.Username = "mai"
	return u
}

func pinFinding(t *testing.T, r http.Handler, caseID string, body map[string]any) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/investigations/"+caseID+"/pin", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestPinFindingChecksScopeAndLinksById(t *testing.T) {
	db := investigationFindingsDB(t)
	require.NoError(t, db.Create(&models.InvestigationCase{ID: "case-1", Title: "t", Status: "OPEN", Owner: "mai", CreatedByUserID: 50, EntitiesJSON: "[]"}).Error)
	r := reviewRouter(scopedCaseOwner("c1"))
	r.POST("/investigations/:id/pin", PinInvestigationEntity(db))

	code, _ := pinFinding(t, r, "case-1", map[string]any{"type": "finding", "label": "anything", "href": "#/risks/2"})
	require.Equal(t, http.StatusNotFound, code, "a finding outside the caller's clusters cannot be linked")

	code, out := pinFinding(t, r, "case-1", map[string]any{"type": "finding", "label": "client label", "href": "#/risks/1"})
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, true, out["pinned"])
	entity := out["entity"].(map[string]any)
	require.Equal(t, "finding:1", entity["id"])
	require.Equal(t, "finding-c1", entity["label"], "the label comes from the finding, not the request")
	require.Equal(t, "1", entity["meta"].(map[string]any)["insightId"])

	code, out = pinFinding(t, r, "case-1", map[string]any{"type": "finding", "label": "other label", "meta": map[string]string{"insightId": "1"}})
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, false, out["pinned"], "the same finding is linked once")
}

func TestPinFindingRejectsAnotherClusterThanTheCase(t *testing.T) {
	db := investigationFindingsDB(t)
	c1 := "c1"
	require.NoError(t, db.Create(&models.InvestigationCase{ID: "case-1", Title: "t", Status: "OPEN", Owner: "admin", ClusterID: &c1, CreatedByUserID: 1, EntitiesJSON: "[]"}).Error)
	r := reviewRouter(adminUser())
	r.POST("/investigations/:id/pin", PinInvestigationEntity(db))

	code, _ := pinFinding(t, r, "case-1", map[string]any{"type": "finding", "label": "x", "href": "#/risks/2"})
	require.Equal(t, http.StatusConflict, code)
}

func TestListInvestigationFindingsIsLiveAndScoped(t *testing.T) {
	db := investigationFindingsDB(t)
	entities := `[{"id":"finding:1","type":"finding","label":"old title","href":"#/risks/1","pinnedAt":"2026-10-06T00:00:00Z"},` +
		`{"id":"finding:2","type":"finding","label":"other cluster","meta":{"insightId":"2"},"pinnedAt":"2026-10-06T00:00:00Z"},` +
		`{"id":"finding:99","type":"finding","label":"gone","href":"#/risks/99","pinnedAt":"2026-10-06T00:00:00Z"},` +
		`{"id":"pod:x","type":"pod","label":"x","pinnedAt":"2026-10-06T00:00:00Z"}]`
	require.NoError(t, db.Create(&models.InvestigationCase{ID: "case-1", Title: "t", Status: "OPEN", Owner: "mai", CreatedByUserID: 50, EntitiesJSON: entities}).Error)
	require.NoError(t, db.Model(&models.Insight{}).Where("id = ?", 1).Update("status", "acknowledged").Error)

	r := reviewRouter(scopedCaseOwner("c1"))
	r.GET("/investigations/:id/findings", ListInvestigationFindings(db))
	body := getJSON(t, r, "/investigations/case-1/findings")
	items := body["items"].([]any)
	require.Len(t, items, 2)
	first := items[0].(map[string]any)
	require.Equal(t, "1", first["insightId"])
	require.Equal(t, "finding-c1", first["title"])
	require.Equal(t, "acknowledged", first["status"], "status is read live")
	require.Equal(t, "high", first["finalLevel"], "risk level comes from the score band")
	require.Equal(t, true, items[1].(map[string]any)["missing"])
	require.Equal(t, float64(1), body["hidden"], "the c2 finding is counted, not returned")
}

func TestListInvestigationCasesFiltersByFinding(t *testing.T) {
	db := investigationFindingsDB(t)
	require.NoError(t, db.Create(&models.InvestigationCase{ID: "case-a", Title: "a", Status: "OPEN", Owner: "admin", CreatedByUserID: 1,
		EntitiesJSON: `[{"id":"finding:1","type":"finding","label":"x","href":"#/risks/1"}]`}).Error)
	require.NoError(t, db.Create(&models.InvestigationCase{ID: "case-b", Title: "b", Status: "OPEN", Owner: "admin", CreatedByUserID: 1,
		EntitiesJSON: `[{"id":"finding:12","type":"finding","label":"x","href":"#/risks/12"}]`}).Error)
	r := reviewRouter(adminUser())
	r.GET("/investigations", ListInvestigationCases(db))

	body := getJSON(t, r, "/investigations?findingId=1")
	items := body["items"].([]any)
	require.Len(t, items, 1)
	require.Equal(t, "case-a", items[0].(map[string]any)["id"])

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/investigations?findingId=1%25", nil))
	require.Equal(t, http.StatusBadRequest, w.Code)
}
