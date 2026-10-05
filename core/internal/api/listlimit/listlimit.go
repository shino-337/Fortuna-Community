// Package listlimit holds the shared row-limit parsing used by every API list
// endpoint, so that "limit" behaves the same way across the API surface:
// missing, non-numeric or non-positive values fall back to the endpoint
// default, and values above the endpoint hard maximum are clamped to it.
//
// Handlers that previously returned an unbounded collection fetch limit+1 rows
// and use Trim to report whether the result was truncated, which keeps their
// existing response shape and only adds a "truncated" flag.
package listlimit

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Parse reads the "limit" query parameter (see ParseParam).
func Parse(c *gin.Context, def, max int) int {
	return ParseParam(c, "limit", def, max)
}

// ParseParam reads an integer row limit from the named query parameter.
// Missing, non-numeric, zero or negative values return def; values above max
// return max. def is itself clamped into [1, max].
func ParseParam(c *gin.Context, name string, def, max int) int {
	return Clamp(strings.TrimSpace(c.Query(name)), def, max)
}

// Clamp applies the shared limit rules to a raw string value.
func Clamp(raw string, def, max int) int {
	if max < 1 {
		max = 1
	}
	if def < 1 {
		def = 1
	}
	if def > max {
		def = max
	}
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

// Trim cuts rows fetched with Limit(limit+1) down to limit and reports whether
// more rows existed.
func Trim[T any](rows []T, limit int) ([]T, bool) {
	if limit >= 0 && len(rows) > limit {
		return rows[:limit], true
	}
	return rows, false
}
