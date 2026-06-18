package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/dreviz/openrouter/internal/store"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ──────────────────────────────────────────────────────
// AuthHandler handles user signup and login.
//
// Two types of authentication in this system:
//   1. JWT tokens — short-lived, used for managing API keys (login/signup)
//   2. API keys — long-lived, used for making chat requests (programmatic)
//
// WHY TWO TYPES?
//   JWT = "I logged in as vaibh@example.com, let me manage my account"
//   API key = "My server code needs to call the API automatically"
//
// You wouldn't put a JWT in your server code — it expires.
// You wouldn't use an API key to log into a dashboard — it's for machines.
// ──────────────────────────────────────────────────────
type AuthHandler struct {
	store     *store.Store
	jwtSecret string
	logger    *slog.Logger
}

func NewAuthHandler(s *store.Store, jwtSecret string, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		store:     s,
		jwtSecret: jwtSecret,
		logger:    logger,
	}
}

// ─── Signup Request/Response ───

type SignupRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SignupResponse struct {
	Token   string `json:"token"`    // JWT for immediate dashboard access
	APIKey  string `json:"api_key"`  // First API key (user gets it right away)
	Message string `json:"message"`
}

// Signup creates a new user account.
//
// Flow:
//   1. Validate email + password are provided
//   2. Hash the password with bcrypt (NEVER store plaintext)
//   3. Create user in database
//   4. Give them free credits ($5.00 to start)
//   5. Generate their first API key
//   6. Return JWT + API key
func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Validate input
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	// Hash the password with bcrypt.
	//
	// bcrypt.GenerateFromPassword takes the plaintext password and produces
	// a hash like: $2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy
	//
	// Why bcrypt instead of SHA256?
	//   - bcrypt is SLOW on purpose (takes ~100ms to hash one password)
	//   - This makes brute-force attacks impractical
	//   - SHA256 is FAST (~1 microsecond) — an attacker can try billions per second
	//   - bcrypt also has a "cost" parameter that increases over time as hardware gets faster
	//
	// The second argument (10) is the cost. Higher = slower = more secure.
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
	if err != nil {
		h.logger.Error("failed to hash password", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create account")
		return
	}

	// Create user in database
	user, err := h.store.CreateUser(r.Context(), req.Email, string(hashedPassword))
	if err != nil {
		writeError(w, http.StatusConflict, "email already exists")
		return
	}

	// Give user free credits ($5.00 to start)
	if err := h.store.CreateCreditAccount(r.Context(), user.ID, 5.00); err != nil {
		h.logger.Error("failed to create credit account", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to setup account")
		return
	}

	// Generate first API key
	_, apiKey, err := generateAPIKey(h.store, r.Context(), user.ID, "default-key")
	if err != nil {
		h.logger.Error("failed to generate api key", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to setup account")
		return
	}

	// Generate JWT token
	token, err := h.generateJWT(user.ID, user.Email)
	if err != nil {
		h.logger.Error("failed to generate jwt", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to setup account")
		return
	}

	h.logger.Info("user signed up", "email", user.Email)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(SignupResponse{
		Token:   token,
		APIKey:  apiKey,
		Message: "Account created with $5.00 free credits",
	})
}

// ─── Login Request/Response ───

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

// Login verifies email + password and returns a JWT.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	// Look up user by email
	user, err := h.store.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	// Compare the password with the stored hash.
	//
	// bcrypt.CompareHashAndPassword does:
	//   1. Extracts the salt from the stored hash
	//   2. Hashes the provided password with the same salt
	//   3. Compares the results
	//
	// We return the SAME error message for wrong email and wrong password.
	// This prevents an attacker from knowing if an email exists.
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	// Generate JWT
	token, err := h.generateJWT(user.ID, user.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	h.logger.Info("user logged in", "email", user.Email)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{Token: token})
}

// ─── Helper: Generate API Key ──
//
// API Key generation flow:
//
//	1. Generate 32 random bytes (using crypto/rand — cryptographically secure)
//	2. Hex encode them → 64 character hex string
//	3. Full key = "sk-vh-" + hex string  (e.g. "sk-vh-a3b1c5d7e9f2...")
//	4. Hash = SHA256(full key)             (stored in DB for lookup)
//	5. Prefix = first 12 chars             (stored in DB for display)
//	6. Return full key to user ONCE
//
// The full key is NEVER stored anywhere. Only the hash is stored.
// When a request comes in, we hash the provided key and compare to the DB.
func generateAPIKey(s *store.Store, ctx context.Context, userID uuid.UUID, name string) (*store.APIKey, string, error) {
	// Step 1: Generate 32 random bytes
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return nil, "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Step 2: Hex encode → "a3b1c5d7e9f24a6b..."
	hexKey := hex.EncodeToString(rawBytes)

	// Step 3: Full key with prefix
	fullKey := "sk-vh-" + hexKey

	// Step 4: SHA256 hash (stored in DB)
	hash := sha256.Sum256([]byte(fullKey))
	keyHash := hex.EncodeToString(hash[:])

	// Step 5: Prefix (first 12 chars, for display)
	keyPrefix := fullKey[:12]

	// Step 6: Store hash + prefix in DB — get the row back for the ID
	keyRow, err := s.CreateAPIKey(ctx, userID, name, keyHash, keyPrefix)
	if err != nil {
		return nil, "", fmt.Errorf("failed to store api key: %w", err)
	}

	// Return the DB row (has the real ID) + the full key (shown only once)
	return keyRow, fullKey, nil
}

// ─── Helper: Generate JWT ──
//
// JWT structure: header.payload.signature
//
//	header:    {"alg": "HS256", "typ": "JWT"}           (algorithm info)
//	payload:   {"sub": "user-uuid", "email": "...", "exp": 1234567890}
//	signature: HMAC-SHA256(header + "." + payload, secret)
//
// The payload is readable by anyone (it's just base64), but
// the signature proves it was generated by us (only we know the secret).
// Nobody can tamper with the payload without invalidating the signature.
func (h *AuthHandler) generateJWT(userID uuid.UUID, email string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   userID.String(),          // subject = user ID
		"email": email,                    // include email for convenience
		"exp":   time.Now().Add(24 * time.Hour).Unix(), // expires in 24 hours
		"iat":   time.Now().Unix(),        // issued at
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.jwtSecret))
}
