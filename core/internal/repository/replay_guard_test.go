package repository

import (
	"context"
	"sync"
	"testing"

	"github.com/fortuna/core/migrations"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newReplayGuardRepo(t *testing.T) (*SBOMRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, migrations.Migration086_AddSBOMProcessingState(db))
	return NewSBOMRepository(db), db
}

func TestReplayGuard_OldEventSkipped(t *testing.T) {
	repo, _ := newReplayGuardRepo(t)
	ctx := context.Background()

	ok, err := repo.ClaimSBOMEvent(ctx, 1, "B", 20)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = repo.ClaimSBOMEvent(ctx, 1, "A", 10)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestReplayGuard_NewEventOverrides(t *testing.T) {
	repo, _ := newReplayGuardRepo(t)
	ctx := context.Background()

	ok, err := repo.ClaimSBOMEvent(ctx, 1, "A", 10)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = repo.ClaimSBOMEvent(ctx, 1, "B", 20)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestReplayGuard_ConcurrentEvents(t *testing.T) {
	repo, _ := newReplayGuardRepo(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	results := make(chan bool, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		ok, err := repo.ClaimSBOMEvent(ctx, 1, "A", 10)
		require.NoError(t, err)
		results <- ok
	}()
	go func() {
		defer wg.Done()
		ok, err := repo.ClaimSBOMEvent(ctx, 1, "B", 20)
		require.NoError(t, err)
		results <- ok
	}()
	wg.Wait()
	close(results)

	success := 0
	for r := range results {
		if r {
			success++
		}
	}
	// Both inserts/updates can return true depending on race order, but the end state must be "latest wins".
	require.GreaterOrEqual(t, success, 1)

	// Now an old event must be rejected deterministically.
	ok, err := repo.ClaimSBOMEvent(ctx, 1, "C", 15)
	require.NoError(t, err)
	require.False(t, ok)

	ok, err = repo.ClaimSBOMEvent(ctx, 1, "D", 25)
	require.NoError(t, err)
	require.True(t, ok)
}

