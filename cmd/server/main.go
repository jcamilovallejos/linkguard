// Command server runs linkguard: a URL shortener protected by a
// distributed, Redis-backed rate limiter.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	linkguardhttp "github.com/jcamilovallejos/linkguard/internal/adapter/http"
	"github.com/jcamilovallejos/linkguard/internal/adapter/postgres"
	"github.com/jcamilovallejos/linkguard/internal/adapter/redis"
	"github.com/jcamilovallejos/linkguard/internal/config"
	"github.com/jcamilovallejos/linkguard/internal/usecase"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()

	pool, err := postgres.Connect(ctx, cfg.PostgresDSN)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := postgres.EnsureSchema(ctx, pool); err != nil {
		return err
	}

	redisClient, err := redis.Connect(ctx, cfg.RedisAddr)
	if err != nil {
		return err
	}
	defer redisClient.Close()

	clock := usecase.SystemClock{}
	shortener := usecase.NewShortener(postgres.NewURLRepository(pool), usecase.NewRandomCodeGenerator(), clock)

	// Separate Redis key prefixes give /shorten and /{shortcode} each
	// their own independent rate-limit budget per company, so heavy
	// redirect traffic can never starve a company's ability to create
	// new short URLs, or vice versa.
	shortenLimiter := usecase.NewSlidingWindowLimiter(redis.NewWindowCounter(redisClient, "ratelimit:shorten"), clock, cfg.RateLimit, cfg.RateWindow)
	resolveLimiter := usecase.NewSlidingWindowLimiter(redis.NewWindowCounter(redisClient, "ratelimit:resolve"), clock, cfg.RateLimit, cfg.RateWindow)

	server := linkguardhttp.NewServer(shortener, shortenLimiter, resolveLimiter, cfg.BaseURL)

	httpServer := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("linkguard listening on %s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdownCtx)
}
