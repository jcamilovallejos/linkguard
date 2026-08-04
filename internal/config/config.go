// Package config reads typed runtime configuration from environment
// variables.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds the environment-derived configuration for the service.
type Config struct {
	HTTPPort    string
	BaseURL     string
	PostgresDSN string
	RedisAddr   string

	// RateLimit and RateWindow configure the Sliding Window Counter
	// applied per company (identified by API key), independently for
	// each endpoint. Deliberately conservative: this runs on a local
	// machine, not production infrastructure, so stability is prioritized
	// over raw throughput.
	RateLimit  int64
	RateWindow time.Duration
}

// Load reads the service configuration from the environment.
func Load() Config {
	return Config{
		HTTPPort:    getEnv("HTTP_PORT", "8080"),
		BaseURL:     getEnv("BASE_URL", "http://localhost:8080"),
		PostgresDSN: getEnv("POSTGRES_DSN", "postgres://linkguard:linkguard@localhost:5432/linkguard?sslmode=disable"),
		RedisAddr:   getEnv("REDIS_ADDR", "localhost:6379"),

		RateLimit:  getEnvInt("RATE_LIMIT", 100),
		RateWindow: time.Duration(getEnvInt("RATE_WINDOW_SECONDS", 1)) * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}
