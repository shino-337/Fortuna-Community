package graph

import (
	"testing"
	"time"

	"github.com/fortuna/core/pkg/models"
)

func TestIsNetworkReachable_DenyOverridesObserved(t *testing.T) {
	now := time.Now()
	from := models.Pod{UID: "a", RestartCount: 0}
	to := models.Pod{UID: "b", PodIP: "10.0.0.2"}
	ctx := ReachabilityContext{
		ObservedEgressIPs: map[string]time.Time{"10.0.0.2": now.Add(-1 * time.Minute)},
		DenyCache:         map[string]time.Time{"a->b": now.Add(-1 * time.Minute)},
		Now:               now,
		DenyTTL:           15 * time.Minute,
		FallbackTTL:       6 * time.Hour,
	}
	if d := isNetworkReachable(from, to, ctx); d != ReachabilityDeny {
		t.Fatalf("expected deny override, got %v", d)
	}
}

func TestIsNetworkReachable_FallbackOnlyNewNoRestart(t *testing.T) {
	now := time.Now()
	start := now.Add(-1 * time.Hour)
	from := models.Pod{UID: "a", RestartCount: 0, StartTime: models.NullTime{Time: &start}}
	to := models.Pod{UID: "b", PodIP: "10.0.0.2"}
	ctx := ReachabilityContext{
		ObservedEgressIPs: map[string]time.Time{},
		DenyCache:         map[string]time.Time{},
		Now:               now,
		DenyTTL:           15 * time.Minute,
		FallbackTTL:       6 * time.Hour,
	}
	if d := isNetworkReachable(from, to, ctx); d != ReachabilitySoftAllow {
		t.Fatalf("expected soft allow, got %v", d)
	}
	from.RestartCount = 2
	if d := isNetworkReachable(from, to, ctx); d != ReachabilityDeny {
		t.Fatalf("expected deny for restarted pod, got %v", d)
	}
}

