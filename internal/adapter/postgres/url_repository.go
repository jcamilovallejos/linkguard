package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jcamilovallejos/linkguard/internal/domain"
)

// URLRepository implements domain.URLRepository against Postgres.
type URLRepository struct {
	pool *pgxpool.Pool
}

// NewURLRepository builds a URLRepository backed by pool.
func NewURLRepository(pool *pgxpool.Pool) *URLRepository {
	return &URLRepository{pool: pool}
}

// Save persists a short URL.
func (r *URLRepository) Save(ctx context.Context, url domain.URL) error {
	const query = `
		INSERT INTO urls (shortcode, original_url, created_at)
		VALUES ($1, $2, $3)
	`
	if _, err := r.pool.Exec(ctx, query, url.ShortCode, url.OriginalURL, url.CreatedAt); err != nil {
		return fmt.Errorf("save url: %w", err)
	}
	return nil
}

// Exists reports whether shortCode is already taken.
func (r *URLRepository) Exists(ctx context.Context, shortCode string) (bool, error) {
	const query = `SELECT EXISTS(SELECT 1 FROM urls WHERE shortcode = $1)`
	var exists bool
	if err := r.pool.QueryRow(ctx, query, shortCode).Scan(&exists); err != nil {
		return false, fmt.Errorf("check url exists: %w", err)
	}
	return exists, nil
}

// IncrementClicksAndGet implements domain.URLRepository. It uses a single
// UPDATE ... RETURNING so the increment and the read happen atomically:
// concurrent redirects for the same short code never lose a click count.
func (r *URLRepository) IncrementClicksAndGet(ctx context.Context, shortCode string) (string, error) {
	const query = `
		UPDATE urls
		SET clicks = clicks + 1
		WHERE shortcode = $1
		RETURNING original_url
	`
	var originalURL string
	err := r.pool.QueryRow(ctx, query, shortCode).Scan(&originalURL)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("increment clicks: %w", err)
	}
	return originalURL, nil
}
