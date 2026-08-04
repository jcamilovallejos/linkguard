package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	linkguardhttp "github.com/jcamilovallejos/linkguard/internal/adapter/http"
	"github.com/jcamilovallejos/linkguard/internal/domain"
	"github.com/jcamilovallejos/linkguard/internal/usecase"
)

type fakeRepo struct {
	byCode map[string]domain.URL
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{byCode: make(map[string]domain.URL)}
}

func (r *fakeRepo) Save(_ context.Context, url domain.URL) error {
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
	return url.OriginalURL, nil
}

type fixedCodes struct{ code string }

func (f fixedCodes) Generate() (string, error) { return f.code, nil }

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

// alwaysAllow and alwaysDeny are minimal domain.RateLimiter fakes so the
// HTTP layer's behavior around limiter results can be tested without a
// real Redis instance.
type alwaysAllow struct{}

func (alwaysAllow) Allow(context.Context, string) (domain.RateLimitResult, error) {
	return domain.RateLimitResult{Allowed: true, Remaining: 1}, nil
}

type alwaysDeny struct{}

func (alwaysDeny) Allow(context.Context, string) (domain.RateLimitResult, error) {
	return domain.RateLimitResult{Allowed: false, RetryAfter: 5 * time.Second}, nil
}

func newTestServer(repo *fakeRepo, shortenLimiter, resolveLimiter domain.RateLimiter) *linkguardhttp.Server {
	shortener := usecase.NewShortener(repo, fixedCodes{code: "abc1234"}, systemClock{})
	return linkguardhttp.NewServer(shortener, shortenLimiter, resolveLimiter, "http://localhost:8080")
}

func TestHandleShorten_Success(t *testing.T) {
	server := newTestServer(newFakeRepo(), alwaysAllow{}, alwaysAllow{})

	body, _ := json.Marshal(map[string]string{"url": "https://example.com"})
	req := httptest.NewRequest("POST", "/shorten", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "company-a")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	require.Equal(t, 201, rec.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "abc1234", resp["shortcode"])
	assert.Equal(t, "http://localhost:8080/abc1234", resp["short_url"])
}

func TestHandleShorten_MissingAPIKey(t *testing.T) {
	server := newTestServer(newFakeRepo(), alwaysAllow{}, alwaysAllow{})

	body, _ := json.Marshal(map[string]string{"url": "https://example.com"})
	req := httptest.NewRequest("POST", "/shorten", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	assert.Equal(t, 401, rec.Code)
}

func TestHandleShorten_RateLimited(t *testing.T) {
	server := newTestServer(newFakeRepo(), alwaysDeny{}, alwaysAllow{})

	body, _ := json.Marshal(map[string]string{"url": "https://example.com"})
	req := httptest.NewRequest("POST", "/shorten", bytes.NewReader(body))
	req.Header.Set("X-API-Key", "company-a")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	assert.Equal(t, 429, rec.Code)
	assert.Equal(t, "5", rec.Header().Get("Retry-After"))
}

func TestHandleResolve_Success(t *testing.T) {
	repo := newFakeRepo()
	require.NoError(t, repo.Save(context.Background(), domain.URL{ShortCode: "abc1234", OriginalURL: "https://example.com"}))
	server := newTestServer(repo, alwaysAllow{}, alwaysAllow{})

	req := httptest.NewRequest("GET", "/abc1234", nil)
	req.Header.Set("X-API-Key", "company-a")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	assert.Equal(t, 302, rec.Code)
	assert.Equal(t, "https://example.com", rec.Header().Get("Location"))
}

func TestHandleResolve_NotFound(t *testing.T) {
	server := newTestServer(newFakeRepo(), alwaysAllow{}, alwaysAllow{})

	req := httptest.NewRequest("GET", "/missing", nil)
	req.Header.Set("X-API-Key", "company-a")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	assert.Equal(t, 404, rec.Code)
}
