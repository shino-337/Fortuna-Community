package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fortuna/core/pkg/cve/loader"
	"github.com/fortuna/core/pkg/metrics"
	"github.com/fortuna/core/pkg/sbom"
	"gorm.io/gorm"
)

// cveRematchStateName is the mirror_state row holding the last CVE catalog generation whose
// re-match was queued. Replicas claim a generation by advancing it, so only one publishes.
const cveRematchStateName = "cve_rematch_generation"

// malwareRematchStateName is the same marker for malware catalog generations: a change of the
// malware feeds (see malware.AikidoSyncer) re-matches every running SBOM too.
const malwareRematchStateName = "malware_rematch_generation"

// CVERematchInterval reads FORTUNA_CVE_REMATCH_INTERVAL (default 5m; 0 or off disables).
func CVERematchInterval() time.Duration {
	s := strings.TrimSpace(os.Getenv("FORTUNA_CVE_REMATCH_INTERVAL"))
	if s == "" {
		return 5 * time.Minute
	}
	if s == "0" || strings.EqualFold(s, "off") || strings.EqualFold(s, "false") {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		if n, err2 := strconv.Atoi(s); err2 == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
		log.Printf("[CVERematch] invalid FORTUNA_CVE_REMATCH_INTERVAL=%q: %v (disabled)", s, err)
		return 0
	}
	return d
}

// CVECatalogRematcher re-matches every running SBOM once when a new CVE catalog generation
// becomes active (for example after the scheduled vulnerability database update), so findings
// reflect new and withdrawn advisories without waiting for pods to restart.
type CVECatalogRematcher struct {
	DB *gorm.DB
	// Publish sends one sbom.created event; nil runs the matcher in-process instead.
	Publish func(subject string, data []byte) error
	Logger  *log.Logger
	Now     func() time.Time
}

