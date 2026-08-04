// Package postgres implements domain.URLRepository against a real
// Postgres database using pgx.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jcamilovallejos/linkguard/migrations"
)

// Connect opens a connection pool to Postgres and verifies it is reachable.
func Connect(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return pool, nil
}

// EnsureSchema applies the embedded schema to pool. It is safe to call on
// every startup.
func EnsureSchema(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, migrations.Schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}
