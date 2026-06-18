package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/dreviz/openrouter/internal/middleware"
	"github.com/dreviz/openrouter/internal/store"
	"github.com/google/uuid"
)

// ──────────────────────────────────────────────────────
// KeysHandler manages API keys for authenticated users.
//
// These endpoints are protected by JWT auth (RequireJWT middleware).
// The middleware extracts the user from the JWT and puts it in context.
// Then our handler reads the user from context to know WHO is making the request.
//
// Endpoints:
//
//	POST   /v1/keys      → create a new API key
//	GET    /v1/keys      → list all your API keys
//	DELETE /v1/keys/{id} → deactivate an API key
// ──────────────────────────────────────────────────────

type KeysHandler struct {
	store  *store.Store
	logger *slog.Logger
}

func NewKeysHandler(s *store.Store, logger *slog.Logger) *KeysHandler {
	return &KeysHandler{
		store:  s,
		logger: logger,
	}
}

// ─── Create Key ──

type CreateKeyRequest struct {
	Name string `json:"name"` // label like "production" or "staging"
}

type CreateKeyResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Key       string `json:"key"`        // FULL key — shown only once!
	KeyPrefix string `json:"key_prefix"` // "sk-vh-a3b1..."
	Message   string `json:"message"`
}

// Create generates a new API key for the authenticated user.
func (h *KeysHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by RequireJWT middleware)
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req CreateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Name == "" {
		req.Name = "unnamed-key"
	}

	// Parse the user ID from the context
	userID, err := uuid.Parse(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	// Generate the API key (shared function in auth.go)
	keyRow, fullKey, err := generateAPIKey(h.store, r.Context(), userID, req.Name)
	if err != nil {
		h.logger.Error("failed to generate api key", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create api key")
		return
	}

	h.logger.Info("api key created", "user", user.Email, "name", req.Name)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(CreateKeyResponse{
		ID:        keyRow.ID.String(),
		Name:      req.Name,
		Key:       fullKey,
		KeyPrefix: fullKey[:12],
		Message:   "Save this key — it won't be shown again!",
	})
}

// ─── List Keys ──

type KeyInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	KeyPrefix string `json:"key_prefix"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"created_at"`
}

// List returns all API keys for the authenticated user.
func (h *KeysHandler) List(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	userID, err := uuid.Parse(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	keys, err := h.store.ListAPIKeysByUser(r.Context(), userID)
	if err != nil {
		h.logger.Error("failed to list api keys", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list keys")
		return
	}

	// Convert to response format (never expose the hash)
	result := make([]KeyInfo, 0, len(keys))
	for _, k := range keys {
		result = append(result, KeyInfo{
			ID:        k.ID.String(),
			Name:      k.Name,
			KeyPrefix: k.KeyPrefix,
			Active:    k.IsActive,
			CreatedAt: k.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string][]KeyInfo{"keys": result})
}

// ─── Delete Key ──

// Deactivate revokes an API key (sets is_active = false).
func (h *KeysHandler) Deactivate(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	// Extract key ID from URL path: /v1/keys/{id}
	// Go 1.22+ path parameters: r.PathValue("id")
	keyIDStr := r.PathValue("id")
	if keyIDStr == "" {
		writeError(w, http.StatusBadRequest, "key id is required")
		return
	}

	keyID, err := uuid.Parse(keyIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid key id format")
		return
	}

	userID, err := uuid.Parse(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "invalid user id")
		return
	}

	if err := h.store.DeactivateAPIKey(r.Context(), keyID, userID); err != nil {
		h.logger.Error("failed to deactivate key", "error", err)
		writeError(w, http.StatusNotFound, "key not found")
		return
	}

	h.logger.Info("api key deactivated", "user", user.Email, "key_id", keyIDStr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "key deactivated",
	})
}
