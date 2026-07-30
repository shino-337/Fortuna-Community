package graph

import (
	"fmt"
	"strings"
)

// ─── Story Generation Engine (Spec §7) ────────────────────────────────────────
//
// Converts a structured AttackChain into a human-readable attack narrative
// following the pentest-report style defined in the UI/UX spec.
//
// Output:
//   Headline   — emoji + "Container escape → Node control → Cluster takeover"
//   Narrative  — prose paragraph from template
//   ImpactText — bullet summary of what attacker gains
//   ExploitText— difficulty assessment
//   Steps      — ordered step list for technical details panel

// ─── Spec §5.1: Conditional language ─────────────────────────────────────────
//
// Confidence thresholds control what verb is used in the narrative.
// This prevents assertive language when evidence is partial or inferred.

// confidenceToLanguage returns the appropriate modal verb for the chain confidence.
// Spec §5.1:
//   ≥ 0.75 → "can"
//   0.5–0.75 → "may"
//   < 0.5  → "unlikely but could"
func confidenceToLanguage(confidenceRaw float64) string {
	switch {
	case confidenceRaw >= 0.75:
		return "can"
	case confidenceRaw >= 0.50:
		return "may"
	default:
		return "unlikely but could"
	}
}

// ─── Step text mapping (spec §7.2) ────────────────────────────────────────────

// stepTextFromCapability maps a capability/provide token to a human action verb phrase.
var stepTextFromCapability = map[string]string{
	"NODE_SHELL_ACCESS":        "gain shell access to the node",
	"KUBELET_API_ACCESS":       "reach the kubelet API",
	"CONTAINER_RUNTIME_ACCESS": "access the container runtime",
	"NODE_ACCESS":              "gain access to the node",
	"SA_TOKEN":                 "obtain a service account token",
	"SA_TOKEN:*":               "harvest service account tokens from node storage",
	"ROLE":                     "escalate privileges via RBAC",
	"DATA_ACCESS:secrets":      "read Kubernetes secrets",
	"WORKLOAD_CONTROL":         "deploy or modify workloads",
	"EXECUTION":                "execute commands in cluster containers",
	"IDENTITY_FORGE":           "forge cluster identity certificates",
	"NETWORK_ACCESS":           "reach cluster-internal services",
}

// chainTypeToSteps maps rule types to an ordered sequence of action tokens
// that describe the canonical attack story.
var chainTypeToSteps = map[string][]string{
	"ESCAPE_TO_PRIV_ESC": {
		"container_escape",
		"node_access",
		"token_harvest",
		"rbac_escalation",
	},
	"LATERAL_TO_PRIV_ESC": {
		"lateral_move",
		"token_acquire",
		"rbac_escalation",
	},
	"NETWORK_BRIDGE": {
		"network_reach",
		"token_acquire",
		"rbac_escalation",
	},
	"SA_TOKEN_REUSE": {
		"token_reuse",
		"rbac_escalation",
	},
	"NODE_DOMINANCE": {
		"node_access",
		"cluster_control",
	},
	"PRIV_ESC_LADDER": {
		"initial_priv_esc",
		"secondary_priv_esc",
	},
}

// stepActionText maps canonical step tokens to human verb phrases.
var stepActionText = map[string]string{
	"container_escape":    "escape the container via %s",
	"node_access":         "gain access to node %s",
	"token_harvest":       "harvest service account tokens from node storage",
	"rbac_escalation":     "escalate privileges to %s level",
	"lateral_move":        "move laterally to pod %s",
	"token_acquire":       "obtain a service account token",
	"network_reach":       "reach pod %s via network",
	"token_reuse":         "reuse a service account token",
	"cluster_control":     "exert control over cluster resources",
	"initial_priv_esc":    "escalate to %s",
	"secondary_priv_esc":  "further escalate to %s",
}

// ─── Impact text mapping (spec §7.3) ──────────────────────────────────────────

var impactTextByObjective = map[string][]string{
	"CLUSTER_TAKEOVER": {
		"Full control over all cluster workloads and infrastructure",
		"Access to all Kubernetes secrets across all namespaces",
		"Ability to deploy, modify, or destroy any workload",
		"Complete visibility into cluster-internal communications",
	},
	"NODE_COMPROMISE": {
		"Control over all pods running on the compromised node",
		"Access to service account tokens mounted on the node",
		"Ability to pivot to other nodes via node-level credentials",
	},
	"SECRET_EXFIL": {
		"Read access to Kubernetes secrets in bound namespaces",
		"Potential exposure of database credentials, API keys, or certificates",
	},
	"WORKLOAD_CONTROL": {
		"Ability to create or modify workloads for persistence",
		"Potential privilege escalation via pod spec manipulation",
		"Risk of workload injection for lateral movement",
	},
	"LATERAL_MOVEMENT": {
		"Ability to reach additional cluster services from the compromised pod",
		"Increased attack surface via cross-namespace access",
	},
	"DATA_EXFILTRATION": {
		"Access to data exposed via the compromised service account",
	},
}

