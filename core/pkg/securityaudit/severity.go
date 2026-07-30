package securityaudit

import "strings"

// ClassifyResultSeverity maps action + result to a recommended governance severity (low|medium|high|critical).
func ClassifyResultSeverity(action, result string) string {
	r := strings.ToLower(strings.TrimSpace(result))
	a := strings.ToLower(strings.TrimSpace(action))
	switch r {
	case "deny", "error":
		return "high"
	case "success":
		if strings.Contains(a, "delete") || strings.Contains(a, "revoke_all") || strings.Contains(a, "bulk") {
			return "high"
		}
		if strings.Contains(a, "password") || strings.Contains(a, "rbac") || strings.Contains(a, "rotate") || strings.Contains(a, "publish") {
			return "high"
		}
		if strings.Contains(a, "upload") || strings.Contains(a, "export") || strings.Contains(a, "graph") {
			return "medium"
		}
		return "low"
	default:
		return "medium"
	}
}
