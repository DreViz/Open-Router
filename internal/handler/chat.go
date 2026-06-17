package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/dreviz/openrouter/internal/middleware"
	"github.com/dreviz/openrouter/internal/model"
	"github.com/dreviz/openrouter/internal/registry"
	"github.com/dreviz/openrouter/internal/store"
	"github.com/google/uuid"
)

// ──────────────────────────────────────────────────────
// ChatHandler handles chat completion requests.
//
// Phase 1: Hardcoded proxy to OpenAI
// Phase 2: Uses the registry to find the right provider
// Phase 3: Adds authentication + billing
//
// What changed in Phase 3:
//   - Now depends on *store.Store (for billing)
//   - Reads user from context (set by RequireAPIKey middleware)
//   - Calculates cost after each request and deducts credits
// ──────────────────────────────────────────────────────
type ChatHandler struct {
	registry *registry.Registry
	store    *store.Store
	logger   *slog.Logger
}

// NewChatHandler creates a handler with registry + store + logger.
func NewChatHandler(reg *registry.Registry, s *store.Store, logger *slog.Logger) *ChatHandler {
	return &ChatHandler{
		registry: reg,
		store:    s,
		logger:   logger,
	}
}

// ServeHTTP handles POST /v1/chat/completions.
//
// The flow is now:
//  1. Parse the incoming request
//  2. Look up the model in the registry
//  3. Convert ChatRequest → CompletionRequest (our internal format)
//  4. Call provider.Complete() — provider handles the rest
//  5. Convert CompletionResponse → ChatResponse (OpenAI format)
//  6. Calculate cost and deduct credits
//  7. Return to caller
func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// ── Get user from context (set by RequireAPIKey middleware) ──
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	// ── STEP 1: Parse the incoming JSON ──
	var req model.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// ── STEP 2: Look up the model in the registry ──
	provider, modelConfig, err := h.registry.Resolve(req.Model)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// ── STEP 3: Convert ChatRequest → CompletionRequest ──
	maxTokens := 0
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}
	temperature := 0.0
	if req.Temperature != nil {
		temperature = *req.Temperature
	}

	completionReq := &model.CompletionRequest{
		Model:       modelConfig.ID,
		Messages:    req.Messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      req.Stream,
	}

	// ── STEP 4: Call the provider ──
	resp, err := provider.Complete(r.Context(), completionReq)
	if err != nil {
		h.logger.Error("provider request failed",
			"model", req.Model,
			"provider", provider.Name(),
			"error", err,
		)
		writeError(w, http.StatusBadGateway, "upstream error: "+err.Error())
		return
	}

	// ── STEP 5: Convert CompletionResponse → ChatResponse ──
	chatResp := model.ChatResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   resp.Model,
		Choices: resp.Choices,
		Usage:   resp.Usage,
	}

	// ── STEP 6: Calculate cost and deduct credits ──
	//
	// Pricing: ModelConfig has InputPrice and OutputPrice per 1M tokens.
	//   cost = (prompt_tokens × input_price + completion_tokens × output_price) / 1,000,000
	//
	// Example: 100 prompt tokens at $3.00/1M = $0.0003
	//          200 completion tokens at $15.00/1M = $0.003
	//          Total = $0.0033
	cost := calculateCost(resp.Usage, modelConfig)

	// Parse user ID for billing
	userID, parseErr := parseUserID(user.ID)
	if parseErr != nil {
		h.logger.Error("invalid user id in context", "error", parseErr)
	} else if err := h.store.DeductCredits(r.Context(), userID, cost); err != nil {
		// Billing failed — but we already called the provider.
		// The tokens are spent. We can't un-send them.
		// Log a warning and continue. In production, you'd flag this
		// for manual review and potentially suspend the account.
		h.logger.Warn("failed to deduct credits",
			"user_id", user.ID,
			"cost", cost,
			"error", err,
		)
	}

	// ── STEP 7: Send the response ──
	respBytes, err := json.Marshal(chatResp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal response")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBytes)

	h.logger.Info("request completed",
		"model", req.Model,
		"provider", provider.Name(),
		"user", user.Email,
		"status", 200,
		"latency_ms", time.Since(start).Milliseconds(),
		"tokens_in", resp.Usage.PromptTokens,
		"tokens_out", resp.Usage.CompletionTokens,
		"cost_usd", fmt.Sprintf("%.6f", cost),
	)
}

// calculateCost computes the USD cost for a request based on token usage.
func calculateCost(usage model.Usage, config model.ModelConfig) float64 {
	inputCost := float64(usage.PromptTokens) * config.InputPrice / 1_000_000
	outputCost := float64(usage.CompletionTokens) * config.OutputPrice / 1_000_000
	return inputCost + outputCost
}

// parseUserID converts a string user ID to uuid.UUID.
func parseUserID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// writeError sends a JSON error response in OpenAI format.
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := model.ErrorResponse{
		Error: model.ErrorDetail{
			Message: message,
			Type:    "gateway_error",
			Code:    fmt.Sprintf("%d", status),
		},
	}
	json.NewEncoder(w).Encode(resp)
}
