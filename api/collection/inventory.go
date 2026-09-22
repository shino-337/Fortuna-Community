// Package collection defines observation metadata, independent of runtime coverage.
package collection

import (
	"fmt"
	"strings"
	"time"
)

const Version = 1
const MaxAge = 10 * time.Minute

var InventoryKinds = []string{"pods", "serviceAccounts", "roles", "roleBindings", "clusterRoles", "clusterRoleBindings", "deployments", "replicasets"}

type Inventory struct {
	Version    int            `json:"version"`
	ID         string         `json:"id"`
	Status     string         `json:"status"`    // complete or failed; never inferred from an empty list
	Namespace  string         `json:"namespace"` // empty means all namespaces
	StartedAt  time.Time      `json:"startedAt"`
	ObservedAt    time.Time            `json:"observedAt"` // end of the multi-kind collection interval
	KindStartedAt map[string]time.Time `json:"kindStartedAt,omitempty"`
	Counts         map[string]int       `json:"counts,omitempty"`
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
			observed, ok := c.KindStartedAt[kind]
			if !ok || observed.IsZero() || observed.Before(c.StartedAt) || observed.After(c.ObservedAt) || observed.After(now.Add(time.Minute)) {
				return fmt.Errorf("invalid observation time for %s", kind)
			}
		}
	}
	return nil
}
