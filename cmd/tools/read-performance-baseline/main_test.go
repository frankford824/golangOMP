package main

import (
	"testing"
	"time"
)

func TestPercentileUsesStableNearestRankFloor(t *testing.T) {
	values := []time.Duration{time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond, 4 * time.Millisecond, 5 * time.Millisecond}
	if got := percentile(values, 0.50); got != 3*time.Millisecond {
		t.Fatalf("p50 = %s", got)
	}
	if got := percentile(values, 0.95); got != 4*time.Millisecond {
		t.Fatalf("p95 = %s", got)
	}
}
