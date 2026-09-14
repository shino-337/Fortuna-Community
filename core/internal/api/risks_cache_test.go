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

func TestCacheKeysDoNotCollide(t *testing.T) {
	key := func(cluster, namespace, search string, since int) string {
		return BuildRisksListCacheKey(cluster, "active", "", search, "", namespace, "", since, 1, 20, 0, 0, "")
	}
	pairs := [][2]string{
		{key("a:b", "c", "", 0), key("a", "b:c", "", 0)},
		{key("", "", "", 0), key("_", "", "", 0)},
		{key("", "", "", 0), key("", "", "", 1)},
		{key("", "", "", 0), key("", "", "_", 0)},
		{BuildInsightsSummaryCacheKey("global", 0), BuildInsightsSummaryGlobalCacheKey(0)},
		{BuildInsightsSummaryCacheKey("", 0), BuildInsightsSummaryCacheKey("_", 0)},
	}
	for _, pair := range pairs {
		if pair[0] == pair[1] {
			t.Fatalf("cache key collision: %s", pair[0])
		}
	}
	if key("c1", "default", "needle", 60) != key("c1", "default", "needle", 60) {
		t.Fatal("unstable key")
	}
}
