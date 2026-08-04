package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jcamilovallejos/linkguard/internal/domain"
	"github.com/jcamilovallejos/linkguard/internal/usecase"
)

type fakeRepo struct {
	byCode  map[string]domain.URL
	saveErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byCode: make(map[string]domain.URL)}
}

func (r *fakeRepo) Save(_ context.Context, url domain.URL) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.byCode[url.ShortCode] = url
	return nil
}

func (r *fakeRepo) Exists(_ context.Context, shortCode string) (bool, error) {
	_, ok := r.byCode[shortCode]
	return ok, nil
}

func (r *fakeRepo) IncrementClicksAndGet(_ context.Context, shortCode string) (string, error) {
	url, ok := r.byCode[shortCode]
	if !ok {
		return "", domain.ErrNotFound
	}
	url.Clicks++
	r.byCode[shortCode] = url
	return url.OriginalURL, nil
}

// fakeCodes returns codes from a fixed queue, so tests can force a
// collision and then a successful attempt deterministically.
type fakeCodes struct {
	codes []string
	next  int
}

func (f *fakeCodes) Generate() (string, error) {
	code := f.codes[f.next]
	if f.next < len(f.codes)-1 {
		f.next++
	}
	return code, nil
}

func TestShortener_CreateShortURL(t *testing.T) {
	repo := newFakeRepo()
	codes := &fakeCodes{codes: []string{"abc1234"}}
	clock := fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	shortener := usecase.NewShortener(repo, codes, clock)

	url, err := shortener.CreateShortURL(context.Background(), "https://example.com")
	require.NoError(t, err)
	assert.Equal(t, "abc1234", url.ShortCode)
	assert.Equal(t, "https://example.com", url.OriginalURL)
	assert.Equal(t, clock.now, url.CreatedAt)
}

func TestShortener_CreateShortURL_RejectsEmptyURL(t *testing.T) {
	shortener := usecase.NewShortener(newFakeRepo(), &fakeCodes{codes: []string{"abc1234"}}, fakeClock{})

	_, err := shortener.CreateShortURL(context.Background(), "")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
}

func TestShortener_CreateShortURL_RetriesOnCollision(t *testing.T) {
	repo := newFakeRepo()
	require.NoError(t, repo.Save(context.Background(), domain.URL{ShortCode: "taken12", OriginalURL: "https://old.example.com"}))

	codes := &fakeCodes{codes: []string{"taken12", "fresh12"}}
	shortener := usecase.NewShortener(repo, codes, fakeClock{})

	url, err := shortener.CreateShortURL(context.Background(), "https://example.com")
	require.NoError(t, err)
	assert.Equal(t, "fresh12", url.ShortCode)
}

func TestShortener_CreateShortURL_GivesUpAfterMaxAttempts(t *testing.T) {
	repo := newFakeRepo()
	require.NoError(t, repo.Save(context.Background(), domain.URL{ShortCode: "taken12", OriginalURL: "https://old.example.com"}))

	codes := &fakeCodes{codes: []string{"taken12"}} // always collides
	shortener := usecase.NewShortener(repo, codes, fakeClock{})

	_, err := shortener.CreateShortURL(context.Background(), "https://example.com")
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrConflict)
}

func TestShortener_Resolve(t *testing.T) {
	repo := newFakeRepo()
	require.NoError(t, repo.Save(context.Background(), domain.URL{ShortCode: "abc1234", OriginalURL: "https://example.com"}))
	shortener := usecase.NewShortener(repo, &fakeCodes{codes: []string{"unused"}}, fakeClock{})

	original, err := shortener.Resolve(context.Background(), "abc1234")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", original)
	assert.EqualValues(t, 1, repo.byCode["abc1234"].Clicks)
}

func TestShortener_Resolve_NotFound(t *testing.T) {
	shortener := usecase.NewShortener(newFakeRepo(), &fakeCodes{codes: []string{"unused"}}, fakeClock{})

	_, err := shortener.Resolve(context.Background(), "missing")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrNotFound))
}
