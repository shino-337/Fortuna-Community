package api

import (
	"testing"
	"time"
)

func TestMemoryRisksCache_GetSet(t *testing.T) {
	c := NewMemoryRisksCache(100 * time.Millisecond)
	defer func() { _ = c }()

	key := "risks:list:_:_:_:1:active:_:_:_:1:20:0:0"
	if _, ok := c.Get(key); ok {
		t.Error("expected miss on empty cache")
	}
	c.Set(key, []byte(`{"total":0}`), 50*time.Millisecond)
	b, ok := c.Get(key)
	if !ok {
		t.Fatal("expected hit")
	}
	if string(b) != `{"total":0}` {
		t.Errorf("got %q", string(b))
	}
	time.Sleep(60 * time.Millisecond)
	if _, ok := c.Get(key); ok {
		t.Error("expected miss after TTL")
	}
}

func TestBuildRisksListCacheKey(t *testing.T) {
	k := BuildRisksListCacheKey("c1", "active", "high", "foo", "", "", "", 60, 1, 20, 0, 0)
	if k != "risks:list:c1:_:_:60:active:high:foo:_:1:20:0:0" {
		t.Errorf("got %q", k)
	}
	k2 := BuildRisksListCacheKey("", "all", "", "", "P1", "default", "vulnerability", 0, 2, 50, 1, 0)
	if k2 != "risks:list:_:default:vulnerability:1:all:_:_:P1:2:50:1:0" {
		t.Errorf("got %q", k2)
	}
}

func TestBuildInsightsSummaryCacheKey(t *testing.T) {
	if BuildInsightsSummaryCacheKey("c1", 30) != "insights:summary:c1:30" {
		t.Error("unexpected key")
	}
	if BuildInsightsSummaryCacheKey("", 0) != "insights:summary:_:0" {
		t.Error("unexpected key for global")
	}
}

func TestBuildInsightsSummaryGlobalCacheKey(t *testing.T) {
	if BuildInsightsSummaryGlobalCacheKey(0) != "insights:summary:global:0" {
		t.Errorf("got %q", BuildInsightsSummaryGlobalCacheKey(0))
	}
	if BuildInsightsSummaryGlobalCacheKey(60) != "insights:summary:global:60" {
		t.Errorf("got %q", BuildInsightsSummaryGlobalCacheKey(60))
	}
}
