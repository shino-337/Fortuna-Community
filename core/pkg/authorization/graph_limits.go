package authorization

import (
	"strings"
	"unicode/utf8"
)

// GraphTraversalMaxResultNodes caps how many nodes may be returned from traversal-style endpoints.
const GraphTraversalMaxResultNodes = 2048

// GraphQueryMaxComplexityScore is a coarse budget for non-advanced Cypher POST /graph/query.
const GraphQueryMaxComplexityScore = 220

// GraphQueryComplexityScore returns a rough complexity estimate (length + MATCH/WITH/CALL weight).
func GraphQueryComplexityScore(q string) int {
	q = strings.TrimSpace(q)
	if q == "" {
		return 0
	}
	score := utf8.RuneCountInString(q)
	score += strings.Count(strings.ToUpper(q), "MATCH") * 12
	score += strings.Count(strings.ToUpper(q), "OPTIONAL MATCH") * 8
	score += strings.Count(strings.ToUpper(q), "CALL") * 25
	score += strings.Count(strings.ToUpper(q), "UNWIND") * 10
	score += strings.Count(strings.ToUpper(q), "WITH") * 4
	return score
}
