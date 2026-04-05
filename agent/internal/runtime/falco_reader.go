package runtime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/fortuna/agent/internal/corehttp"
)

// Falco JSON event format (minimal subset).
// Typical keys: time, rule, priority, output, tags, output_fields.
type falcoEvent struct {
	Time         interface{}            `json:"time"`
	Rule         string                 `json:"rule"`
	Priority     string                 `json:"priority"`
	Output       string                 `json:"output"`
	Tags         []string               `json:"tags"`
	OutputFields map[string]interface{} `json:"output_fields"`
}

type FalcoReader struct {
	path       string
	poll       time.Duration
	coreURL    string
	nodeName   string
	kubeClient kubernetes.Interface
	httpClient *http.Client
	logger     *log.Logger
	offset     int64
	// lineBuf holds an incomplete trailing line (no '\n' yet) across polls so we never
	// json.Unmarshal a half-written Falco record (causes "invalid character ...", EOF, etc.).
	lineBuf []byte

	podUIDCache   map[string]podUIDCacheEntry
	podUIDCacheMu sync.Mutex

	// ingestion quality counters
	invalidLines  uint64
	sentBatches   uint64
	sentEvents    uint64
	failedBatches uint64
	failedEvents  uint64
	v2Success     uint64
	v1Fallback    uint64
}

type podUIDCacheEntry struct {
	uid       string
	expiresAt time.Time
}

func NewFalcoReader(path string, poll time.Duration, coreURL string, nodeName string, kubeClient kubernetes.Interface) *FalcoReader {
	if poll <= 0 {
		poll = 5 * time.Second
	}
	return &FalcoReader{
		path:        path,
		poll:        poll,
		coreURL:     strings.TrimRight(coreURL, "/"),
		nodeName:    strings.TrimSpace(nodeName),
		kubeClient:  kubeClient,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
		logger:      log.New(log.Writer(), "[FalcoEvents] ", log.LstdFlags),
		podUIDCache: map[string]podUIDCacheEntry{},
	}
}

func (r *FalcoReader) Start(ctx context.Context) {
	r.readAndSend(ctx)
	ticker := time.NewTicker(r.poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.readAndSend(ctx)
		}
	}
}

func (r *FalcoReader) readAndSend(ctx context.Context) {
	f, err := os.Open(r.path)
	if err != nil {
		return
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return
	}
	fileSize := st.Size()
	// Log rotation / truncate: start over
	if r.offset > fileSize {
		r.offset = 0
		r.lineBuf = nil
	}
	// First run: tail from EOF to avoid loading historical multi-GB Falco backlog
	// into memory (can OOM the agent). New alerts after startup are still captured.
	if r.offset == 0 && fileSize > 0 {
		r.offset = fileSize
		return
	}

	startOffset := r.offset
	if _, err := f.Seek(startOffset, io.SeekStart); err != nil {
		r.logger.Printf("Failed to seek falco events file: %v", err)
		return
	}

	chunk, err := io.ReadAll(f)
	if err != nil {
		r.logger.Printf("Failed to read falco events file: %v", err)
		return
	}

	data := append(r.lineBuf, chunk...)
	r.lineBuf = nil

	events := make([]Event, 0, 20)
	for len(data) > 0 {
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			r.lineBuf = append([]byte{}, data...)
			break
		}
		line := bytes.TrimSpace(data[:idx])
		data = data[idx+1:]
		if len(line) == 0 {
			continue
		}
		fes, err := parseFalcoJSONLines(line)
		if err != nil {
			atomic.AddUint64(&r.invalidLines, 1)
			r.logger.Printf("Invalid falco JSON: %v", err)
			continue
		}
		for i := range fes {
			ev, ok := r.toRuntimeEvent(ctx, &fes[i])
			if !ok {
				continue
			}
			events = append(events, ev)
		}
	}

	// Next read starts after all bytes we consumed from the file this poll (not after partial line).
	r.offset = startOffset + int64(len(chunk))

	if len(events) == 0 {
		return
	}
	if err := r.send(events); err != nil {
		atomic.AddUint64(&r.failedBatches, 1)
		atomic.AddUint64(&r.failedEvents, uint64(len(events)))
		r.logger.Printf("Failed to send falco events: %v", err)
		r.logIngestionStats("send_failed")
		return
	}
	atomic.AddUint64(&r.sentBatches, 1)
	atomic.AddUint64(&r.sentEvents, uint64(len(events)))
	r.logIngestionStats("send_ok")
}

