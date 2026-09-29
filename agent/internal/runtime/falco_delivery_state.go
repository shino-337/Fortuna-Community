package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fortuna/agent/internal/corehttp"
)

const falcoStateMaxBytes = 16 << 20
const falcoStateMaxEvents = 2048
const falcoQuarantineRetry = 5 * time.Minute

type falcoOutbox struct {
	Version           int       `json:"version"`
	ClusterID         string    `json:"clusterId"`
	AgentID           string    `json:"agentId"`
	CoreURL           string    `json:"coreUrl"`
	SourcePath        string    `json:"sourcePath"`
	SourceFile        string    `json:"sourceFile"`
	Offset            int64     `json:"offset"`
	Pending           []Event   `json:"pending"`
	Quarantine        []Event   `json:"quarantine"`
	RetryAt           time.Time `json:"retryAt"`
	QuarantineRetryAt time.Time `json:"quarantineRetryAt"`
}

type falcoDeliveryState struct {
	path       string
	expected   falcoOutbox
	state      falcoOutbox
	loaded     bool
	checkpoint bool
	lock       *os.File
}

// SetDeliveryState configures a node-local durable outbox. Without it the reader
// keeps its existing fail-closed whole-slice retry; it never discards stale UIDs.
func (r *FalcoReader) SetDeliveryState(path, clusterID, agentID string) {
	if path == "" {
		return
	}
	r.delivery = &falcoDeliveryState{path: path, expected: falcoOutbox{Version: 1, ClusterID: clusterID, AgentID: agentID, CoreURL: r.coreURL, SourcePath: filepath.Clean(r.path)}}
}

func (r *FalcoReader) CloseDeliveryState() {
	if r.delivery != nil && r.delivery.lock != nil {
		_ = r.delivery.lock.Close()
		r.delivery.lock = nil
		r.delivery.loaded = false
	}
}

