package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/dreviz/openrouter/internal/config"
	"github.com/dreviz/openrouter/internal/handler"
	"github.com/dreviz/openrouter/internal/provider/anthropic"
	"github.com/dreviz/openrouter/internal/provider/openai"
	"github.com/dreviz/openrouter/internal/registry"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	_ = godotenv.Load()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Set up logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	logger.Info("starting gateway", "port", cfg.Port)

	// ──────────────────────────────────────────────
	// NEW: Create providers and register them
	// ──────────────────────────────────────────────
	//
	// This is the Phase 2 wiring:
	//   1. Create each provider with its config
	//   2. Create the registry
	//   3. Register each provider (which adds its models)
	//   4. Pass the registry to the handler
	//
	// Adding a new provider = create it here + register it.
	// No changes to handler, no changes to registry, no changes to models.

	reg := registry.New(logger)

	// Register OpenAI/Z.AI provider (if API key is configured)
	if cfg.OpenAIAPIKey != "" {
		openaiProvider := openai.NewOpenAIProvider(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL)
		reg.Register(openaiProvider)
		logger.Info("registered openai provider", "base_url", cfg.OpenAIBaseURL)
	}

	// Register Anthropic provider (if API key is configured)
	if cfg.AnthropicAPIKey != "" {
		anthropicProvider := anthropic.NewAnthropicProvider(cfg.AnthropicAPIKey, cfg.AnthropicBaseURL)
		reg.Register(anthropicProvider)
		logger.Info("registered anthropic provider", "base_url", cfg.AnthropicBaseURL)
	}

	// Create handler with the registry
	chatHandler := handler.NewChatHandler(reg, logger)

	// Set up routes
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","timestamp":"%s"}`, time.Now().Format(time.RFC3339))
	})

	mux.HandleFunc("POST /v1/chat/completions", chatHandler.ServeHTTP)

	// Start the server
	logger.Info("server listening", "port", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, mux))
}
