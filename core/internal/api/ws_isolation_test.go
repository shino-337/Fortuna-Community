package api

import (
	"strings"
	"sync"
	"testing"
)

func TestRiskBroadcastDoesNotExposeForeignIDs(t *testing.T) {
	old := defaultRisksHub
	defaultRisksHub = &RisksWSHub{conns: make(map[*risksWSConn]struct{})}
	t.Cleanup(func() { defaultRisksHub = old })
	c := &risksWSConn{send: make(chan []byte, 1)}
	defaultRisksHub.Register(c)
	BroadcastRisksUpdateWithPayload(&RisksUpdatePayload{Type: "insights_updated", ChangedIDs: []string{"foreign-id"}, ChangeType: "delete"})
	msg := string(<-c.send)
	if strings.Contains(msg, "foreign-id") || msg != `{"type":"insights_updated"}` {
		t.Fatalf("unsafe broadcast: %s", msg)
	}
}

func TestPodHubConcurrentBroadcastAndDisconnect(t *testing.T) {
	hub := NewPodDetailWSHub()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			hub.Broadcast("pod-a", []byte(`{"type":"events"}`))
		}
	}()
	for i := 0; i < 500; i++ {
		c := &podDetailWSConn{send: make(chan []byte, 8)}
		hub.Register("pod-a", c)
		hub.Unregister("pod-a", c)
		close(c.send)
	}
	wg.Wait()
}
