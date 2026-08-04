package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/jcamilovallejos/linkguard/internal/domain"
)

// incrementAndPeekScript atomically increments the counter for the current
// window and reads the previous window's counter in a single round trip.
// This is what keeps the Sliding Window Counter race-free across
// concurrent service replicas: without this atomicity, two replicas could
// both read stale counts before either increments, letting their combined
// traffic exceed the limit.
const incrementAndPeekScript = `
local current = redis.call("INCR", KEYS[1])
if current == 1 then
    redis.call("EXPIRE", KEYS[1], ARGV[1])
end
local previous = tonumber(redis.call("GET", KEYS[2]) or "0")
return {current, previous}
`

// WindowCounter implements domain.WindowCounter against Redis.
type WindowCounter struct {
	client *redis.Client
	script *redis.Script
	prefix string
}

// NewWindowCounter builds a WindowCounter. prefix namespaces the Redis
// keys it creates (e.g. "ratelimit:shorten", "ratelimit:resolve"), so
// limiters configured for different endpoints never share a budget.
func NewWindowCounter(client *redis.Client, prefix string) *WindowCounter {
	return &WindowCounter{
		client: client,
		script: redis.NewScript(incrementAndPeekScript),
		prefix: prefix,
	}
}

// IncrementAndPeek implements domain.WindowCounter.
func (c *WindowCounter) IncrementAndPeek(ctx context.Context, key string, currentWindow, previousWindow time.Time) (int64, int64, error) {
	currentKey := c.windowKey(key, currentWindow)
	previousKey := c.windowKey(key, previousWindow)
	// Keep the current window's key alive long enough for the *next*
	// window to still read it as "previous" before it expires.
	ttlSeconds := int(2 * currentWindow.Sub(previousWindow).Seconds())

	result, err := c.script.Run(ctx, c.client, []string{currentKey, previousKey}, ttlSeconds).Result()
	if err != nil {
		return 0, 0, fmt.Errorf("increment and peek rate limit counters: %w", err)
	}

	values, ok := result.([]interface{})
	if !ok || len(values) != 2 {
		return 0, 0, fmt.Errorf("unexpected rate limit script result: %#v", result)
	}
	current, ok := values[0].(int64)
	if !ok {
		return 0, 0, fmt.Errorf("unexpected current count type: %#v", values[0])
	}
	previous, ok := values[1].(int64)
	if !ok {
		return 0, 0, fmt.Errorf("unexpected previous count type: %#v", values[1])
	}
	return current, previous, nil
}

func (c *WindowCounter) windowKey(key string, window time.Time) string {
	return fmt.Sprintf("%s:%s:%d", c.prefix, key, window.Unix())
}

var _ domain.WindowCounter = (*WindowCounter)(nil)
