package rep

import "time"

// REP_CDetector is governance metadata for a stateful correlator (REP-C). See docs/adr/004-rep-detector-governance.md.
type REP_CDetector struct {
	ID               string
	Version          string
	IncidentType     string
	InputFactTypes   []string
	Window           time.Duration
	WindowLabel      string
	MinDistinctFacts int64 // 0 when not applicable (non-count detectors)
	Cooldown         time.Duration
	Confidence       float64
	SeverityHint     string
	ExpectedFPClass  string
	BucketWindow     time.Duration // used for deterministic incident_id buckets
}

// Bucket returns the time-bucket key component for incident_id (unix / bucket seconds).
func (d REP_CDetector) Bucket(t time.Time) int64 {
	sec := int64(d.BucketWindow.Seconds())
	if sec <= 0 {
		sec = int64((5 * time.Minute).Seconds())
	}
	return t.UTC().Unix() / sec
}

// DetectorReconBurst — registry entry aligned with correlateReconBurst.
var DetectorReconBurst = REP_CDetector{
	ID:               "repc.recon_burst.v1",
	Version:          "1",
	IncidentType:     "RECON_BURST",
	InputFactTypes:   []string{"NETWORK_CONNECT", "EXTERNAL_CONNECT"},
	Window:           5 * time.Minute,
	WindowLabel:      "5m",
	MinDistinctFacts: 6,
	Cooldown:         30 * time.Minute,
	Confidence:       0.70,
	SeverityHint:     "MEDIUM",
	ExpectedFPClass:  "chatty_workload_network",
	BucketWindow:     5 * time.Minute,
}

// DetectorPostExploitExecChain — registry entry aligned with correlatePostExploitExecChain.
var DetectorPostExploitExecChain = REP_CDetector{
	ID:               "repc.post_exploit_exec_chain.v1",
	Version:          "1",
	IncidentType:     "POST_EXPLOIT_EXEC_CHAIN",
	InputFactTypes:   []string{"INTERACTIVE_SHELL", "REMOTE_TOOL_EXEC", "TMP_BINARY_EXEC"},
	Window:           10 * time.Minute,
	WindowLabel:      "10m",
	MinDistinctFacts: 0,
	Cooldown:         30 * time.Minute,
	Confidence:       0.85,
	SeverityHint:     "HIGH",
	ExpectedFPClass:  "dev_interactive_workload",
	BucketWindow:     10 * time.Minute,
}

// DetectorExfilLikeSequence — registry entry aligned with correlateExfilLikeSequence.
var DetectorExfilLikeSequence = REP_CDetector{
	ID:               "repc.exfil_like_sequence.v1",
	Version:          "1",
	IncidentType:     "EXFIL_LIKE_SEQUENCE",
	InputFactTypes:   []string{"SERVICEACCOUNT_TOKEN_READ", "EXTERNAL_CONNECT"},
	Window:           5 * time.Minute,
	WindowLabel:      "5m",
	MinDistinctFacts: 0,
	Cooldown:         30 * time.Minute,
	Confidence:       0.80,
	SeverityHint:     "HIGH",
	ExpectedFPClass:  "legit_external_api_after_token_cache",
	BucketWindow:     5 * time.Minute,
}

// AllREP_CDetectors returns all registered REP-C detectors (order stable for tests/docs).
func AllREP_CDetectors() []REP_CDetector {
	return []REP_CDetector{
		DetectorReconBurst,
		DetectorPostExploitExecChain,
		DetectorExfilLikeSequence,
	}
}
