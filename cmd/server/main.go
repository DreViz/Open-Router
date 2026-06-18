package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dreviz/openrouter/internal/config"
	"github.com/dreviz/openrouter/internal/handler"
	"github.com/dreviz/openrouter/internal/middleware"
	"github.com/dreviz/openrouter/internal/provider/anthropic"
	"github.com/dreviz/openrouter/internal/provider/openai"
	"github.com/dreviz/openrouter/internal/registry"
	"github.com/dreviz/openrouter/internal/store"
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
	// Phase 3: Connect to PostgreSQL
	// ──────────────────────────────────────────────
	//
	// DATABASE_URL is the connection string for PostgreSQL.
	// Example: "postgres://openrouter:openrouter@localhost:5432/openrouter"
	//
	// The store connects, runs migrations, and gives us a pool of connections.
	// Every query after this reuses connections from the pool.

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required (set it in .env)")
	}
	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required (set it in .env)")
	}

	ctx := context.Background()
	db, err := store.New(ctx, cfg.DatabaseURL, "file://./migrations", logger)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// ──────────────────────────────────────────────
	// Create providers and register them (Phase 2)
	// ──────────────────────────────────────────────

	reg := registry.New(logger)

	if cfg.OpenAIAPIKey != "" {
		openaiProvider := openai.NewOpenAIProvider(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL)
		reg.Register(openaiProvider)
		logger.Info("registered openai provider", "base_url", cfg.OpenAIBaseURL)
	}

	if cfg.AnthropicAPIKey != "" {
		anthropicProvider := anthropic.NewAnthropicProvider(cfg.AnthropicAPIKey, cfg.AnthropicBaseURL)
		reg.Register(anthropicProvider)
		logger.Info("registered anthropic provider", "base_url", cfg.AnthropicBaseURL)
	}

	// ──────────────────────────────────────────────
	// Create handlers (Phase 3)
	// ──────────────────────────────────────────────
	//
	// Auth handler: signup + login (no auth needed to access these)
	authHandler := handler.NewAuthHandler(db, cfg.JWTSecret, logger)

	// Keys handler: create/list/deactivate API keys (needs JWT auth)
	keysHandler := handler.NewKeysHandler(db, logger)

	// Chat handler: proxy to LLM providers (needs API key auth + billing)
	chatHandler := handler.NewChatHandler(reg, db, logger)

	// ──────────────────────────────────────────────
	// Set up routes with middleware (Phase 3)
	// ──────────────────────────────────────────────
	//
	// Route diagram:
	//
	//	POST /v1/auth/signup          → authHandler.Signup       (no auth)
	//	POST /v1/auth/login           → authHandler.Login        (no auth)
	//	POST /v1/keys                 → keysHandler.Create       (JWT required)
	//	GET  /v1/keys                 → keysHandler.List         (JWT required)
	//	DELETE /v1/keys/{id}          → keysHandler.Deactivate   (JWT required)
	//	POST /v1/chat/completions     → chatHandler              (API key + rate limited)
	//	GET  /health                  → health check             (no auth)
	//
	// Middleware wraps a handler. The syntax is:
	//   middleware.RequireJWT(db, secret, logger)(handler)
	//
	// This returns an http.Handler that checks auth BEFORE calling the real handler.

	mux := http.NewServeMux()

	// Public routes (no auth)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","timestamp":"%s"}`, time.Now().Format(time.RFC3339))
	})

	mux.HandleFunc("POST /v1/auth/signup", authHandler.Signup)
	mux.HandleFunc("POST /v1/auth/login", authHandler.Login)

	// Protected routes (JWT auth — for dashboard/key management)
	jwtAuth := middleware.RequireJWT(db, cfg.JWTSecret, logger)
	mux.Handle("POST /v1/keys", jwtAuth(http.HandlerFunc(keysHandler.Create)))
	mux.Handle("GET /v1/keys", jwtAuth(http.HandlerFunc(keysHandler.List)))
	mux.Handle("DELETE /v1/keys/{id}", jwtAuth(http.HandlerFunc(keysHandler.Deactivate)))

	// Protected routes (API key auth — for chat completions)
	//
	// Middleware chain for chat completions:
	//   RequireAPIKey → RateLimiter → ChatHandler
	//
	// 1. RequireAPIKey checks the API key is valid and loads the user
	// 2. RateLimiter checks the user hasn't exceeded their request quota
	// 3. ChatHandler processes the actual chat request
	//
	// Rate limit: 20 requests per minute per user (free tier)
	apiKeyAuth := middleware.RequireAPIKey(db, logger)
	rateLimiter := middleware.NewRateLimiter(20, time.Minute, logger)
	mux.Handle("POST /v1/chat/completions", apiKeyAuth(rateLimiter.Middleware(chatHandler)))

	// ──────────────────────────────────────────────
	// Wrap the entire router with CORS middleware
	// ──────────────────────────────────────────────
	//
	// CORS must be the OUTERMOST middleware — it needs to intercept
	// every request, including preflight OPTIONS requests, before
	// any auth middleware runs.
	//
	// In production, replace localhost:3000 with your real frontend domain.
	corsMiddleware := middleware.CORS("http://localhost:3000", logger)
	handler := corsMiddleware(mux)

	// ──────────────────────────────────────────────
	// Start server with graceful shutdown
	// ──────────────────────────────────────────────
	//
	// Graceful shutdown means: when you press Ctrl+C:
	//   1. Stop accepting new requests
	//   2. Finish processing current requests
	//   3. Close database connections
	//   4. Exit
	//
	// Without this, pressing Ctrl+C kills the process instantly,
	// which can leave database connections open or responses half-sent.

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start listening in a goroutine
	go func() {
		logger.Info("server listening", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for interrupt signal (Ctrl+C)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server...")

	// Give outstanding requests 10 seconds to finish
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
	}

	logger.Info("server stopped")
}
