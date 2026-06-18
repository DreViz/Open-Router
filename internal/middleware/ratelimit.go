package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// ──────────────────────────────────────────────────────
// RateLimiter prevents a single user from sending too many
// requests in a short time window.
//
// WHY RATE LIMIT?
//   - Protects your server from abuse (one user spamming 1000 req/sec)
//   - Protects upstream APIs (you pay for every token)
//   - Fairness: one user can't monopolize resources
//
// ALGORITHM: Fixed Window Counter
//
//   Each user gets a "window" (e.g., 1 minute).
//   We count how many requests they've made in that window.
//   If count >= limit, we reject with 429 Too Many Requests.
//   When the window expires, the counter resets.
//
//   Example with limit=20, window=1min:
//
//     00:00 ─ user sends request #1  → count=1  → allowed
//     00:05 ─ user sends request #20 → count=20 → allowed
//     00:10 ─ user sends request #21 → count=20 → REJECTED (429)
//     01:01 ─ window expired, count resets → request allowed again
//
// IN-MEMORY (no Redis):
//   This stores counters in a Go map. It works for a single server instance.
//   If you run multiple instances behind a load balancer, each would have
//   its own counter — that's where Redis comes in (shared state).
//   For a portfolio project with one instance, this is perfect.
// ──────────────────────────────────────────────────────

// window tracks one user's request count for the current time window.
type window struct {
	count int
	start time.Time
}

// RateLimiter limits requests per user using in-memory counters.
type RateLimiter struct {
	mu       sync.Mutex             // protects the requests map
	requests map[string]*window     // user ID → their current window
	limit    int                    // max requests per window
	window   time.Duration          // window size (e.g., 1 minute)
	logger   *slog.Logger
}

// NewRateLimiter creates a rate limiter.
//
// Parameters:
//   - limit: max requests per window (e.g., 20)
//   - window: how long the window lasts (e.g., 1 * time.Minute)
func NewRateLimiter(limit int, windowDur time.Duration, logger *slog.Logger) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*window),
		limit:    limit,
		window:   windowDur,
		logger:   logger,
	}

	// Start a background goroutine to clean up expired windows.
	//
	// WHY? Without cleanup, the map grows forever as new users make requests.
	// Old users who haven't made a request in hours still have entries.
	// This goroutine runs every minute and removes stale entries.
	go rl.cleanup()

	return rl
}

// Allow checks if the user is allowed to make a request.
// Returns true if allowed, false if rate limited.
func (rl *RateLimiter) Allow(userID string) (remaining int, retryAfter time.Duration, allowed bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	w, exists := rl.requests[userID]

	// If no window exists, or the window has expired, start a fresh one
	if !exists || now.Sub(w.start) >= rl.window {
		rl.requests[userID] = &window{
			count: 1,
			start: now,
		}
		return rl.limit - 1, 0, true
	}

	// Window is still active — check if user exceeded the limit
	if w.count >= rl.limit {
		// Calculate how long until the window resets
		retryAfter = rl.window - now.Sub(w.start)
		return 0, retryAfter, false
	}

	// Still under the limit — increment and allow
	w.count++
	return rl.limit - w.count, 0, true
}

// cleanup removes expired windows every minute to prevent memory growth.
//
// This runs in a goroutine that loops forever:
//   1. Sleep for the window duration
//   2. Lock the map
//   3. Remove all entries where the window has expired
//   4. Unlock and repeat
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for userID, w := range rl.requests {
			if now.Sub(w.start) >= rl.window {
				delete(rl.requests, userID)
			}
		}
		rl.mu.Unlock()
	}
}

// ──────────────────────────────────────────────────────
// Middleware: RequireRateLimit
//
// This wraps a handler. Before the handler runs, it checks:
//   "Has this user made too many requests?"
//
// If yes → return 429 Too Many Requests (handler never runs)
// If no → increment the counter and let the handler run
//
// USAGE (in main.go):
//   rateLimiter := middleware.NewRateLimiter(20, time.Minute, logger)
//   mux.Handle("POST /v1/chat/completions",
//       apiKeyAuth(rateLimiter.Middleware(chatHandler)))
//
// The middleware chain becomes:
//   RequireAPIKey → RequireRateLimit → ChatHandler
// ──────────────────────────────────────────────────────

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get user from context (set by RequireAPIKey middleware)
		user, ok := UserFromContext(r.Context())
		if !ok {
			// No user in context — this shouldn't happen if RequireAPIKey
			// ran before us. Let the request through as a fallback.
			next.ServeHTTP(w, r)
			return
		}

		remaining, retryAfter, allowed := rl.Allow(user.ID)

		// Always set rate limit headers so clients know their quota
		//
		// X-RateLimit-Limit:     total requests allowed per window
		// X-RateLimit-Remaining: how many requests the user has left
		// Retry-After:           seconds to wait before retrying (only on 429)
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(max(remaining, 0)))

		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)

			// Match OpenAI's error format for consistency
			resp := map[string]any{
				"error": map[string]string{
					"message": "Rate limit exceeded. Too many requests.",
					"type":    "rate_limit_error",
					"code":    "429",
				},
			}
			json.NewEncoder(w).Encode(resp)

			rl.logger.Warn("rate limited",
				"user", user.Email,
				"limit", rl.limit,
				"window", rl.window,
			)
			return
		}

		next.ServeHTTP(w, r)
	})
}


