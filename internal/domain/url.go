package domain

import (
	"context"
	"time"
)

// URL is a shortened URL entity.
type URL struct {
	ShortCode   string
	OriginalURL string
	Clicks      int64
	CreatedAt   time.Time
}

// URLRepository persists and retrieves shortened URLs. Implementations
// must make IncrementClicksAndGet atomic (a single round trip), since
// concurrent redirects for the same short code are the common case in
// production, not the exception.
type URLRepository interface {
	// Save persists a newly created short URL.
	Save(ctx context.Context, url URL) error
	// Exists reports whether shortCode is already taken.
	Exists(ctx context.Context, shortCode string) (bool, error)
	// IncrementClicksAndGet atomically increments the click counter for
	// shortCode and returns the original URL it points to. It returns
	// ErrNotFound if shortCode does not exist.
	IncrementClicksAndGet(ctx context.Context, shortCode string) (originalURL string, err error)
}

// CodeGenerator produces candidate short codes for new URLs.
type CodeGenerator interface {
	Generate() (string, error)
}

// Clock abstracts wall-clock access so time-dependent logic (created-at
// timestamps, rate-limit windows) can be tested deterministically.
type Clock interface {
	Now() time.Time
}
