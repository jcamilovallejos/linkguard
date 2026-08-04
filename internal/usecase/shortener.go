// Package usecase implements linkguard's business logic: creating short
// URLs, resolving them back to their original target, and the sliding
// window rate-limiting algorithm that protects both operations. It depends
// only on the ports declared in package domain, so every use case here is
// unit-testable without a real Postgres or Redis instance.
package usecase

import (
	"context"
	"fmt"

	"github.com/jcamilovallejos/linkguard/internal/domain"
)

// maxGenerateAttempts bounds how many candidate short codes are tried
// before giving up on a collision. With a 7-character base62 alphabet
// (62^7 ≈ 3.5e12 possibilities), a collision on the first attempt is
// already astronomically unlikely; this bound exists to fail fast rather
// than loop indefinitely if it ever does happen.
const maxGenerateAttempts = 5

// Shortener implements the URL-shortening use cases: CreateShortURL and
// Resolve.
type Shortener struct {
	repo  domain.URLRepository
	codes domain.CodeGenerator
	clock domain.Clock
}

// NewShortener builds a Shortener from its ports.
func NewShortener(repo domain.URLRepository, codes domain.CodeGenerator, clock domain.Clock) *Shortener {
	return &Shortener{repo: repo, codes: codes, clock: clock}
}

// CreateShortURL generates a unique short code for originalURL and
// persists it.
func (s *Shortener) CreateShortURL(ctx context.Context, originalURL string) (domain.URL, error) {
	if originalURL == "" {
		return domain.URL{}, fmt.Errorf("%w: url is required", domain.ErrInvalidInput)
	}

	code, err := s.generateUniqueCode(ctx)
	if err != nil {
		return domain.URL{}, err
	}

	url := domain.URL{
		ShortCode:   code,
		OriginalURL: originalURL,
		CreatedAt:   s.clock.Now(),
	}
	if err := s.repo.Save(ctx, url); err != nil {
		return domain.URL{}, err
	}
	return url, nil
}

func (s *Shortener) generateUniqueCode(ctx context.Context) (string, error) {
	for attempt := 0; attempt < maxGenerateAttempts; attempt++ {
		candidate, err := s.codes.Generate()
		if err != nil {
			return "", err
		}
		exists, err := s.repo.Exists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%w: could not generate a unique short code after %d attempts", domain.ErrConflict, maxGenerateAttempts)
}

// Resolve returns the original URL for shortCode, atomically recording the
// click against it. It returns domain.ErrNotFound if shortCode is unknown.
func (s *Shortener) Resolve(ctx context.Context, shortCode string) (string, error) {
	return s.repo.IncrementClicksAndGet(ctx, shortCode)
}