// parseFalcoJSONLines decodes one or more JSON objects from a single line (rare but possible).
func parseFalcoJSONLines(line []byte) ([]falcoEvent, error) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return nil, nil
	}
	var single falcoEvent
	if err := json.Unmarshal(line, &single); err == nil {
		return []falcoEvent{single}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	var out []falcoEvent
	for {
		var fe falcoEvent
		if err := dec.Decode(&fe); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		out = append(out, fe)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no JSON object decoded")
	}
	return out, nil
}

func (r *FalcoReader) send(events []Event) error {
	body, _ := json.Marshal(events)
	// Prefer v2 endpoint (canonical contract). Fallback to v1 on 404.
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v2/runtime/events", r.coreURL), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	corehttp.ApplyOptionalAuthorization(req)
	resp, err := r.httpClient.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			atomic.AddUint64(&r.v2Success, 1)
			return nil
		}
		if resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("falco events POST v2 failed: %s", resp.Status)
		}
	}

	// Fallback to v1
	req2, err2 := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/runtime/events", r.coreURL), bytes.NewReader(body))
	if err2 != nil {
		return err2
	}
	req2.Header.Set("Content-Type", "application/json")
	corehttp.ApplyOptionalAuthorization(req2)
	resp2, err3 := r.httpClient.Do(req2)
	if err3 != nil {
		return err3
	}
	defer resp2.Body.Close()
	if resp2.StatusCode < 200 || resp2.StatusCode >= 300 {
		return fmt.Errorf("falco events POST failed: %s", resp2.Status)
	}
	atomic.AddUint64(&r.v1Fallback, 1)
	return nil
}

func (r *FalcoReader) logIngestionStats(status string) {
	r.logger.Printf(
		"[IngestQuality] status=%s sent_batches=%d sent_events=%d failed_batches=%d failed_events=%d invalid_lines=%d v2_success=%d v1_fallback=%d",
		status,
		atomic.LoadUint64(&r.sentBatches),
		atomic.LoadUint64(&r.sentEvents),
		atomic.LoadUint64(&r.failedBatches),
		atomic.LoadUint64(&r.failedEvents),
		atomic.LoadUint64(&r.invalidLines),
		atomic.LoadUint64(&r.v2Success),
		atomic.LoadUint64(&r.v1Fallback),
	)
}

