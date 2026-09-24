package assetmedia

import (
	"context"
	"sync"
	"time"
)

// DirectionLimiter is shared by every transfer in one WAN direction. Each
// caller reserves only its next bounded chunk, not an entire file.
type DirectionLimiter struct {
	mu                 sync.Mutex
	next               time.Time
	DayMbps, NightMbps int64
}

func (l *DirectionLimiter) Wait(ctx context.Context, n int) error {
	if l == nil || n <= 0 {
		return nil
	}
	now := time.Now()
	hour := now.UTC().Add(8 * time.Hour).Hour()
	mbps := l.DayMbps
	if hour < 8 {
		mbps = l.NightMbps
	}
	if mbps <= 0 {
		return nil
	}
	l.mu.Lock()
	start := l.next
	if start.Before(now) {
		start = now
	}
	l.next = start.Add(time.Duration(int64(n) * 8 * int64(time.Second) / (mbps * 1_000_000)))
	l.mu.Unlock()
	timer := time.NewTimer(time.Until(start))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
