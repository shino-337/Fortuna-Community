// Package collection defines observation metadata, independent of runtime coverage.
package collection

import (
	"fmt"
	"strings"
	"time"
)

const Version = 1
const MaxAge = 10 * time.Minute

// ProjectionSemanticsNonAuthoritativeDeletion means a complete collection proves
// observation/persistence of the included lists, but absence is not authoritative
// deletion evidence and may be intentionally retained in the Core projection.
const ProjectionSemanticsNonAuthoritativeDeletion = "non_authoritative_deletion"

var InventoryKinds = []string{"pods", "serviceAccounts", "roles", "roleBindings", "clusterRoles", "clusterRoleBindings", "deployments", "replicasets"}

type Inventory struct {
	Version       int                  `json:"version"`
	ID            string               `json:"id"`
	Status        string               `json:"status"`    // collection complete/failed only; never means the DB projection is fully reconciled
	Namespace     string               `json:"namespace"` // empty means all namespaces
	StartedAt     time.Time            `json:"startedAt"`
	ObservedAt    time.Time            `json:"observedAt"`              // end of the multi-kind collection interval
	KindStartedAt map[string]time.Time `json:"kindStartedAt,omitempty"` // conservative lower bound captured immediately before each List request
	Counts        map[string]int       `json:"counts,omitempty"`
}

func (c Inventory) Validate(now time.Time) error {
	if c.Version != Version || len(c.ID) < 16 || len(c.ID) > 128 || strings.TrimSpace(c.ID) != c.ID {
		return fmt.Errorf("invalid inventory collection identity/version")
	}
	if c.Status != "complete" && c.Status != "failed" {
		return fmt.Errorf("invalid inventory collection status")
	}
	if c.StartedAt.IsZero() || c.ObservedAt.Before(c.StartedAt) || c.ObservedAt.After(now.Add(time.Minute)) || c.StartedAt.Before(now.Add(-MaxAge)) {
		return fmt.Errorf("invalid or stale inventory observation interval")
	}
	if len(c.Namespace) > 253 || strings.TrimSpace(c.Namespace) != c.Namespace {
		return fmt.Errorf("invalid inventory namespace scope")
	}
	if c.Status == "complete" {
		if len(c.Counts) != len(InventoryKinds) || len(c.KindStartedAt) != len(InventoryKinds) {
			return fmt.Errorf("inventory collection metadata incomplete")
		}
		for _, kind := range InventoryKinds {
			if n, ok := c.Counts[kind]; !ok || n < 0 {
				return fmt.Errorf("invalid inventory count for %s", kind)
			}
			started, ok := c.KindStartedAt[kind]
			if !ok || started.IsZero() || started.Before(c.StartedAt) || started.After(c.ObservedAt) || started.After(now.Add(time.Minute)) {
				return fmt.Errorf("invalid List start time for %s", kind)
			}
		}
	}
	return nil
}
