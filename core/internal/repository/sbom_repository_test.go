package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/fortuna/core/internal/contextkeys"
	"github.com/fortuna/core/pkg/models"
)

func newTestRepo(t *testing.T) (*SBOMRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	// SQLite :memory: is per-connection; limit to 1 so concurrent goroutines share the same DB.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.SBOM{}, &models.SBOMComponent{}, &models.SBOMMatchRun{}))
	return NewSBOMRepository(db), db
}

// createSBOM creates an SBOM with the given status via the repo (with mutation flag so create/update succeeds).
// Returns the created SBOM for use in tests. For "finalized" the repo sets status on create.
func createSBOM(t *testing.T, repo *SBOMRepository, status string) *models.SBOM {
	t.Helper()
	ctx := contextkeys.WithSBOMMutationAllowed(context.Background())
	podUID := "test-pod-uid-" + status
	st := status
	if st == "" {
		st = "pending"
	}
	sbom := &models.SBOM{
		PodUID:        podUID,
		ImageName:     "test/image",
		ImageTag:      "latest",
		ImageDigest:   "sha256:abc",
		PodName:       "test-pod",
		Namespace:     "default",
		ContainerName: "main",
		PackageCount:  0,
		LastUsedAt:    time.Now(),
		UseCount:      1,
		Status:        st,
		Version:       1,
	}
	persisted, _, err := repo.UpsertSBOMWithComponents(ctx, sbom, nil)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	return persisted
}

// --- A1–A4: WRITE GUARD (SBOM IMMUTABILITY) ---

// TestSBOMRepository_BlockMutation_WhenFinalized verifies that updating an existing finalized SBOM
// without the context flag returns an error containing "immutable" or "finalized".
func TestSBOMRepository_BlockMutation_WhenFinalized(t *testing.T) {
	repo, _ := newTestRepo(t)
	sbom := createSBOM(t, repo, "finalized")

	ctx := context.Background() // no mutation flag
	updateSBOM := &models.SBOM{
		PodUID:        sbom.PodUID,
		ImageName:     "hacked",
		ImageTag:      sbom.ImageTag,
		ImageDigest:   sbom.ImageDigest,
		PodName:       sbom.PodName,
		Namespace:     sbom.Namespace,
		ContainerName: sbom.ContainerName,
		PackageCount:  0,
		LastUsedAt:    time.Now(),
		UseCount:      sbom.UseCount + 1,
		Status:        "finalized",
		Version:       sbom.Version,
	}
	_, _, err := repo.UpsertSBOMWithComponents(ctx, updateSBOM, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "immutable")
}

// TestSBOMRepository_AllowMutation_WithContextFlag verifies that updating an existing finalized SBOM
// with the mutation-allowed context succeeds.
func TestSBOMRepository_AllowMutation_WithContextFlag(t *testing.T) {
	repo, _ := newTestRepo(t)
	sbom := createSBOM(t, repo, "finalized")

	ctx := contextkeys.WithSBOMMutationAllowed(context.Background())
	updateSBOM := &models.SBOM{
		PodUID:        sbom.PodUID,
		ImageName:     "legit-update",
		ImageTag:      sbom.ImageTag,
		ImageDigest:   sbom.ImageDigest,
		PodName:       sbom.PodName,
		Namespace:     sbom.Namespace,
		ContainerName: sbom.ContainerName,
		PackageCount:  0,
		LastUsedAt:    time.Now(),
		UseCount:      sbom.UseCount + 1,
		Status:        "finalized",
		Version:       sbom.Version,
	}
	persisted, _, err := repo.UpsertSBOMWithComponents(ctx, updateSBOM, nil)
	require.NoError(t, err)
	require.NotNil(t, persisted)
	require.Equal(t, "legit-update", persisted.ImageName)
}

// --- B1–B3: IDEMPOTENT MATCH RUN ---

const (
	testMatchRunSBOMID   = uint(1)
	testMatchRunVersion  = 1
	testMirrorVersion    = "ts-hour-1"
)

