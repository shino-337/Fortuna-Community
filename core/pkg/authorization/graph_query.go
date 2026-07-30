package authorization

import (
	"context"
	"regexp"
	"strings"
	"time"
)

// Graph safe-query limits (RBAC.md §13.2).
const (
	GraphSafeMaxDepthSeconds = 10 * time.Second
	GraphSafeMaxResultRows   = 1000
	GraphSafeMaxDepthTraversal = 5
)

var graphWritePattern = regexp.MustCompile(`(?is)\b(CREATE|DELETE|SET\s|REMOVE|MERGE|DROP|DETACH\s+DELETE|CALL\s+db\.|CALL\s+apoc\.)\b`)

// ContextForGraphQuery returns a context with timeout appropriate for query mode.
func ContextForGraphQuery(parent context.Context, advanced bool) (context.Context, context.CancelFunc) {
	if advanced {
		return context.WithTimeout(parent, 60*time.Second)
	}
	return context.WithTimeout(parent, GraphSafeMaxDepthSeconds)
}

// ValidateSafeGraphQuery rejects obviously mutating or system-privileged Cypher for graph.query.safe callers.
func ValidateSafeGraphQuery(q string) bool {
	q = strings.TrimSpace(q)
	if q == "" {
		return false
	}
	if graphWritePattern.MatchString(q) {
		return false
	}
	return true
}
