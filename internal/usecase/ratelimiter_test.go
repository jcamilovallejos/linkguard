package usecase_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jcamilovallejos/linkguard/internal/usecase"
)

// fakeCounter is an in-memory domain.WindowCounter used to unit-test the
// sliding window math without any real store.
type fakeCounter struct {
	mu     sync.Mutex
	counts map[string]map[time.Time]int64
}

func newFakeCounter() *fakeCounter {
	return &fakeCounter{counts: make(map[string]map[time.Time]int64)}
}

func (f *fakeCounter) seed(key string, window time.Time, count int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.counts[key] == nil {
		f.counts[key] = make(map[time.Time]int64)
	}
	f.counts[key][window] = count
}

func (f *fakeCounter) IncrementAndPeek(_ context.Context, key string, currentWindow, previousWindow time.Time) (int64, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.counts[key] == nil {
		f.counts[key] = make(map[time.Time]int64)
	}
	f.counts[key][currentWindow]++
	return f.counts[key][currentWindow], f.counts[key][previousWindow], nil
}

type fakeClock struct{ now time.Time }

func (f fakeClock) Now() time.Time { return f.now }

func TestSlidingWindowLimiter_Allow(t *testing.T) {
	const window = time.Minute
	windowStart := time.Date(2026, 1, 1, 0, 10, 0, 0, time.UTC)

	tests := []struct {
		name           string
		limit          int64
		now            time.Time
		seedPrevious   int64
		requestsBefore int
		wantAllowed    bool
	}{
		{
			name:        "first request under limit is allowed",
			limit:       10,
			now:         windowStart,
			wantAllowed: true,
		},
		{
			name:           "request beyond limit within same window is rejected",
			limit:          3,
			now:            windowStart,
			requestsBefore: 3,
			wantAllowed:    false,
		},
		{
			name:         "heavy previous window weighted near boundary still rejects",
			limit:        10,
			now:          windowStart.Add(5 * time.Second), // 5/60 elapsed into current window
			seedPrevious: 10,                               // previous window was at the limit
			wantAllowed:  false,
		},
		{
			name:         "previous window fully decayed at end of current window allows",
			limit:        10,
			now:          windowStart.Add(59 * time.Second), // almost no overlap left
			seedPrevious: 10,
			wantAllowed:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := newFakeCounter()
			clock := fakeClock{now: tt.now}
			limiter := usecase.NewSlidingWindowLimiter(counter, clock, tt.limit, window)

			currentWindow := tt.now.Truncate(window)
			previousWindow := currentWindow.Add(-window)
			if tt.seedPrevious > 0 {
				counter.seed("client-a", previousWindow, tt.seedPrevious)
			}

			for i := 0; i < tt.requestsBefore; i++ {
				_, err := limiter.Allow(context.Background(), "client-a")
				require.NoError(t, err)
			}

			result, err := limiter.Allow(context.Background(), "client-a")
			require.NoError(t, err)

			assert.Equal(t, tt.wantAllowed, result.Allowed)
			if !result.Allowed {
				assert.Zero(t, result.Remaining)
				assert.Greater(t, result.RetryAfter, time.Duration(0))
			}
		})
	}
}

func TestSlidingWindowLimiter_Allow_KeysAreIndependent(t *testing.T) {
	counter := newFakeCounter()
	clock := fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	limiter := usecase.NewSlidingWindowLimiter(counter, clock, 1, time.Minute)

	resultA, err := limiter.Allow(context.Background(), "client-a")
	require.NoError(t, err)
	assert.True(t, resultA.Allowed)

	resultB, err := limiter.Allow(context.Background(), "client-b")
	require.NoError(t, err)
	assert.True(t, resultB.Allowed, "a different key must have its own independent budget")

	resultA2, err := limiter.Allow(context.Background(), "client-a")
	require.NoError(t, err)
	assert.False(t, resultA2.Allowed, "second request for client-a exceeds its limit of 1")
}

// TestSlidingWindowLimiter_Allow_ConcurrentSameKey exercises the limiter
// from many goroutines at once (run with -race) to catch data races in the
// algorithm itself, independent of whatever store backs WindowCounter.
func TestSlidingWindowLimiter_Allow_ConcurrentSameKey(t *testing.T) {
	counter := newFakeCounter()
	clock := fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	const limit = 50
	limiter := usecase.NewSlidingWindowLimiter(counter, clock, limit, time.Minute)

	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := limiter.Allow(context.Background(), "client-a")
			require.NoError(t, err)
			if result.Allowed {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	assert.LessOrEqual(t, allowed, limit, "concurrent callers must never be allowed past the configured limit")
}