// Run checks every interval until ctx is cancelled.
func (r *CVECatalogRematcher) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := r.CheckOnce(ctx); err != nil {
			r.logf("check failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// CheckOnce updates the catalog freshness gauges and, when this replica claims a newly active
// CVE or malware generation, queues a re-match of every running SBOM. It returns the number of
// SBOMs queued.
func (r *CVECatalogRematcher) CheckOnce(ctx context.Context) (int, error) {
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	gen, err := loader.ActiveCVEGeneration(ctx, r.DB)
	if err != nil {
		return 0, fmt.Errorf("read active CVE generation: %w", err)
	}
	if gen == nil || gen.ActivatedAt == nil {
		metrics.CVECatalogAgeSeconds.Set(-1)
		metrics.CVECatalogGeneration.Set(0)
	} else {
		metrics.CVECatalogAgeSeconds.Set(now.Sub(*gen.ActivatedAt).Seconds())
		metrics.CVECatalogGeneration.Set(float64(gen.ID))
		if n, err := r.rematchGeneration(ctx, cveRematchStateName, "CVE", gen.ID, now); err != nil || n > 0 {
			// One re-match also covers a malware change seen at the same time.
			if err == nil {
				r.markMalwareRematched(ctx, now)
			}
			return n, err
		}
	}
	malwareGen, err := r.activeMalwareGeneration(ctx)
	if err != nil || malwareGen == 0 {
		return 0, err
	}
	return r.rematchGeneration(ctx, malwareRematchStateName, "malware", malwareGen, now)
}

// activeMalwareGeneration returns the newest active malware catalog generation (0 when none).
func (r *CVECatalogRematcher) activeMalwareGeneration(ctx context.Context) (uint, error) {
	if !r.DB.Migrator().HasTable("catalog_generations") {
		return 0, nil
	}
	var id uint
	if err := r.DB.WithContext(ctx).Raw(
		`SELECT COALESCE(MAX(id), 0) FROM catalog_generations WHERE catalog_type = 'malware' AND status = 'active'`,
	).Scan(&id).Error; err != nil {
		return 0, fmt.Errorf("read active malware generation: %w", err)
	}
	return id, nil
}

// markMalwareRematched advances the malware marker to the newest malware generation without
// queueing, after a CVE re-match that already used it.
func (r *CVECatalogRematcher) markMalwareRematched(ctx context.Context, now time.Time) {
	if gen, err := r.activeMalwareGeneration(ctx); err == nil && gen > 0 {
		if _, err := r.claim(ctx, malwareRematchStateName, gen, now); err != nil {
			r.logf("advance %s: %v", malwareRematchStateName, err)
		}
	}
}

// rematchGeneration claims generation id under marker state and, when this replica claimed it,
// queues a re-match of every running SBOM.
func (r *CVECatalogRematcher) rematchGeneration(ctx context.Context, state, catalog string, id uint, now time.Time) (int, error) {
	claimed, err := r.claim(ctx, state, id, now)
	if err != nil || !claimed {
		return 0, err
	}
	// Matcher runs keyed by the "osv" mirror_state version see this bump.
	if err := r.DB.WithContext(ctx).Exec(`INSERT INTO mirror_state (name, version, updated_at) VALUES ('osv', 1, ?)
ON CONFLICT (name) DO UPDATE SET version = mirror_state.version + 1, updated_at = excluded.updated_at`, now).Error; err != nil {
		r.logf("bump osv mirror version: %v", err)
	}
	n, err := r.queueRematch(ctx, id, now)
	if err != nil {
		// Release the claim so the next check retries; re-matching an SBOM twice for the same
		// generation is a no-op (match runs are keyed by generation).
		if rerr := r.DB.WithContext(ctx).Exec(`UPDATE mirror_state SET version = ? WHERE name = ? AND version = ?`,
			int64(id)-1, state, int64(id)).Error; rerr != nil {
			r.logf("release claim for generation %d: %v", id, rerr)
		}
		return n, err
	}
	r.logf("%s catalog generation %d is active; queued re-match of %d SBOMs", catalog, id, n)
	return n, nil
}

// claim advances the rematch marker state to generation id; true when this call advanced it.
func (r *CVECatalogRematcher) claim(ctx context.Context, state string, id uint, now time.Time) (bool, error) {
	res := r.DB.WithContext(ctx).Exec(`INSERT INTO mirror_state (name, version, updated_at) VALUES (?, ?, ?)
ON CONFLICT (name) DO UPDATE SET version = excluded.version, updated_at = excluded.updated_at
WHERE mirror_state.version < excluded.version`, state, int64(id), now)
	if res.Error != nil {
		return false, fmt.Errorf("claim CVE rematch: %w", res.Error)
	}
	return res.RowsAffected == 1, nil
}

type rematchSBOMRow struct {
	ID            uint
	ClusterID     string
	PodUID        string
	PodName       string
	Namespace     string
	ContainerName string
	ImageName     string
	ImageTag      string
	ImageDigest   string
}

func (r *CVECatalogRematcher) queueRematch(ctx context.Context, generation uint, now time.Time) (int, error) {
	var rows []rematchSBOMRow
	// SBOMs of pods that still exist; the same selection as scripts/utils/load-cve-data.sh.
	if err := r.DB.WithContext(ctx).Raw(`
SELECT s.id, s.cluster_id, s.pod_uid, s.pod_name, s.namespace, s.container_name, s.image_name, s.image_tag, s.image_digest
FROM sboms s
INNER JOIN pods p ON p.cluster_id = s.cluster_id AND p.uid = s.pod_uid AND p.deleted_at IS NULL
WHERE s.deleted_at IS NULL
  AND lower(coalesce(s.status, '')) IN ('complete', 'partial', 'finalized')
ORDER BY s.id`).Scan(&rows).Error; err != nil {
		return 0, fmt.Errorf("list SBOMs to re-match: %w", err)
	}
	var inProcess *CVEMatcherWorker
	if r.Publish == nil {
		inProcess = NewCVEMatcherWorker(nil, r.DB, nil)
	}
	queued := 0
	for _, row := range rows {
		ev := sbom.SBOMCreatedEvent{
			Type:           "sbom.created",
			Timestamp:      now.Unix(),
			EventID:        fmt.Sprintf("cve-catalog-rematch-%d-%d", row.ID, generation),
			SchemaVersion:  sbom.SBOMCreatedEventSchemaVersion,
			CorrelationID:  fmt.Sprintf("cve-catalog-rematch-%d", generation),
			ClusterID:      row.ClusterID,
			PodUID:         row.PodUID,
			PodName:        row.PodName,
			PodNamespace:   row.Namespace,
			ContainerName:  row.ContainerName,
			ContainerImage: row.ImageName + ":" + row.ImageTag,
			SBOMID:         row.ID,
			ImageDigest:    row.ImageDigest,
		}
		if inProcess != nil {
			if err := inProcess.ProcessSBOMCreatedEvent(ctx, ev); err != nil {
				r.logf("re-match sbom_id=%d: %v", row.ID, err)
				continue
			}
		} else {
			data, err := json.Marshal(ev)
			if err != nil {
				return queued, err
			}
			if err := r.Publish("fortuna.sbom.created", data); err != nil {
				return queued, fmt.Errorf("publish re-match for sbom_id=%d: %w", row.ID, err)
			}
		}
		queued++
		metrics.CVECatalogRematchEventsTotal.Inc()
	}
	return queued, nil
}

func (r *CVECatalogRematcher) logf(format string, args ...interface{}) {
	if r.Logger != nil {
		r.Logger.Printf("[CVERematch] "+format, args...)
		return
	}
	log.Printf("[CVERematch] "+format, args...)
}
