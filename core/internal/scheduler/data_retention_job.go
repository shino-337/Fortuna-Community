package scheduler

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// retentionBatchSize bounds each DELETE so a first run on a large table does not
// hold one long lock.
const retentionBatchSize = 5000

// RetentionRule hard-deletes rows of one table once its age column is older
// than the window. Days <= 0 disables the rule.
type RetentionRule struct {
	Name   string // log label
	Table  string
	AgeSQL string // SQL expression giving the row's age timestamp
	EnvVar string // FORTUNA_RETENTION_*_DAYS override
	Days   int
	// Deadline marks AgeSQL as an expiry time rather than an age: rows go once it
	// passes, and Days only switches the rule on (> 0) or off.
	Deadline bool
}

// DefaultRetentionRules lists the tables that otherwise grow forever. Append-only
// security_activity_logs and investigation_activity_logs are deliberately absent:
// the database refuses deletes on them.
func DefaultRetentionRules() []RetentionRule {
	return []RetentionRule{
		{Name: "runtime events", Table: "runtime_events", AgeSQL: "COALESCE(observed_at, created_at)", EnvVar: "FORTUNA_RETENTION_RUNTIME_DAYS", Days: 30},
		{Name: "runtime facts", Table: "runtime_behavior_facts", AgeSQL: "observed_at", EnvVar: "FORTUNA_RETENTION_RUNTIME_DAYS", Days: 30},
		{Name: "runtime signals", Table: "runtime_signals", AgeSQL: "COALESCE(last_seen_at, created_at)", EnvVar: "FORTUNA_RETENTION_RUNTIME_DAYS", Days: 30},
		{Name: "runtime incidents", Table: "runtime_incidents", AgeSQL: "last_seen_at", EnvVar: "FORTUNA_RETENTION_RUNTIME_DAYS", Days: 30},
		{Name: "Kubernetes events", Table: "k8s_events", AgeSQL: "COALESCE(last_timestamp, created_at)", EnvVar: "FORTUNA_RETENTION_K8S_EVENTS_DAYS", Days: 14},
		{Name: "pod runtime metrics", Table: "pod_runtime_metrics", AgeSQL: "last_observed_at", EnvVar: "FORTUNA_RETENTION_POD_METRICS_DAYS", Days: 30},
		{Name: "expired sessions", Table: "user_sessions", AgeSQL: "expires_at", EnvVar: "FORTUNA_RETENTION_SESSIONS_DAYS", Days: 30},
		{Name: "audit logs", Table: "audit_logs", AgeSQL: "created_at", EnvVar: "FORTUNA_RETENTION_AUDIT_LOG_DAYS", Days: 90},
		// Archived cases carry their own deadline (retention_until, set on archive).
		// Set FORTUNA_RETENTION_CASES_DAYS=0 to keep them regardless.
		{Name: "archived investigation cases", Table: "investigation_cases", AgeSQL: "CASE WHEN deleted_at IS NOT NULL THEN retention_until END", EnvVar: "FORTUNA_RETENTION_CASES_DAYS", Days: 1, Deadline: true},
	}
}

// DataRetentionJob applies RetentionRules on an interval (FORTUNA_RETENTION_INTERVAL,
// default 6h) and scrubs password hashes of deleted users.
type DataRetentionJob struct {
	db       *gorm.DB
	rules    []RetentionRule
	interval time.Duration
	now      func() time.Time
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewDataRetentionJob reads per-rule overrides from the environment.
func NewDataRetentionJob(db *gorm.DB) *DataRetentionJob {
	rules := DefaultRetentionRules()
	for i := range rules {
		rules[i] = applyRetentionEnv(rules[i])
	}
	interval := 6 * time.Hour
	if v := os.Getenv("FORTUNA_RETENTION_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			interval = d
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &DataRetentionJob{db: db, rules: rules, interval: interval, now: time.Now, ctx: ctx, cancel: cancel}
}

func applyRetentionEnv(r RetentionRule) RetentionRule {
	v := strings.TrimSpace(os.Getenv(r.EnvVar))
	if v == "" {
		return r
	}
	if n, err := strconv.Atoi(v); err == nil && n >= 0 {
		r.Days = n
	}
	return r
}

// Start runs the job until Stop. Call in a goroutine.
func (j *DataRetentionJob) Start() {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()
	log.Printf("[DataRetentionJob] Started - interval=%s", j.interval)
	j.RunOnce()
	for {
		select {
		case <-j.ctx.Done():
			log.Printf("[DataRetentionJob] Stopped")
			return
		case <-ticker.C:
			j.RunOnce()
		}
	}
}

// Stop stops the job.
func (j *DataRetentionJob) Stop() { j.cancel() }

// RunOnce applies every enabled rule once and returns rows deleted per table.
func (j *DataRetentionJob) RunOnce() map[string]int64 {
	deleted := map[string]int64{}
	now := j.now().UTC()
	db := j.db.WithContext(j.ctx)
	for _, r := range j.rules {
		if r.Days <= 0 || !db.Migrator().HasTable(r.Table) {
			continue
		}
		cutoff := now.Add(-time.Duration(r.Days) * 24 * time.Hour)
		if r.Deadline {
			cutoff = now
		}
		n, err := deleteOlderThan(db, r, cutoff)
		if err != nil {
			log.Printf("[DataRetentionJob] %s: %v", r.Name, err)
			continue
		}
		if n > 0 {
			deleted[r.Table] = n
			log.Printf("[DataRetentionJob] Deleted %d %s (older than %s)", n, r.Name, cutoff.Format(time.RFC3339))
		}
	}
	if n, err := scrubDeletedUserPasswords(db); err != nil {
		log.Printf("[DataRetentionJob] deleted users: %v", err)
	} else if n > 0 {
		deleted["users.password"] = n
		log.Printf("[DataRetentionJob] Cleared password hashes of %d deleted users", n)
	}
	return deleted
}

func deleteOlderThan(db *gorm.DB, r RetentionRule, cutoff time.Time) (int64, error) {
	// Table and age expression come from the fixed rule list, never from input.
	q := "DELETE FROM " + r.Table + " WHERE id IN (SELECT id FROM " + r.Table +
		" WHERE " + r.AgeSQL + " < ? LIMIT ?)"
	var total int64
	for {
		res := db.Exec(q, cutoff, retentionBatchSize)
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
		if res.RowsAffected < retentionBatchSize {
			return total, nil
		}
	}
}

// scrubDeletedUserPasswords clears the hash of soft-deleted accounts. The row stays
// so audit entries keep resolving the user id; the hash has no further use.
func scrubDeletedUserPasswords(db *gorm.DB) (int64, error) {
	if !db.Migrator().HasTable("users") || !db.Migrator().HasColumn("users", "deleted_at") {
		return 0, nil
	}
	res := db.Exec("UPDATE users SET password = '' WHERE deleted_at IS NOT NULL AND password <> ''")
	return res.RowsAffected, res.Error
}