// ─── Headline emoji mapping ────────────────────────────────────────────────────

var headlineEmojiByObjective = map[string]string{
	"CLUSTER_TAKEOVER": "🔥",
	"NODE_COMPROMISE":  "⚠️",
	"SECRET_EXFIL":     "🔑",
	"WORKLOAD_CONTROL": "🚀",
	"LATERAL_MOVEMENT": "↔️",
	"DATA_EXFILTRATION": "📦",
}

// ─── Headline phrase chain ────────────────────────────────────────────────────

var chainTypeHeadline = map[string]string{
	"ESCAPE_TO_PRIV_ESC":  "Container escape → Node control → %s",
	"LATERAL_TO_PRIV_ESC": "Lateral movement → Token capture → %s",
	"NETWORK_BRIDGE":      "Network reach → Token capture → %s",
	"SA_TOKEN_REUSE":      "Token reuse → %s",
	"NODE_DOMINANCE":      "Node access → %s",
	"PRIV_ESC_LADDER":     "Privilege escalation → %s",
}

// objectivePhraseShort maps objective to a short target phrase for headlines.
var objectivePhraseShort = map[string]string{
	"CLUSTER_TAKEOVER": "Cluster takeover",
	"NODE_COMPROMISE":  "Node compromise",
	"SECRET_EXFIL":     "Secret exfiltration",
	"WORKLOAD_CONTROL": "Workload control",
	"LATERAL_MOVEMENT": "Lateral movement",
	"DATA_EXFILTRATION": "Data exfiltration",
}

// ─── Exploit difficulty text ───────────────────────────────────────────────────

// exploitabilityText returns a human-readable difficulty string based on cost
// and chain type (spec §6.2).
func exploitabilityText(cost int, chainType string, realism float64) string {
	verb := "Requires"
	switch {
	case cost <= 4:
		verb = "Straightforward —"
	case cost <= 7:
		verb = "Moderate —"
	default:
		verb = "Complex —"
	}

	steps := "multiple attack steps"
	switch chainType {
	case "ESCAPE_TO_PRIV_ESC":
		steps = "container escape, node pivot, and token harvest"
	case "LATERAL_TO_PRIV_ESC":
		steps = "lateral movement and token capture"
	case "SA_TOKEN_REUSE":
		steps = "valid service account token reuse"
	case "NODE_DOMINANCE":
		steps = "node-level access"
	}

	realismStr := ""
	switch {
	case realism >= 0.85:
		realismStr = " (high exploit likelihood in typical clusters)"
	case realism >= 0.60:
		realismStr = " (moderate exploit likelihood; depends on cluster hardening)"
	default:
		realismStr = " (lower likelihood in hardened clusters)"
	}

	return fmt.Sprintf("%s requires %s%s", verb, steps, realismStr)
}

// ─── Main entry point ─────────────────────────────────────────────────────────

// GenerateStory builds the AttackStory for a chain given the paths it references.
func GenerateStory(ch AttackChain, paths []pathNormalized, totalClusterNodes int) AttackStory {
	sourceName := chainSourceName(ch, paths)
	escapeMechanism := chainEscapeMechanism(ch, paths)
	nodeTarget := chainNodeTarget(ch, paths)

	// Confidence-conditional language (spec §5.1)
	confLang := confidenceToLanguage(ch.ConfidenceRaw)

	// Headline
	chainPhrase, ok := chainTypeHeadline[ch.Type]
	if !ok {
		chainPhrase = "%s"
	}
	objShort := objectivePhraseShort[ch.Objective]
	if objShort == "" {
		objShort = strings.ReplaceAll(ch.Objective, "_", " ")
	}
	emoji := headlineEmojiByObjective[ch.Objective]
	if emoji == "" {
		emoji = "⚡"
	}
	headline := fmt.Sprintf("%s %s", emoji, fmt.Sprintf(chainPhrase, objShort))

	// Narrative prose — now conditional (spec §5.2)
	narrative := buildNarrative(ch, sourceName, escapeMechanism, nodeTarget, paths, confLang)

	// Impact text
	impactBullets := impactTextByObjective[ch.Objective]
	if len(impactBullets) == 0 {
		impactBullets = []string{"Access to resources via compromised service account"}
	}
	impactText := strings.Join(impactBullets, "\n• ")
	impactText = "• " + impactText

	// Exploit text
	exploitText := exploitabilityText(ch.ExploitCost, ch.Type, ch.Realism)

	// Technical steps
	steps := buildTechnicalSteps(ch, paths, escapeMechanism, nodeTarget)

	// Evidence summary (spec §5.4 — displayed FIRST in UI before narrative)
	evSummary := buildEvidenceSummary(ch.Evidence)

	// Assumption summary (spec §5.4 — displayed between evidence and story)
	asmSummary := buildAssumptionSummary(ch.Assumptions)

	return AttackStory{
		Headline:          headline,
		Narrative:         narrative,
		ImpactText:        impactText,
		ExploitText:       exploitText,
		Steps:             steps,
		EvidenceSummary:   evSummary,
		AssumptionSummary: asmSummary,
	}
}

