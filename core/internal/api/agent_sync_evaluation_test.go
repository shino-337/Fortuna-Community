package api

import (
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
	"time"
)

func TestFullSyncEvaluationCoalescesAndRetainsPendingPass(t *testing.T) {
	var active, maximum atomic.Int32
	entered := make(chan struct{}, 3)
	release := make(chan struct{}, 3)
	trigger := coalescedEvaluation(func() {
		current := active.Add(1)
		for current > maximum.Load() && !maximum.CompareAndSwap(maximum.Load(), current) {
		}
		entered <- struct{}{}
		<-release
		active.Add(-1)
	})
	trigger()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first evaluation missing")
	}
	for i := 0; i < 200; i++ {
		trigger()
	}
	release <- struct{}{}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("pending inventory pass missing")
	}
	require.EqualValues(t, 1, maximum.Load())
	release <- struct{}{}
	require.Eventually(t, func() bool { return active.Load() == 0 }, time.Second, time.Millisecond)
	// A subsequent accepted sync still requests a pass after the burst is drained.
	trigger()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("subsequent evaluation missing")
	}
	release <- struct{}{}
	require.Eventually(t, func() bool { return active.Load() == 0 }, time.Second, time.Millisecond)
	require.EqualValues(t, 1, maximum.Load())
}
