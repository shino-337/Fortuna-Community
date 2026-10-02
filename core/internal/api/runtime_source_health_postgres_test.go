package api

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fortuna/api/collection"
	"github.com/fortuna/core/migrations"
	"github.com/fortuna/core/pkg/models"
	"github.com/stretchr/testify/require"
)

func TestSourceHealthPostgresConcurrencyAndRollback(t *testing.T) {
	db := sbomListPostgresDB(t)
	require.NoError(t, migrations.EnsureRuntimeCoverage(db))
	require.NoError(t, migrations.EnsureRuntimeCoverage(db))
	f := newHealthFixture(t)
	seedRuntimeProducer(t, db, &f.principal, "falco", "falco", runtimeTestSession, f.now.Add(-30*time.Second), f.now, true, false, collection.RuntimeProducerStarting)
	h := f.report(f.now.Add(-4*time.Second), f.now.Add(-3*time.Second))
	signed := f.sign(t, h)
	var wg sync.WaitGroup
	codes := make(chan int, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); codes <- postHealth(db, f, signed).Code }()
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		require.Equal(t, 200, code)
	}
	var count int64
	require.NoError(t, db.Model(&models.RuntimeSourceHealthReceipt{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	changed := h
	changed.Status = "failed"
	changed.Reason = "altered"
	require.Equal(t, 409, postHealth(db, f, f.sign(t, changed)).Code)
	state := healthState(t, db)
	require.True(t, state.Authoritative)
	// A real PostgreSQL failure after receipt insertion must roll back evidence and authority.
	require.NoError(t, db.Exec(`CREATE FUNCTION reject_health_update() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected producer update failure'; END $$`).Error)
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_health_update BEFORE UPDATE ON runtime_producer_states FOR EACH ROW EXECUTE FUNCTION reject_health_update()`).Error)
	failed := h
	failed.Sequence = 2
	failed.WindowStart = h.WindowEnd
	failed.WindowEnd = f.now.Add(-2 * time.Second)
	failed.Status = "failed"
	failed.Reason = "sensor failure"
	w := postHealth(db, f, f.sign(t, failed))
	require.Equal(t, 503, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"retryable":true`)
	require.NoError(t, db.Model(&models.RuntimeSourceHealthReceipt{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.True(t, healthState(t, db).Authoritative)
	require.NoError(t, db.Exec(`DROP TRIGGER reject_health_update ON runtime_producer_states`).Error)
	require.Equal(t, 200, postHealth(db, f, f.sign(t, failed)).Code)
	require.False(t, healthState(t, db).Authoritative)
	// Foreign authenticated ownership cannot reuse the same signed principal.
	foreign := f
	foreign.principal.ClusterID = "cluster-b"
	require.Equal(t, 403, postHealth(db, foreign, signed).Code)
	require.NoError(t, db.Model(&models.RuntimeSourceHealthReceipt{}).Count(&count).Error)
	require.EqualValues(t, 2, count, fmt.Sprint(count))
}
