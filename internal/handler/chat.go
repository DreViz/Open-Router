package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/dreviz/openrouter/internal/middleware"
	"github.com/dreviz/openrouter/internal/model"
	"github.com/dreviz/openrouter/internal/provider"
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
// Phase 4: Adds streaming support (SSE)
//
// The handler now has two code paths:
//   - Normal:   wait for full response, return as JSON
//   - Streaming: forward tokens one at a time as SSE chunks
// ──────────────────────────────────────────────────────
type ChatHandler struct {
	registry *registry.Registry
	store    *store.Store
	logger   *slog.Logger
}

func NewChatHandler(reg *registry.Registry, s *store.Store, logger *slog.Logger) *ChatHandler {
	return &ChatHandler{
		registry: reg,
		store:    s,
		logger:   logger,
	}
}

// ServeHTTP handles POST /v1/chat/completions.
func (h *ChatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	var req model.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Resolve the provider from the registry
	//
	// We use variable name "p" instead of "provider" to avoid
	// colliding with the imported package name "provider".
	p, modelConfig, err := h.registry.Resolve(req.Model)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

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

	// ── BRANCH: streaming vs non-streaming ──
	//
	// Phase 4: if the client asked for streaming (stream: true),
	// we take a completely different code path.
	if req.Stream {
		h.handleStream(w, r, p, modelConfig, completionReq, user, start)
		return
	}

	h.handleComplete(w, r, p, modelConfig, completionReq, user, start)
}

// ──────────────────────────────────────────────────────
// handleComplete — the non-streaming path (Phase 1-3)
//
// Same flow as before: call provider.Complete, get full response, return JSON.
// ──────────────────────────────────────────────────────
func (h *ChatHandler) handleComplete(
	w http.ResponseWriter,
	r *http.Request,
	p provider.Provider,
	modelConfig model.ModelConfig,
	req *model.CompletionRequest,
	user middleware.ContextUser,
	start time.Time,
) {
	resp, err := p.Complete(r.Context(), req)
	if err != nil {
		h.logger.Error("provider request failed",
			"model", req.Model,
			"provider", p.Name(),
			"error", err,
		)
		writeError(w, http.StatusBadGateway, "upstream error: "+err.Error())
		return
	}

	chatResp := model.ChatResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   resp.Model,
		Choices: resp.Choices,
		Usage:   resp.Usage,
	}

	cost := calculateCost(resp.Usage, modelConfig)
	h.deductCredits(r.Context(), user, cost)

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
		"provider", p.Name(),
		"user", user.Email,
		"latency_ms", time.Since(start).Milliseconds(),
		"tokens_in", resp.Usage.PromptTokens,
		"tokens_out", resp.Usage.CompletionTokens,
		"cost_usd", fmt.Sprintf("%.6f", cost),
	)
}

// ──────────────────────────────────────────────────────
// handleStream — the streaming path (Phase 4)
//
// Instead of waiting for the full response, we:
//  1. Set SSE headers (tells the client "I'm going to send chunks")
//  2. Call provider.Stream() with a callback
//  3. The callback writes each chunk as "data: <json>\n\n" + flush
//  4. After streaming finishes, deduct credits based on token usage
//
// WHY FLUSH MATTERS:
// Go's HTTP response writer BUFFERS data by default (groups small writes).
// In streaming, we want each chunk sent IMMEDIATELY to the client.
// http.Flusher.Flush() forces the buffer out right now.
//
// IMPORTANT: Once we write the 200 status header, we CANNOT change it.
// If the provider errors mid-stream, we can only write an error chunk,
// not return a different HTTP status code.
// ──────────────────────────────────────────────────────
func (h *ChatHandler) handleStream(
	w http.ResponseWriter,
	r *http.Request,
	p provider.Provider,
	modelConfig model.ModelConfig,
	req *model.CompletionRequest,
	user middleware.ContextUser,
	start time.Time,
) {
	// ── Set SSE response headers ──
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	// Get the flusher if the ResponseWriter supports it
	flusher, canFlush := w.(http.Flusher)

	// ── Call provider.Stream with a callback ──
	//
	// For every chunk the provider receives, our callback:
	//   1. Marshals it to JSON
	//   2. Writes "data: <json>\n\n" (SSE format)
	//   3. Flushes to push it to the client immediately
	usage, err := p.Stream(r.Context(), req, func(chunk *model.StreamChunk) error {
		data, err := json.Marshal(chunk)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "data: %s\n\n", data)
		if canFlush {
			flusher.Flush()
		}
		return nil
	})

	// ── Handle errors mid-stream ──
	//
	// We can't change HTTP status (already sent 200).
	// We send the error as a final SSE chunk.
	if err != nil {
		h.logger.Error("stream failed",
			"model", req.Model,
			"provider", p.Name(),
			"error", err,
		)
		errorData, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintf(w, "data: %s\n\n", errorData)
		if canFlush {
			flusher.Flush()
		}
	}

	// ── Send [DONE] marker ──
	//
	// This tells the client "the stream is finished."
	fmt.Fprintf(w, "data: [DONE]\n\n")
	if canFlush {
		flusher.Flush()
	}

	// ── Deduct credits based on usage ──
	cost := calculateCost(*usage, modelConfig)
	h.deductCredits(r.Context(), user, cost)

	h.logger.Info("stream completed",
		"model", req.Model,
		"provider", p.Name(),
		"user", user.Email,
		"latency_ms", time.Since(start).Milliseconds(),
		"tokens_in", usage.PromptTokens,
		"tokens_out", usage.CompletionTokens,
		"cost_usd", fmt.Sprintf("%.6f", cost),
	)
}

// ─── Helpers ───

// deductCredits deducts cost from the user's balance.
// Errors are logged but don't fail the request — the response is already sent.
func (h *ChatHandler) deductCredits(ctx context.Context, user middleware.ContextUser, cost float64) {
	userID, err := uuid.Parse(user.ID)
	if err != nil {
		h.logger.Error("invalid user id for billing", "error", err)
		return
	}
	if err := h.store.DeductCredits(ctx, userID, cost); err != nil {
		// Billing failed but we already sent the response.
		// Log and continue — in production, flag this for review.
		h.logger.Warn("failed to deduct credits",
			"user_id", user.ID,
			"cost", cost,
			"error", err,
		)
	}
}

// calculateCost computes the USD cost based on token usage and model pricing.
func calculateCost(usage model.Usage, config model.ModelConfig) float64 {
	inputCost := float64(usage.PromptTokens) * config.InputPrice / 1_000_000
	outputCost := float64(usage.CompletionTokens) * config.OutputPrice / 1_000_000
	return inputCost + outputCost
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
