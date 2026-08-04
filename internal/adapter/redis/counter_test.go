package redis_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jcamilovallejos/linkguard/internal/adapter/redis"
)

func newTestClient(t *testing.T) *goredis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	return goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
}

func TestWindowCounter_IncrementAndPeek(t *testing.T) {
	client := newTestClient(t)
	counter := redis.NewWindowCounter(client, "ratelimit:test")

	windowStart := time.Now().Truncate(time.Minute)
	previousWindow := windowStart.Add(-time.Minute)

	current, previous, err := counter.IncrementAndPeek(context.Background(), "client-a", windowStart, previousWindow)
	require.NoError(t, err)
	assert.EqualValues(t, 1, current)
	assert.EqualValues(t, 0, previous)

	current, previous, err = counter.IncrementAndPeek(context.Background(), "client-a", windowStart, previousWindow)
	require.NoError(t, err)
	assert.EqualValues(t, 2, current)
	assert.EqualValues(t, 0, previous)
}

func TestWindowCounter_IncrementAndPeek_ReadsPreviousWindow(t *testing.T) {
	client := newTestClient(t)
	counter := redis.NewWindowCounter(client, "ratelimit:test")

	firstWindow := time.Now().Truncate(time.Minute)
	secondWindow := firstWindow.Add(time.Minute)
	beforeFirst := firstWindow.Add(-time.Minute)

	for i := 0; i < 3; i++ {
		_, _, err := counter.IncrementAndPeek(context.Background(), "client-a", firstWindow, beforeFirst)
		require.NoError(t, err)
	}

	current, previous, err := counter.IncrementAndPeek(context.Background(), "client-a", secondWindow, firstWindow)
	require.NoError(t, err)
	assert.EqualValues(t, 1, current)
	assert.EqualValues(t, 3, previous, "the second window must see the first window's count as its previous")
}

// TestWindowCounter_IncrementAndPeek_Concurrent exercises the Lua script
// from many goroutines at once (run with -race) to confirm INCR truly is
// atomic across concurrent callers sharing one key — the property the
// whole distributed rate limiter depends on.
func TestWindowCounter_IncrementAndPeek_Concurrent(t *testing.T) {
	client := newTestClient(t)
	counter := redis.NewWindowCounter(client, "ratelimit:test")

	windowStart := time.Now().Truncate(time.Minute)
	previousWindow := windowStart.Add(-time.Minute)

	const calls = 100
	var wg sync.WaitGroup
	results := make(chan int64, calls)
	for i := 0; i < calls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			current, _, err := counter.IncrementAndPeek(context.Background(), "client-a", windowStart, previousWindow)
			assert.NoError(t, err)
			results <- current
		}()
	}
	wg.Wait()
	close(results)

	seen := make(map[int64]bool)
	for r := range results {
		assert.False(t, seen[r], "two concurrent callers observed the same counter value %d — INCR was not atomic", r)
		seen[r] = true
	}
	assert.Len(t, seen, calls)
}