// TestMatchRun_FirstInsert verifies the first EnsureMatchRun for a key creates the run and returns true.
func TestMatchRun_FirstInsert(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()

	ok, err := repo.EnsureMatchRun(ctx, testMatchRunSBOMID, testMatchRunVersion, testMirrorVersion)
	require.NoError(t, err)
	require.True(t, ok)
}

// TestMatchRun_DuplicateInsert_ShouldSkip verifies the second EnsureMatchRun for the same key returns false (skip).
func TestMatchRun_DuplicateInsert_ShouldSkip(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()

	_, _ = repo.EnsureMatchRun(ctx, testMatchRunSBOMID, testMatchRunVersion, testMirrorVersion)

	ok, err := repo.EnsureMatchRun(ctx, testMatchRunSBOMID, testMatchRunVersion, testMirrorVersion)
	require.NoError(t, err)
	require.False(t, ok)
}

// TestMatchRun_ConcurrentInsert verifies that under concurrent calls only one goroutine gets true (idempotency).
func TestMatchRun_ConcurrentInsert(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	results := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := repo.EnsureMatchRun(ctx, testMatchRunSBOMID, testMatchRunVersion, testMirrorVersion)
			require.NoError(t, err)
			results <- ok
		}()
	}

	wg.Wait()
	close(results)

	successCount := 0
	for r := range results {
		if r {
			successCount++
		}
	}
	require.Equal(t, 1, successCount, "exactly one run should succeed")
}

// TestMatchRun_DuplicateKey_ShouldNotError verifies that when a run already exists (e.g. inserted by another process),
// EnsureMatchRun returns false and no error (idempotent skip).
func TestMatchRun_DuplicateKey_ShouldNotError(t *testing.T) {
	repo, db := newTestRepo(t)
	ctx := context.Background()

	// Pre-insert the run so the next EnsureMatchRun finds it and skips.
	require.NoError(t, db.Create(&models.SBOMMatchRun{
		SBOMID:        testMatchRunSBOMID,
		Version:       testMatchRunVersion,
		MirrorVersion: testMirrorVersion,
		Status:        "running",
		CreatedAt:     time.Now(),
	}).Error)

	ok, err := repo.EnsureMatchRun(ctx, testMatchRunSBOMID, testMatchRunVersion, testMirrorVersion)
	require.NoError(t, err)
	require.False(t, ok)
}

// createRun inserts a match run with the given status and created_at (for stale tests).
func createRun(t *testing.T, db *gorm.DB, sbomID uint, version int, mirrorVersion string, status string, createdAt time.Time) {
	t.Helper()
	require.NoError(t, db.Create(&models.SBOMMatchRun{
		SBOMID:        sbomID,
		Version:       version,
		MirrorVersion: mirrorVersion,
		Status:        status,
		CreatedAt:     createdAt,
	}).Error)
}

// TestMatchRun_StaleReclaim verifies that an old "running" run (older than staleAfter) is reclaimed and EnsureMatchRun returns true.
func TestMatchRun_StaleReclaim(t *testing.T) {
	repo, db := newTestRepo(t)
	ctx := context.Background()

	createRun(t, db, testMatchRunSBOMID, testMatchRunVersion, testMirrorVersion, "running", time.Now().Add(-20*time.Minute))

	ok, err := repo.EnsureMatchRun(ctx, testMatchRunSBOMID, testMatchRunVersion, testMirrorVersion)
	require.NoError(t, err)
	require.True(t, ok)
}

// TestMatchRun_NotStale_ShouldSkip verifies that a recent "running" run is not reclaimed; EnsureMatchRun returns false.
func TestMatchRun_NotStale_ShouldSkip(t *testing.T) {
	repo, db := newTestRepo(t)
	ctx := context.Background()

	createRun(t, db, testMatchRunSBOMID, testMatchRunVersion, testMirrorVersion, "running", time.Now())

	ok, err := repo.EnsureMatchRun(ctx, testMatchRunSBOMID, testMatchRunVersion, testMirrorVersion)
	require.NoError(t, err)
	require.False(t, ok)
}
