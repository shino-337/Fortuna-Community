package investigation

import (
	"fmt"
	"strings"
)

// Canonical investigation lifecycle states (v2).
const (
	StatusOpen        = "OPEN"
	StatusTriaged     = "TRIAGED"
	StatusActive      = "ACTIVE"
	StatusContained   = "CONTAINED"
	StatusRemediating = "REMEDIATING"
	StatusResolved    = "RESOLVED"
	StatusArchived    = "ARCHIVED"
)

var allowedTransitions = map[string][]string{
	StatusOpen:        {StatusTriaged, StatusActive, StatusArchived},
	StatusTriaged:     {StatusActive, StatusContained, StatusArchived},
	StatusActive:      {StatusContained, StatusRemediating, StatusTriaged, StatusArchived},
	StatusContained:   {StatusRemediating, StatusResolved, StatusActive, StatusArchived},
	StatusRemediating: {StatusResolved, StatusContained, StatusArchived},
	StatusResolved:    {StatusArchived, StatusActive},
	StatusArchived:    {}, // terminal; restore requires admin override
}

// NormalizeStatus maps legacy lowercase states to canonical uppercase.
func NormalizeStatus(raw string) string {
	s := strings.ToUpper(strings.TrimSpace(raw))
	switch s {
	case "OPEN", "TRIAGED", "ACTIVE", "CONTAINED", "REMEDIATING", "RESOLVED", "ARCHIVED":
		return s
	case "TRIAGING":
		return StatusTriaged
	case "CLOSED":
		return StatusResolved
	case "CONTAINED_LEGACY":
		return StatusContained
	default:
		if s == "" {
			return StatusOpen
		}
		return s
	}
}

// CanTransition reports whether from -> to is allowed. Admin override skips validation when override=true.
func CanTransition(from, to string, adminOverride bool) bool {
	from = NormalizeStatus(from)
	to = NormalizeStatus(to)
	if from == to {
		return true
	}
	if adminOverride {
		return true
	}
	next, ok := allowedTransitions[from]
	if !ok {
		return false
	}
	for _, n := range next {
		if n == to {
			return true
		}
	}
	return false
}

// ValidateTransition returns an error if the transition is not permitted.
func ValidateTransition(from, to string, adminOverride bool) error {
	if CanTransition(from, to, adminOverride) {
		return nil
	}
	return fmt.Errorf("invalid status transition: %s -> %s", from, to)
}

// IsTerminalOpen returns false for resolved/archived (used in stats).
func IsTerminalOpen(status string) bool {
	s := NormalizeStatus(status)
	return s != StatusResolved && s != StatusArchived
}
