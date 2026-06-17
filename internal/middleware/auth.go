package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dreviz/openrouter/internal/store"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ──────────────────────────────────────────────────────
// Middleware sits BETWEEN the client and the handler.
//
// Request flow with middleware:
//
//	Client → RequireAPIKey middleware → ChatHandler
//	Client → RequireJWT middleware    → KeysHandler
//
// The middleware decides:
//	  "Is this request allowed? If yes, who is making it?"
//	If the answer is "no" → return 401, handler never runs
//	If the answer is "yes" → inject user into context, handler runs
//
// WHY context.Values?
//	Go's context.Context carries request-scoped data through the call chain.
//	Middleware writes user info into context, handler reads it back.
//	This avoids global variables and keeps things clean.
// ──────────────────────────────────────────────────────

// contextKey is an unexported type for context keys.
//
// Why unexported? So nobody outside this package can collide with our keys.
// If we used a string like "user", another package might also use "user"
// and overwrite our data. Using an unexported type prevents that.
type contextKey string

const userKey contextKey = "user"

// ContextUser is what we store in context after authentication.
type ContextUser struct {
	ID    string
	Email string
}

// WithUser stores user info in the context.
func WithUser(ctx context.Context, user ContextUser) context.Context {
	return context.WithValue(ctx, userKey, user)
}

// UserFromContext retrieves user info from the context.
// Returns the user and true if found, or zero value and false if not.
func UserFromContext(ctx context.Context) (ContextUser, bool) {
	user, ok := ctx.Value(userKey).(ContextUser)
	return user, ok
}

// ─── RequireJWT ──
//
// Validates JWT tokens from the Authorization header.
// Used for /v1/keys endpoints (dashboard-style actions).
//
// Flow:
//  1. Read "Authorization: Bearer <token>" header
//  2. Parse and verify the JWT signature using our secret
//  3. Extract user ID and email from claims
//  4. Load user from database (verify they still exist)
//  5. Inject user into context → call the next handler
func RequireJWT(s *store.Store, jwtSecret string, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Step 1: Extract the Bearer token
			tokenStr := extractBearerToken(r)
			if tokenStr == "" {
				http.Error(w, `{"error":{"message":"missing authorization header","type":"auth_error","code":"401"}}`, http.StatusUnauthorized)
				return
			}

			// Step 2: Parse and verify the JWT
			//
			// jwt.Parse does three things:
			//   a) Decodes the base64 header + payload
			//   b) Verifies the signature using our secret
			//   c) Checks exp (expiration) and iat (issued-at) claims
			token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
				// Verify the signing method is HMAC (HS256)
				// This prevents an attack where someone sends a token
				// signed with a different algorithm (like "none")
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(jwtSecret), nil
			})
			if err != nil {
				logger.Warn("invalid jwt", "error", err)
				http.Error(w, `{"error":{"message":"invalid or expired token","type":"auth_error","code":"401"}}`, http.StatusUnauthorized)
				return
			}

			// Step 3: Extract claims (user ID and email)
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, `{"error":{"message":"invalid token claims","type":"auth_error","code":"401"}}`, http.StatusUnauthorized)
				return
			}

			userID, _ := claims["sub"].(string)
			email, _ := claims["email"].(string)

			if userID == "" {
				http.Error(w, `{"error":{"message":"invalid token: missing subject","type":"auth_error","code":"401"}}`, http.StatusUnauthorized)
				return
			}

			// Step 4: Verify user still exists in database
			//
			// Why check the database? The JWT might be valid but:
			//   - The user might have been deleted
			//   - The user's account might be suspended
			// Always verify against the source of truth.
			_, err = s.GetUserByID(r.Context(), parseUUID(userID))
			if err != nil {
				logger.Warn("jwt user not found in db", "user_id", userID)
				http.Error(w, `{"error":{"message":"user not found","type":"auth_error","code":"401"}}`, http.StatusUnauthorized)
				return
			}

			// Step 5: Inject user into context and proceed
			ctx := WithUser(r.Context(), ContextUser{
				ID:    userID,
				Email: email,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ─── RequireAPIKey ──
//
// Validates API keys from the Authorization header.
// Used for /v1/chat/completions (programmatic access).
//
// Flow:
//  1. Read "Authorization: Bearer sk-vh-..." header
//  2. SHA256 hash the key
//  3. Look up the hash in the database
//  4. Load the user who owns this key
//  5. Check they have credits > 0
//  6. Inject user into context → call the next handler
func RequireAPIKey(s *store.Store, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Step 1: Extract the Bearer token (which is the API key)
			keyStr := extractBearerToken(r)
			if keyStr == "" {
				http.Error(w, `{"error":{"message":"missing authorization header","type":"auth_error","code":"401"}}`, http.StatusUnauthorized)
				return
			}

			// Step 2: Verify it looks like our API key format
			if !strings.HasPrefix(keyStr, "sk-vh-") {
				http.Error(w, `{"error":{"message":"invalid api key format","type":"auth_error","code":"401"}}`, http.StatusUnauthorized)
				return
			}

			// Step 3: SHA256 hash the key
			//
			// We never store the raw key — only its SHA256 hash.
			// So to look it up, we hash what the client sent.
			hash := sha256.Sum256([]byte(keyStr))
			keyHash := hex.EncodeToString(hash[:])

			// Step 4: Look up the hash in the database
			apiKey, err := s.GetAPIKeyByHash(r.Context(), keyHash)
			if err != nil {
				logger.Warn("api key not found", "key_prefix", keyStr[:12])
				http.Error(w, `{"error":{"message":"invalid api key","type":"auth_error","code":"401"}}`, http.StatusUnauthorized)
				return
			}

			// Step 5: Load the user who owns this key
			user, err := s.GetUserByID(r.Context(), apiKey.UserID)
			if err != nil {
				logger.Warn("api key user not found", "user_id", apiKey.UserID)
				http.Error(w, `{"error":{"message":"user not found","type":"auth_error","code":"401"}}`, http.StatusUnauthorized)
				return
			}

			// Step 6: Check credits
			balance, err := s.GetBalance(r.Context(), user.ID)
			if err != nil || balance <= 0 {
				http.Error(w, `{"error":{"message":"insufficient credits","type":"billing_error","code":"402"}}`, http.StatusPaymentRequired)
				return
			}

			// Step 7: Inject user into context and proceed
			ctx := WithUser(r.Context(), ContextUser{
				ID:    user.ID.String(),
				Email: user.Email,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractBearerToken reads "Authorization: Bearer <token>" from headers.
func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	// "Bearer sk-vh-abc123" → "sk-vh-abc123"
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// parseUUID parses a string UUID, returning zero UUID on failure.
func parseUUID(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil
	}
	return id
}