func (r *FalcoReader) toRuntimeEvent(ctx context.Context, fe *falcoEvent) (Event, bool) {
	fields := fe.OutputFields
	if fields == nil {
		fields = map[string]interface{}{}
	}

	podUID := firstNonEmpty(
		asString(fields["k8s.pod.uid"]),
		asString(fields["k8s.pod.uid.value"]),
		asString(fields["k8s.pod.uid_raw"]),
		asString(fields["k8s.pod.uid.raw"]),
	)
	ns := firstNonEmpty(
		asString(fields["k8s.ns.name"]),
		asString(fields["k8s.ns"]),
		asString(fields["k8s.namespace"]),
	)
	podName := firstNonEmpty(
		asString(fields["k8s.pod.name"]),
		asString(fields["k8s.pod"]),
	)
	node := firstNonEmpty(asString(fields["k8s.node.name"]), asString(fields["hostname"]), r.nodeName)

	// Some Falco rules (even with json output enabled) don't include k8s.pod.uid in output_fields.
	// Core requires pod_uid (NOT NULL), so resolve it from Kubernetes (best-effort, cached).
	if podUID == "" && podName != "" && ns != "" && r.kubeClient != nil {
		if resolved := r.resolvePodUID(ctx, ns, podName); resolved != "" {
			podUID = resolved
		}
	}

	syscall := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		asString(fields["evt.type"]),
		asString(fields["syscall.type"]),
	)))
	if syscall == "" {
		// Core rejects empty syscall; many Falco rules omit evt.type in output_fields.
		syscall = "falco.alert"
	}

	// For network-ish syscalls, prefer fd.* metadata because it contains connection tuple
	// (ip:port, dst=..., proto=...) which the signal classifiers use for heuristics.
	var target string
	switch strings.ToLower(syscall) {
	case "connect":
		fdName := firstNonEmpty(
			asString(fields["fd.name"]),
			asString(fields["fd.name.value"]),
			asString(fields["fd.name.raw"]),
		)
		proto := firstNonEmpty(asString(fields["fd.l4proto"]), asString(fields["fd.proto"]))

		// fd.name looks like: "<srcIP>:<srcPort>-><dstIP>:<dstPort>"
		dstIP, dstPort := "", ""
		if fdName != "" {
			if parts := strings.SplitN(fdName, "->", 2); len(parts) == 2 {
				rhs := strings.TrimSpace(parts[1]) // "<dstIP>:<dstPort>"
				if colon := strings.LastIndex(rhs, ":"); colon > 0 && colon < len(rhs)-1 {
					dstIP = strings.TrimSpace(rhs[:colon])
					dstPort = strings.TrimSpace(rhs[colon+1:])
				}
			}
		}

		// Include dst=/proto=/dport= tokens so core heuristics match reliably.
		if dstIP != "" && dstPort != "" {
			target = fmt.Sprintf("dst=%s:%s proto=%s dport=%s fd.name=%s", dstIP, dstPort, proto, dstPort, fdName)
		} else {
			target = fdName
		}
		if strings.TrimSpace(target) == "" {
			target = firstNonEmpty(
				asString(fields["proc.cmdline"]),
				asString(fields["proc.exepath"]),
				asString(fields["fd.name"]),
				fe.Output,
			)
		}
	default:
		target = firstNonEmpty(
			asString(fields["proc.cmdline"]),
			asString(fields["proc.exepath"]),
			asString(fields["fd.name"]),
			fe.Output,
		)
	}

	mitre := extractMitreTechnique(fe.Tags)
	sev := mapFalcoPriority(fe.Priority)
	ts := time.Now().Unix()
	now := time.Now().UTC()

	signal := strings.TrimSpace(firstNonEmpty(fe.Rule, "FALCO_ALERT"))
	payloadJSON := map[string]interface{}{
		"runtime":       "falco",
		"rule":          strings.TrimSpace(fe.Rule),
		"priority":      strings.TrimSpace(fe.Priority),
		"output":        fe.Output,
		"tags":          fe.Tags,
		"syscall":       syscall,
		"target":        strings.TrimSpace(target),
		"signal":        signal,
		"mitre":         extractMitreTechnique(fe.Tags),
		"severity":      sev,
		"node":          node,
		"pod_namespace": ns,
		"pod_name":      podName,
	}
	payloadBytes, _ := json.Marshal(payloadJSON)
	payloadHash := sha256.Sum256(payloadBytes)
	evIDHash := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%d|%s", podUID, syscall, strings.TrimSpace(target), ts, strings.TrimSpace(fe.Rule))))

	return Event{
		EventType:      "runtime.falco.alert",
		MitreTechnique: mitre,
		Signal:         signal,
		Severity:       sev,
		Pod: map[string]interface{}{
			"uid":       podUID,
			"namespace": ns,
			"name":      podName,
			"node":      node,
		},
		Runtime:    "falco",
		Syscall:    syscall,
		Target:     strings.TrimSpace(target),
		Capability: "",
		Timestamp:  ts,

		// Canonical contract fields (v2 ingest)
		EventID:         hex.EncodeToString(evIDHash[:]),
		ObservedAt:      now.Format(time.RFC3339),
		IngestedAt:      now.Format(time.RFC3339),
		ResolutionState: resolutionStateFromFalco(fe),
		SourceKind:      "falco",
		SourceSensorID:  strings.TrimSpace(r.nodeName),
		SourceRule:      strings.TrimSpace(fe.Rule),
		PayloadJSON:     payloadJSON,
		PayloadHash:     hex.EncodeToString(payloadHash[:]),
	}, podUID != "" // Core needs pod uid to store
}

