package usecase

import (
	"context"
	"time"

	"github.com/jcamilovallejos/linkguard/internal/domain"
)

// SlidingWindowLimiter enforces a request limit over a fixed window size
// using the Sliding Window Counter algorithm: the previous window's count
// is weighted by how much of it still overlaps the current moment,
// smoothing out the hard-boundary bursts a plain fixed-window counter
// allows (e.g. limit requests at 0:59 and again at 1:00 for 2x the
// intended rate). It implements domain.RateLimiter.
type SlidingWindowLimiter struct {
	counter domain.WindowCounter
	clock   domain.Clock
	limit   int64
	window  time.Duration
}

// NewSlidingWindowLimiter builds a SlidingWindowLimiter that allows at
// most limit requests per window for any given key. counter is the only
// infrastructure dependency; in production it is backed by Redis so the
// limit is shared and correct across every replica of the service.
func NewSlidingWindowLimiter(counter domain.WindowCounter, clock domain.Clock, limit int64, window time.Duration) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{counter: counter, clock: clock, limit: limit, window: window}
}

// Allow reports whether a request identified by key is within the limit.
// A rejected request still counts against the window: this is intentional
// and prevents a client from bypassing the limit by retrying rapidly.
func (l *SlidingWindowLimiter) Allow(ctx context.Context, key string) (domain.RateLimitResult, error) {
	now := l.clock.Now()
	currentWindow := now.Truncate(l.window)
	previousWindow := currentWindow.Add(-l.window)
	elapsed := now.Sub(currentWindow)

	current, previous, err := l.counter.IncrementAndPeek(ctx, key, currentWindow, previousWindow)
	if err != nil {
		return domain.RateLimitResult{}, err
	}

	weight := float64(l.window-elapsed) / float64(l.window)
	estimated := float64(previous)*weight + float64(current)

	if estimated > float64(l.limit) {
		return domain.RateLimitResult{
			Allowed:    false,
			Remaining:  0,
			RetryAfter: l.window - elapsed,
		}, nil
	}

	remaining := l.limit - int64(estimated)
	if remaining < 0 {
		remaining = 0
	}
	return domain.RateLimitResult{Allowed: true, Remaining: remaining}, nil
}