// buildNarrative constructs the conditional prose narrative (spec §5.2 template):
// "If an attacker compromises {source}, and successfully performs {technique},
//  they {may/can} gain {capability}, which allows them to {technique_2},
//  eventually leading to {objective}."
func buildNarrative(ch AttackChain, source, escape, node string, paths []pathNormalized, confLang string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("If an attacker compromises %s, ", source))

	switch ch.Type {
	case "ESCAPE_TO_PRIV_ESC":
		b.WriteString("and successfully performs a container escape")
		if escape != "" {
			b.WriteString(fmt.Sprintf(" via %s", escape))
		}
		b.WriteString(fmt.Sprintf(", they %s gain access to the underlying node", confLang))
		if node != "" {
			b.WriteString(fmt.Sprintf(" (%s)", node))
		}
		b.WriteString(fmt.Sprintf(", harvest service account tokens, and %s ", confLang))
		b.WriteString(objectiveActionPhrase(ch.Objective))

	case "LATERAL_TO_PRIV_ESC":
		b.WriteString(fmt.Sprintf("they %s move laterally to another pod, ", confLang))
		b.WriteString(fmt.Sprintf("capture its service account token, and %s ", confLang))
		b.WriteString(objectiveActionPhrase(ch.Objective))

	case "NETWORK_BRIDGE":
		b.WriteString(fmt.Sprintf("they %s reach an internal cluster service via network, ", confLang))
		b.WriteString(fmt.Sprintf("obtain its service account token, and %s ", confLang))
		b.WriteString(objectiveActionPhrase(ch.Objective))

	case "SA_TOKEN_REUSE":
		b.WriteString(fmt.Sprintf("they %s reuse an existing service account token to ", confLang))
		b.WriteString(objectiveActionPhrase(ch.Objective))

	case "NODE_DOMINANCE":
		if node != "" {
			b.WriteString(fmt.Sprintf("they %s leverage existing node access on %s to ", confLang, node))
		} else {
			b.WriteString(fmt.Sprintf("they %s leverage existing node access to ", confLang))
		}
		b.WriteString(objectiveActionPhrase(ch.Objective))

	default:
		b.WriteString(fmt.Sprintf("they %s chain multiple attack steps to ", confLang))
		b.WriteString(objectiveActionPhrase(ch.Objective))
	}

	b.WriteString(".")

	// Conditional qualifier for low-confidence chains
	if ch.ConfidenceRaw < 0.50 {
		b.WriteString(" Note: this scenario requires multiple unconfirmed conditions to hold simultaneously.")
	}

	// Blast radius context
	if len(ch.VariantNodes) > 1 {
		b.WriteString(fmt.Sprintf(
			" This attack path is also feasible via %d other node(s), widening the blast radius.",
			len(ch.VariantNodes)-1,
		))
	}

	return b.String()
}

// buildEvidenceSummary creates a concise evidence list for the UI (spec §5.4).
func buildEvidenceSummary(evidence []Evidence) []string {
	out := make([]string, 0, len(evidence))
	for _, ev := range evidence {
		var label string
		switch ev.Type {
		case "FACT":
			label = "[Confirmed]"
		case "CONFIG":
			label = "[Config]"
		case "RELATION":
			label = "[Relation]"
		default:
			label = "[Evidence]"
		}
		out = append(out, fmt.Sprintf("%s %s — %s", label, ev.SourceRef, ev.Detail))
	}
	return out
}

// buildAssumptionSummary creates a concise assumption list for the UI (spec §5.4).
func buildAssumptionSummary(assumptions []Assumption) []string {
	out := make([]string, 0, len(assumptions))
	for _, a := range assumptions {
		out = append(out, fmt.Sprintf("%s (source: %s, confidence: %.0f%%)",
			a.Description, a.Source, a.Confidence*100))
	}
	return out
}

