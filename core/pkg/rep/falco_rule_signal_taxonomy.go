package rep

import (
	"strings"
)

// FalcoRuleSignal maps a Falco rule name (and optional syscall) to Fortuna semantic
// runtime_signal types used by:
//   - SignalAdapter (runtime_signals row)
//   - classifySignal / legacy REP (capability promotion + baseScore)
//   - asset_security_state booleans (has_suspicious_exec, has_escape_related, …)
//
// When evt.type is missing, Agent sends syscall "falco.alert". This taxonomy fills the gap
// until a full "Falco rule → runtime_signal" registry (DB or YAML) lands.
//
// Matching: rule name is lowercased; each pattern uses substring Contains (first matching group wins).
type FalcoRuleSignal struct {
	SignalType string
	Category   string
	Confidence float64
	Mitre      string
	BaseScore  int
}

type falcoRulePatternGroup struct {
	substrings []string
	out        FalcoRuleSignal
}

// Order matters: first match wins (more specific groups first).
var falcoRulePatternGroups = []falcoRulePatternGroup{
	{
		substrings: []string{
			"mount", "sensitive mount", "release_agent", "debugfs", "filesystem",
			"create hard link", "hardlink",
		},
		out: FalcoRuleSignal{
			SignalType: "FS_ESCAPE_ATTEMPT",
			Category:   "ESCAPE",
			Confidence: 0.9,
			Mitre:      "T1610",
			BaseScore:  72,
		},
	},
	{
		substrings: []string{
			"unshare", "setns", "namespace change", "privileged pod with namespace",
			"thread namespace", "mount namespace",
		},
		out: FalcoRuleSignal{
			SignalType: "NAMESPACE_ESCAPE",
			Category:   "ESCAPE",
			Confidence: 0.86,
			Mitre:      "T1055",
			BaseScore:  68,
		},
	},
	{
		substrings: []string{
			"/proc/", "proc files", "read sensitive file", "credential", "kubeconfig",
			"service account token", "token", "privilege escalation",
		},
		out: FalcoRuleSignal{
			SignalType: "PROC_ROOT_PIVOT",
			Category:   "ESCAPE",
			Confidence: 0.84,
			Mitre:      "T1611.001",
			BaseScore:  70,
		},
	},
	{
		substrings: []string{
			"privileged container", "privileged pod", "launch privileged",
			"privileged", "non sudo setuid", "user exec binary",
		},
		out: FalcoRuleSignal{
			SignalType: "CAPABILITY_MISUSE",
			Category:   "ESCAPE",
			Confidence: 0.8,
			Mitre:      "T1611.002",
			BaseScore:  62,
		},
	},
	{
		substrings: []string{
			"unexpected network", "outbound connection", "network tool",
			"network activity", "packet", "dns", "ssh", "nc ", "netcat",
			"listen on", "port forward",
		},
		out: FalcoRuleSignal{
			SignalType: "NETWORK_QUEUE_ANOMALY",
			Category:   "NETWORK",
			Confidence: 0.72,
			Mitre:      "T1046",
			BaseScore:  48,
		},
	},
	{
		substrings: []string{
			"terminal shell", "shell in container", "interactive shell",
			"spawned shell", "suspicious binary", "system procs", "run shell",
			"base64", "curl", "wget", "reverse shell",
		},
		out: FalcoRuleSignal{
			SignalType: "SUSPICIOUS_EXEC_FROM_SNAPSHOT",
			Category:   "EXECUTION",
			Confidence: 0.82,
			Mitre:      "T1059",
			BaseScore:  52,
		},
	},
}

// ClassifyFalcoRuleToSignal returns semantic mapping when runtime is Falco and syscall
// is generic (e.g. falco.alert) or when rule name alone is sufficient.
func ClassifyFalcoRuleToSignal(ruleName, syscall string) (FalcoRuleSignal, bool) {
	rule := normalizeFalcoRuleName(ruleName)
	syscall = strings.ToLower(strings.TrimSpace(syscall))

	if rule != "" {
		for _, g := range falcoRulePatternGroups {
			for _, sub := range g.substrings {
				if sub == "" {
					continue
				}
				if strings.Contains(rule, strings.ToLower(sub)) {
					return g.out, true
				}
			}
		}
		// Named rule but no pattern match: still above UNKNOWN for scoring / insight rules.
		return FalcoRuleSignal{
			SignalType: "SUSPICIOUS_EXEC_FROM_SNAPSHOT",
			Category:   "EXECUTION",
			Confidence: 0.62,
			Mitre:      "T1059",
			BaseScore:  42,
		}, true
	}

	// Generic Falco alert without evt.type: still better than UNKNOWN for scoring / YAML runtime rules.
	if isFalcoGenericSyscall(syscall) {
		return FalcoRuleSignal{
			SignalType: "SUSPICIOUS_EXEC_FROM_SNAPSHOT",
			Category:   "EXECUTION",
			Confidence: 0.58,
			Mitre:      "T1059",
			BaseScore:  38,
		}, true
	}

	return FalcoRuleSignal{}, false
}

func normalizeFalcoRuleName(rule string) string {
	return strings.ToLower(strings.TrimSpace(rule))
}

func isFalcoGenericSyscall(syscall string) bool {
	s := strings.ToLower(strings.TrimSpace(syscall))
	return s == "" || s == "falco.alert"
}

// FalcoRuleWorthRescore reports whether a Falco alert (named rule) should open a debounced
// rescore window. Any non-empty rule name qualifies so low-priority Falco severities still refresh V3.
func FalcoRuleWorthRescore(rule string) bool {
	return strings.TrimSpace(rule) != ""
}