func (s *falcoDeliveryState) load() error {
	if s.loaded {
		return nil
	}
	if s.expected.ClusterID == "" || s.expected.AgentID == "" || !filepath.IsAbs(s.path) {
		return fmt.Errorf("Falco delivery state requires an absolute path and cluster/agent identity")
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0750); err != nil {
		return err
	}
	if s.lock == nil {
		fd, err := syscall.Open(s.path+".lock", syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
		if err != nil {
			return err
		}
		lock := os.NewFile(uintptr(fd), s.path+".lock")
		if err := syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			_ = lock.Close()
			return fmt.Errorf("Falco delivery state already in use: %w", err)
		}
		s.lock = lock
	}
	st, err := os.Lstat(s.path)
	if os.IsNotExist(err) {
		s.state = s.expected
		s.loaded = true
		return nil
	}
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() || st.Size() > falcoStateMaxBytes {
		return fmt.Errorf("invalid Falco state file type/size")
	}
	f, err := os.Open(s.path)
	if err != nil {
		return err
	}
	defer f.Close()
	var state falcoOutbox
	decoder := json.NewDecoder(io.LimitReader(f, falcoStateMaxBytes+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("trailing Falco state data")
	}
	if err := s.validate(state); err != nil {
		return err
	}
	s.state = state
	s.loaded = true
	s.checkpoint = true
	return nil
}

func (s *falcoDeliveryState) validate(state falcoOutbox) error {
	want := s.expected
	if state.Version != want.Version || state.ClusterID != want.ClusterID || state.AgentID != want.AgentID || state.CoreURL != want.CoreURL || state.SourcePath != want.SourcePath || state.Offset < 0 {
		return fmt.Errorf("Falco state version/source/principal binding mismatch")
	}
	if state.Offset > 0 && state.SourceFile == "" {
		return fmt.Errorf("Falco cursor has no source-file identity")
	}
	if len(state.Pending)+len(state.Quarantine) > falcoStateMaxEvents {
		return fmt.Errorf("Falco outbox capacity exceeded; evidence retained, ingestion backpressured")
	}
	seen := map[string]bool{}
	for _, events := range [][]Event{state.Pending, state.Quarantine} {
		for _, event := range events {
			id, err := hex.DecodeString(event.SourceRecordID)
			uid, _ := event.Pod["uid"].(string)
			payload, _ := json.Marshal(event.PayloadJSON)
			hash := sha256.Sum256(payload)
			if err != nil || len(id) != 32 || seen[event.SourceRecordID] || uid == "" || event.IngestedAt == "" || event.PayloadJSON == nil || event.PayloadHash != hex.EncodeToString(hash[:]) {
				return fmt.Errorf("invalid or duplicate persisted Falco event")
			}
			seen[event.SourceRecordID] = true
		}
	}
	return nil
}

// Commit the cursor and complete canonical payloads together before delivery.
// Rename/fsync failure never advances in-memory state; a crash can replay an
// accepted event, which Core deduplicates by the preserved physical record ID.
func (s *falcoDeliveryState) save(next falcoOutbox) error {
	if err := s.validate(next); err != nil {
		return err
	}
	data, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if len(data) > falcoStateMaxBytes {
		return fmt.Errorf("Falco outbox byte capacity exceeded")
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".falco-outbox-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	defer f.Close()
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp, s.path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(s.path))
	if err != nil {
		return err
	}
	defer dir.Close()
	if err = dir.Sync(); err != nil {
		return err
	}
	s.state = next
	s.checkpoint = true
	return nil
}

func falcoSourceFile(st os.FileInfo) string {
	if stat, ok := st.Sys().(*syscall.Stat_t); ok {
		return fmt.Sprintf("%d:%d", uint64(stat.Dev), uint64(stat.Ino))
	}
	return st.Name()
}

func (r *FalcoReader) checkpointDelivery(st os.FileInfo, offset int64, events []Event) error {
	if r.delivery == nil {
		return nil
	}
	next := r.delivery.state
	next.SourceFile = falcoSourceFile(st)
	next.Offset = offset
	next.Pending = events
	return r.delivery.save(next)
}

func (r *FalcoReader) flushDelivery(ctx context.Context, budget *corehttp.DeliveryBudget, stats *CoverageStats, quarantine bool) error {
	s := r.delivery
	if s == nil || !s.loaded {
		return nil
	}
	if budget.Now.Before(s.state.RetryAt) {
		budget.Blocked = true
		return nil
	}
	if budget.Blocked || budget.Remaining <= 0 || ctx.Err() != nil {
		return ctx.Err()
	}
	if len(s.state.Pending) == 0 && (!quarantine || len(s.state.Quarantine) == 0 || budget.Now.Before(s.state.QuarantineRetryAt)) {
		return nil
	}
	next := s.state
	var lastErr error
	if len(next.Pending) > 0 {
		stats.Emitted += uint64(len(next.Pending))
		result := corehttp.DeliverIsolated(ctx, next.Pending, budget, "runtime_ownership_mismatch", r.sendContext)
		stats.Delivered += uint64(len(result.Delivered))
		wasEmpty := len(next.Quarantine) == 0
		next.Pending = result.Retry
		next.Quarantine = append(append([]Event(nil), next.Quarantine...), result.Rejected...)
		if len(result.Rejected) > 0 && wasEmpty {
			next.QuarantineRetryAt = time.Now().Add(falcoQuarantineRetry)
		}
		lastErr = result.Err
		atomic.AddUint64(&r.sentEvents, uint64(len(result.Delivered)))
		atomic.AddUint64(&r.failedEvents, uint64(len(result.Retry)+len(result.Rejected)))
		if len(result.Delivered) > 0 {
			atomic.AddUint64(&r.sentBatches, 1)
		}
	}
	if quarantine && !budget.Blocked && budget.Remaining > 0 && len(next.Quarantine) > 0 && !budget.Now.Before(next.QuarantineRetryAt) {
		stats.Emitted += uint64(len(next.Quarantine))
		result := corehttp.DeliverIsolated(ctx, next.Quarantine, budget, "runtime_ownership_mismatch", r.sendContext)
		stats.Delivered += uint64(len(result.Delivered))
		next.Quarantine = append(result.Retry, result.Rejected...)
		next.QuarantineRetryAt = time.Now().Add(falcoQuarantineRetry)
		atomic.AddUint64(&r.sentEvents, uint64(len(result.Delivered)))
		atomic.AddUint64(&r.failedEvents, uint64(len(result.Retry)+len(result.Rejected)))
		if len(result.Delivered) > 0 {
			atomic.AddUint64(&r.sentBatches, 1)
		}
		lastErr = result.Err
	}
	next.RetryAt = budget.RetryAt
	if err := s.save(next); err != nil {
		return err
	}
	if lastErr != nil {
		atomic.AddUint64(&r.failedBatches, 1)
		r.logger.Printf("Falco ingest retained for retry: %v (retry_at=%s)", lastErr, next.RetryAt)
	}
	return lastErr
}
