package authorization

// GraphTraversalMaxResultNodes caps how many nodes may be returned from traversal-style endpoints.
const GraphTraversalMaxResultNodes = 2048

// GraphQueryMaxComplexityScore is a coarse budget for non-advanced Cypher POST /graph/query.
const GraphQueryMaxComplexityScore = 220
