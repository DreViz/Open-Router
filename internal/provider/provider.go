package provider

import (
	"context"

	"github.com/dreviz/openrouter/internal/model"
)

// ──────────────────────────────────────────────────────
// Provider is the core interface of Phase 2.
//
// WHAT IS AN INTERFACE IN GO?
// An interface is a contract — it says "any type that has these methods
// can be used as a Provider." It doesn't contain any code itself.
// The actual code lives in each provider's adapter file.
//
// ANALOGY:
// Think of Provider like a "power outlet" — it defines the shape of the plug.
// Each adapter (OpenAI, Anthropic, Google) is a different "plug" that fits.
// The handler only sees the outlet shape, not what's behind the wall.
//
// WHY THIS MATTERS:
// Without this interface, the handler has if/else for every provider:
//   if model == "gpt-4" { forward to OpenAI }
//   if model == "claude" { forward to Anthropic }
//   → Messy, grows forever, violates Open/Closed principle
//
// With the interface:
//   provider.Complete(ctx, req)  // one line, works for ALL providers
//   → Clean, adding providers = one new file, handler never changes
// ──────────────────────────────────────────────────────

// StreamCallback is a function the provider calls for each chunk.
//
// The handler passes this callback to provider.Stream().
// Every time the provider receives a chunk from the upstream API,
// it calls onChunk(chunk) — and the handler decides what to do with it
// (usually: write it to the HTTP response as SSE data).
//
// WHY A CALLBACK INSTEAD OF RETURNING A CHANNEL?
//   - Simpler: no goroutine management, no channel closing logic
//   - Synchronous: chunks flow in order, naturally
//   - The handler controls HOW chunks are written (to HTTP, to a buffer, to a test)
//   - If the client disconnects, the callback returns an error and the provider stops
type StreamCallback func(chunk *model.StreamChunk) error

type Provider interface {
	// Name returns the provider identifier.
		// Example: "openai", "anthropic", "google"
	Name() string

	// Complete sends a non-streaming chat completion request
	// and returns the full response.
	//
	// Parameters:
	//   - ctx: context for cancellation (if client disconnects, this cancels)
	//   - req: the normalized request (same format regardless of provider)
	//
	// Returns:
	//   - *CompletionResponse: the normalized response (same format regardless of provider)
	//   - error: nil if success, error with details if something went wrong
	Complete(ctx context.Context, req *model.CompletionRequest) (*model.CompletionResponse, error)

	// Stream sends a streaming chat completion request.
	//
	// Instead of waiting for the full response, the provider sends chunks
	// one at a time by calling onChunk() for each piece of the response.
	//
	// Parameters:
	//   - ctx: context for cancellation (client disconnect → context canceled)
	//   - req: the normalized request (Stream field should be true)
	//   - onChunk: callback called for each chunk received from the upstream API
	//
	// Returns:
	//   - *model.Usage: final token counts (for billing). May be partial if the
	//     stream was interrupted.
	//   - error: nil if the stream completed successfully
	Stream(ctx context.Context, req *model.CompletionRequest, onChunk StreamCallback) (*model.Usage, error)

	// Models returns the list of models this provider supports.
	// Used by the registry to know which provider handles which model.
	Models() []model.ModelConfig
}
