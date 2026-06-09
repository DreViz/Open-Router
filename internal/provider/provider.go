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

	// Models returns the list of models this provider supports.
	// Used by the registry to know which provider handles which model.
	Models() []model.ModelConfig
}
