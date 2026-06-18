package middleware

import (
	"log/slog"
	"net/http"
)

// ──────────────────────────────────────────────────────
// CORS (Cross-Origin Resource Sharing) middleware.
//
// WHY DO WE NEED THIS?
//
// Your frontend runs at http://localhost:3000 (Next.js dev server).
// Your backend runs at http://localhost:8080 (Go server).
//
// Browsers block requests between different "origins" (port counts!)
// by default — this is called the "Same-Origin Policy."
//
// CORS headers tell the browser: "It's OK, I allow requests from port 3000."
//
// Without CORS, the browser blocks every API call from your frontend:
//
//	Access to fetch at 'http://localhost:8080/...' from origin 'http://localhost:3000'
//	has been blocked by CORS policy: No 'Access-Control-Allow-Origin' header
//
// In production, you'd set this to your real domain (e.g., https://myapp.com).
// ──────────────────────────────────────────────────────

// CORS returns middleware that allows cross-origin requests.
//
// allowedOrigin is the frontend URL (e.g., "http://localhost:3000").
func CORS(allowedOrigin string, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set CORS headers on EVERY response
			//
			// Access-Control-Allow-Origin: which frontend is allowed
			// Access-Control-Allow-Methods: which HTTP methods are allowed
			// Access-Control-Allow-Headers: which request headers are allowed
			// Access-Control-Allow-Credentials: allow cookies/auth headers
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "86400") // cache preflight for 24h

			// Handle preflight requests
			//
			// Before a "real" cross-origin request, the browser sends an
			// OPTIONS request first (called a "preflight") to check if
			// the server allows the actual request.
			//
			// We respond with 204 (No Content) and the CORS headers above.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
