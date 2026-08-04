// Package http exposes linkguard's two endpoints (POST /shorten,
// GET /{shortcode}) over plain net/http, enforcing the distributed rate
// limiter in front of each.
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/jcamilovallejos/linkguard/internal/domain"
	"github.com/jcamilovallejos/linkguard/internal/usecase"
)

// apiKeyHeader identifies the calling company for rate-limiting purposes.
// It is not a real credential (there is no authentication in scope for
// this project) — it only namespaces rate-limit budgets per company so
// one abusive client cannot exhaust another's.
const apiKeyHeader = "X-API-Key"

// Server wires the shortener use case and rate limiters to HTTP handlers.
type Server struct {
	shortener      *usecase.Shortener
	shortenLimiter domain.RateLimiter
	resolveLimiter domain.RateLimiter
	baseURL        string
}

// NewServer builds a Server. shortenLimiter guards URL creation and
// resolveLimiter guards the public redirect endpoint; both are keyed by
// the caller's API key, each with an independent budget. baseURL is
// prepended to short codes when building the short_url field in
// responses (e.g. "http://localhost:8080").
func NewServer(shortener *usecase.Shortener, shortenLimiter, resolveLimiter domain.RateLimiter, baseURL string) *Server {
	return &Server{
		shortener:      shortener,
		shortenLimiter: shortenLimiter,
		resolveLimiter: resolveLimiter,
		baseURL:        baseURL,
	}
}

// Routes returns the HTTP handler serving every linkguard endpoint,
// including the OpenAPI document and its Swagger UI viewer.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("POST /shorten", s.rateLimited(s.shortenLimiter, s.handleShorten))
	mux.HandleFunc("GET /{shortcode}", s.rateLimited(s.resolveLimiter, s.handleResolve))
	mux.HandleFunc("GET /openapi.yaml", handleOpenAPISpec)
	mux.HandleFunc("GET /docs", handleSwaggerUI)
	return mux
}

func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// shortenRequest is the POST /shorten request body.
type shortenRequest struct {
	URL string `json:"url"`
}

// shortenResponse is the POST /shorten response body.
type shortenResponse struct {
	ShortCode   string `json:"shortcode"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

func (s *Server) handleShorten(w http.ResponseWriter, r *http.Request) {
	var body shortenRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}

	url, err := s.shortener.CreateShortURL(r.Context(), body.URL)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, shortenResponse{
		ShortCode:   url.ShortCode,
		ShortURL:    s.baseURL + "/" + url.ShortCode,
		OriginalURL: url.OriginalURL,
	})
}

func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	shortCode := r.PathValue("shortcode")

	originalURL, err := s.shortener.Resolve(r.Context(), shortCode)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	http.Redirect(w, r, originalURL, http.StatusFound)
}

// rateLimited wraps next so it only runs once limiter allows the request
// identified by the caller's API key. A missing API key is rejected
// outright: every request must be attributable to a company budget.
func (s *Server) rateLimited(limiter domain.RateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get(apiKeyHeader)
		if apiKey == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", apiKeyHeader+" header is required")
			return
		}

		result, err := limiter.Allow(r.Context(), apiKey)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "rate limiter error")
			return
		}
		if !result.Allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))
			writeError(w, http.StatusTooManyRequests, "rate_limited", "rate limit exceeded")
			return
		}

		next(w, r)
	}
}

func writeUseCaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "short code not found")
	case errors.Is(err, domain.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal", "internal server error")
	}
}

func writeError(w http.ResponseWriter, statusCode int, code, message string) {
	writeJSON(w, statusCode, map[string]string{"error": code, "message": message})
}

func writeJSON(w http.ResponseWriter, statusCode int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(body)
}