// objectiveActionPhrase maps an objective to the final consequence clause.
func objectiveActionPhrase(objective string) string {
	switch objective {
	case "CLUSTER_TAKEOVER":
		return "escalate privileges to cluster-admin level, gaining full control over the cluster"
	case "NODE_COMPROMISE":
		return "achieve persistent access to the underlying node"
	case "SECRET_EXFIL":
		return "read sensitive Kubernetes secrets across namespaces"
	case "WORKLOAD_CONTROL":
		return "deploy or manipulate workloads for persistence"
	case "LATERAL_MOVEMENT":
		return "pivot to additional cluster services"
	default:
		return "access protected cluster resources"
	}
}

// buildTechnicalSteps returns an ordered list of technical action strings.
func buildTechnicalSteps(ch AttackChain, paths []pathNormalized, escape, node string) []string {
	steps := []string{}
	canonical, ok := chainTypeToSteps[ch.Type]
	if !ok {
		canonical = []string{"access", "exploit"}
	}
	for _, tok := range canonical {
		tmpl := stepActionText[tok]
		if tmpl == "" {
			continue
		}
		var text string
		switch tok {
		case "container_escape":
			if escape != "" {
				text = fmt.Sprintf(tmpl, escape)
			} else {
				text = "escape the container"
			}
		case "node_access":
			if node != "" {
				text = fmt.Sprintf(tmpl, node)
			} else {
				text = "gain access to a cluster node"
			}
		case "rbac_escalation", "initial_priv_esc", "secondary_priv_esc":
			text = fmt.Sprintf(tmpl, roleFromFinalTarget(ch.FinalTarget))
		case "lateral_move", "network_reach":
			text = fmt.Sprintf(tmpl, peerPodFromPaths(paths))
		default:
			text = tmpl
		}
		steps = append(steps, text)
	}
	return steps
}

// ─── Chain introspection helpers ──────────────────────────────────────────────

func chainSourceName(ch AttackChain, paths []pathNormalized) string {
	for _, pid := range ch.Paths {
		for _, p := range paths {
			if p.PathID == pid {
				for _, n := range p.Raw.Nodes {
					if strings.EqualFold(n.Type, NodeTypePod) {
						if name, ok := n.Properties["name"].(string); ok && name != "" {
							return name
						}
						return n.ID
					}
				}
			}
		}
	}
	return "the compromised pod"
}

func chainEscapeMechanism(ch AttackChain, paths []pathNormalized) string {
	for _, pid := range ch.Paths {
		for _, p := range paths {
			if p.PathID == pid {
				for _, n := range p.Raw.Nodes {
					if n.Type == NodeTypeCapability {
						// Convert capability ID to human label.
						return capabilityHumanLabel(n.ID)
					}
				}
			}
		}
	}
	return ""
}

func chainNodeTarget(ch AttackChain, paths []pathNormalized) string {
	for _, pid := range ch.Paths {
		for _, p := range paths {
			if p.PathID == pid {
				for _, n := range p.Raw.Nodes {
					if strings.EqualFold(n.Type, NodeTypeNode) {
						if name, ok := n.Properties["name"].(string); ok && name != "" {
							return name
						}
						return n.ID
					}
				}
			}
		}
	}
	return ""
}

func roleFromFinalTarget(ft string) string {
	// FinalTarget format: "cluster_role:role:cluster-admin"
	parts := strings.SplitN(ft, ":", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ft
}

func peerPodFromPaths(paths []pathNormalized) string {
	for _, p := range paths {
		for _, n := range p.Raw.Nodes {
			if strings.EqualFold(n.Type, NodeTypePod) {
				if name, ok := n.Properties["name"].(string); ok && name != "" {
					return name
				}
			}
		}
	}
	return "peer pod"
}

// capabilityHumanLabel converts a capability ID to a readable mechanism description.
func capabilityHumanLabel(capID string) string {
	switch strings.ToUpper(strings.TrimSpace(capID)) {
	case "ESC_HOSTPATH_NODE":
		return "hostPath volume mount"
	case "ESC_RUNTIME_ACTIVE":
		return "container runtime escape"
	case "ESC_RUNTIME_PROC_ROOT":
		return "/proc/1/root filesystem pivot"
	case "ESC_PRIV_POD":
		return "privileged container"
	case "ESC_HOSTPID_POD":
		return "hostPID namespace access"
	case "ESC_HOSTIPC_POD":
		return "hostIPC namespace access"
	case "ESC_RUNTIME_PROBE":
		return "runtime probe"
	default:
		// Strip cap: prefix and format
		label := capID
		if idx := strings.LastIndex(label, ":"); idx >= 0 {
			label = label[idx+1:]
		}
		return strings.ToLower(strings.ReplaceAll(label, "_", " "))
	}
}
