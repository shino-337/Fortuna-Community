package authorization

import (
	"time"
)

// Graph safe-query limits (RBAC.md §13.2).
const (
	GraphSafeMaxDepthSeconds   = 10 * time.Second
	GraphSafeMaxResultRows     = 1000
	GraphSafeMaxDepthTraversal = 5
)