func resolutionStateFromFalco(fe *falcoEvent) string {
	if fe == nil {
		return "unresolved"
	}
	fields := fe.OutputFields
	if fields != nil {
		v := firstNonEmpty(
			asString(fields["fortuna.resolution_state"]),
			asString(fields["resolution_state"]),
		)
		if strings.TrimSpace(v) != "" {
			return normalizeResolutionState(v)
		}
	}
	for _, t := range fe.Tags {
		s := strings.TrimSpace(strings.ToLower(t))
		if strings.HasPrefix(s, "resolution_state=") {
			return normalizeResolutionState(strings.TrimSpace(strings.TrimPrefix(s, "resolution_state=")))
		}
		if strings.HasPrefix(s, "fortuna.resolution_state=") {
			return normalizeResolutionState(strings.TrimSpace(strings.TrimPrefix(s, "fortuna.resolution_state=")))
		}
	}
	return "unresolved"
}

func (r *FalcoReader) resolvePodUID(ctx context.Context, namespace, podName string) string {
	if r.kubeClient == nil || namespace == "" || podName == "" {
		return ""
	}

	key := namespace + "/" + podName
	now := time.Now()

	r.podUIDCacheMu.Lock()
	if ent, ok := r.podUIDCache[key]; ok && ent.expiresAt.After(now) {
		uid := ent.uid
		r.podUIDCacheMu.Unlock()
		return uid
	}
	r.podUIDCacheMu.Unlock()

	pod, err := r.kubeClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		// Some deployments grant list/watch but not get. Fall back to LIST by name.
		pods, lerr := r.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			FieldSelector: "metadata.name=" + podName,
			Limit:         1,
		})
		if lerr != nil || pods == nil || len(pods.Items) == 0 {
			return ""
		}
		pod = &pods.Items[0]
	}

	uid := string(pod.UID)
	if strings.TrimSpace(uid) == "" {
		return ""
	}

	ttl := 5 * time.Minute
	r.podUIDCacheMu.Lock()
	r.podUIDCache[key] = podUIDCacheEntry{
		uid:       uid,
		expiresAt: now.Add(ttl),
	}
	r.podUIDCacheMu.Unlock()
	return uid
}

func extractMitreTechnique(tags []string) string {
	for _, t := range tags {
		s := strings.TrimSpace(t)
		if s == "" {
			continue
		}
		// common patterns:
		// - "mitre_technique=T1059"
		// - "mitre_attack_technique=T1059"
		// - "mitre=T1059"
		for _, p := range []string{"mitre_technique=", "mitre_attack_technique=", "mitre="} {
			if strings.HasPrefix(s, p) {
				return strings.TrimSpace(strings.TrimPrefix(s, p))
			}
		}
		// allow plain "T1059" tag
		if len(s) >= 5 && (s[0] == 'T' || s[0] == 't') && isDigits(s[1:5]) {
			return strings.ToUpper(s[:5])
		}
	}
	return ""
}

func mapFalcoPriority(p string) string {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "emergency", "alert", "critical":
		return "critical"
	case "error":
		return "high"
	case "warning":
		return "medium"
	case "notice":
		return "low"
	case "informational", "info", "debug":
		return "low"
	default:
		return "low"
	}
}

func asString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func isDigits(s string) bool {
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
