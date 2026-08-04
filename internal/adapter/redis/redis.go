// Package redis implements domain.WindowCounter against a real Redis
// instance, using an atomic Lua script so concurrent service replicas
// never race on the same counters.
package redis

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Connect opens a Redis client and verifies it is reachable.
func Connect(ctx context.Context, addr string) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connect to redis: %w", err)
	}
	return client, nil
}
