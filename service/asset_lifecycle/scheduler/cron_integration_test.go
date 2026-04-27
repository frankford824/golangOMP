//go:build integration

package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestCronTick_FiresFakeJob(t *testing.T) {
	c := New(context.Background(), nil)
	var count atomic.Int32
	if err := c.Add("fake", "@every 1s", func(context.Context) error {
		count.Add(1)
		return nil
	}); err != nil {
		t.Fatalf("add fake job: %v", err)
	}
	c.Start()
	time.Sleep(2500 * time.Millisecond)
	if got := count.Load(); got < 2 {
		t.Fatalf("fake job count = %d, want >= 2", got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Stop(ctx); err != nil {
		t.Fatalf("stop cron: %v", err)
	}
	stoppedAt := count.Load()
	time.Sleep(1200 * time.Millisecond)
	if got := count.Load(); got != stoppedAt {
		t.Fatalf("fake job count after stop = %d, want %d", got, stoppedAt)
	}
}
