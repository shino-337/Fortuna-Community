package networkbucket

import (
	"testing"
	"time"
)

func TestFloorBucket5MUTC(t *testing.T) {
	// 2024-06-15 12:07:30 UTC -> bucket 12:05:00
	in := time.Date(2024, 6, 15, 12, 7, 30, 0, time.UTC)
	got := FloorBucket5MUTC(in)
	want := time.Date(2024, 6, 15, 12, 5, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if got.Location() != time.UTC {
		t.Fatalf("expected UTC, got %v", got.Location())
	}
}
