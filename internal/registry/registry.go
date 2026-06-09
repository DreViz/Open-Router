package registry

import (
	"fmt"
	"log/slog"

	"github.com/dreviz/openrouter/internal/model"
	"github.com/dreviz/openrouter/internal/provider"
)

// ──────────────────────────────────────────────────────
// Registry maps model names to the correct provider.
//
// HOW IT WORKS:
//   1. At startup, we register all providers and their models
//   2. When a request comes in for "glm-5", the registry looks it up
//   3. It returns: "glm-5 is handled by the Z.AI provider"
//   4. The handler calls provider.Complete() with the request
//
// ANALOGY:
// Think of the registry like a phone book:
//   "glm-5"       → OpenAI provider (Z.AI endpoint)
//   "claude-sonnet-4" → Anthropic provider
//   "gpt-4o"      → OpenAI provider
//
// The handler doesn't need to know WHICH provider handles which model.
// It just asks the registry and gets back a provider + pricing info.
// ──────────────────────────────────────────────────────

// entry holds a model's config AND the provider that handles it.
type entry struct {
	config   model.ModelConfig
	provider provider.Provider
}

// Registry is the lookup table: model name → provider + pricing.
type Registry struct {
	models    map[string]entry   // model ID → entry
	providers map[string]provider.Provider // provider name → provider instance
	logger    *slog.Logger
}

// New creates an empty registry.
func New(logger *slog.Logger) *Registry {
	return &Registry{
		models:    make(map[string]entry),
		providers: make(map[string]provider.Provider),
		logger:    logger,
	}
}

// Register adds a provider and all its models to the registry.
//
// This is called at startup in main.go:
//   registry.Register(openaiProvider)
//   registry.Register(anthropicProvider)
//
// It iterates through the provider's Models() list and adds
// each one to the lookup table.
func (r *Registry) Register(p provider.Provider) {
	// Store the provider
	r.providers[p.Name()] = p

	// Store each model this provider supports
	for _, m := range p.Models() {
		r.models[m.ID] = entry{
			config:   m,
			provider: p,
		}
		r.logger.Info("registered model", "model", m.ID, "provider", p.Name())
	}
}

// Resolve looks up a model name and returns its provider + config.
//
// This is called by the handler on every request:
//   provider, config, err := registry.Resolve("glm-5")
func (r *Registry) Resolve(modelID string) (provider.Provider, model.ModelConfig, error) {
	entry, ok := r.models[modelID]
	if !ok {
		// Model not found — return a clear error
		available := make([]string, 0, len(r.models))
		for id := range r.models {
			available = append(available, id)
		}
		return nil, model.ModelConfig{}, fmt.Errorf("model %q not found. Available: %v", modelID, available)
	}
	return entry.provider, entry.config, nil
}

// ListModels returns all registered models (for a /v1/models endpoint).
func (r *Registry) ListModels() []model.ModelConfig {
	models := make([]model.ModelConfig, 0, len(r.models))
	for _, entry := range r.models {
		models = append(models, entry.config)
	}
	return models
}
