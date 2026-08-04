package domain

import (
	"context"
	"time"
)

// RateLimitResult is the outcome of a rate-limit check.
type RateLimitResult struct {
	Allowed    bool
	Remaining  int64
	RetryAfter time.Duration
}

// RateLimiter reports whether a request identified by key is within its
// configured limit. Implementations must be safe to share across replicas
// of the service: two replicas checking the same key concurrently must
// never both observe "allowed" once the limit has been reached.
type RateLimiter interface {
	Allow(ctx context.Context, key string) (RateLimitResult, error)
}

// WindowCounter is the lower-level port the Sliding Window Counter
// algorithm depends on to persist per-window request counts. Keeping this
// separate from RateLimiter is what lets the algorithm itself (see
// usecase.SlidingWindowLimiter) be unit-tested with an in-memory fake,
// independent of whichever store backs it in production (Redis).
type WindowCounter interface {
	// IncrementAndPeek must increment the counter for currentWindow by one
	// and return both the resulting current-window count and the previous
	// window's count. Implementations MUST perform both operations
	// atomically in a single round trip (the Redis adapter does this via
	// one Lua EVAL script) — otherwise concurrent replicas can race
	// between reading and incrementing, letting combined traffic exceed
	// the configured limit.
	IncrementAndPeek(ctx context.Context, key string, currentWindow, previousWindow time.Time) (current int64, previous int64, err error)
}
