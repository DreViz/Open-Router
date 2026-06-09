package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/dreviz/openrouter/internal/model"
	"github.com/dreviz/openrouter/internal/registry"
)

// ──────────────────────────────────────────────────────
// ChatHandler handles chat completion requests.
//
// Phase 1: Hardcoded proxy to OpenAI
// Phase 2: Uses the registry to find the right provider
//
// Notice what changed:
//   - No more *config.Config dependency
//   - No more *http.Client dependency
//   - Now depends on *registry.Registry instead
//
// The handler doesn't know about providers, API keys, or URLs.
// It just asks the registry "who handles this model?" and delegates.
// ──────────────────────────────────────────────────────
type ChatHandler struct {
	registry *registry.Registry
	logger   *slog.Logger
}

// NewChatHandler creates a handler with registry + logger.
func NewChatHandler(reg *registry.Registry, logger *slog.Logger) *ChatHandler {
	return &ChatHandler{
		registry: reg,
		logger:   logger,
	}
}

// ServeHTTP handles POST /v1/chat/completions.
//
// The flow is now:
//   1. Parse the incoming request
//   2. Look up the model in the registry
//   3. Convert ChatRequest → CompletionRequest (our internal format)
//   4. Call provider.Complete() — provider handles the rest
//   5. Convert CompletionResponse → ChatResponse (OpenAI format)
//   6. Return to caller
func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// ── STEP 1: Parse the incoming JSON ──
	var req model.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// ── STEP 2: Look up the model in the registry ──
	//
	// The registry tells us: "glm-5 is handled by the OpenAI provider"
	// or "claude-sonnet-4 is handled by the Anthropic provider"
	provider, modelConfig, err := h.registry.Resolve(req.Model)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// ── STEP 3: Convert ChatRequest → CompletionRequest ──
	//
	// ChatRequest is the OpenAI-compatible format the client sends.
	// CompletionRequest is our internal format that all providers accept.
	//
	// For now the conversion is mostly the same, but it gives us
	// a place to add validation, defaults, and transformations later.

	maxTokens := 0
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}
	temperature := 0.0
	if req.Temperature != nil {
		temperature = *req.Temperature
	}

	completionReq := &model.CompletionRequest{
		Model:       modelConfig.ID,     // use the registry's model ID (might differ from request)
		Messages:    req.Messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      req.Stream,
	}

	// ── STEP 4: Call the provider ──
	//
	// This is the key line. The handler doesn't know or care
	// whether this goes to OpenAI, Anthropic, or Google.
	// The provider handles all the translation and HTTP details.
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

	// ── STEP 5: Convert CompletionResponse → ChatResponse (OpenAI format) ──
	//
	// We always return OpenAI format to the client,
	// regardless of which provider we used internally.
	chatResp := model.ChatResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   resp.Model,
		Choices: resp.Choices,
		Usage:   resp.Usage,
	}

	// ── STEP 6: Send the response ──
	respBytes, err := json.Marshal(chatResp)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal response")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBytes)

	// Log what happened
	h.logger.Info("request completed",
		"model", req.Model,
		"provider", provider.Name(),
		"status", 200,
		"latency_ms", time.Since(start).Milliseconds(),
		"tokens_in", resp.Usage.PromptTokens,
		"tokens_out", resp.Usage.CompletionTokens,
	)
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
