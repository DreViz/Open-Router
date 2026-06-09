package config

import (
	"fmt"
	"os"
)

// Config holds all configuration for the gateway.
//
// This is a struct — a collection of named fields.
// Think of it like a class with only properties, no methods.
// Each field has a name and a type.
type Config struct {
	Port             string
	GinMode          string
	OpenAIAPIKey     string
	OpenAIBaseURL    string
	AnthropicAPIKey  string
	AnthropicBaseURL string
	GoogleAPIKey     string
	DatabaseURL      string
	RedisURL         string
	JWTSecret        string
}

// Load reads configuration from environment variables and returns a Config.
//
// Return values: (*Config, error)
//   - *Config: pointer to a Config struct (nil if there's an error)
//   - error:  nil if everything worked, otherwise describes what went wrong
//
// Why a pointer (*Config) instead of just Config?
//   - Pointers are cheap to pass around (just a memory address)
//   - We can return nil to signal "something went wrong"
//   - If Config had large fields, copying would be wasteful
func Load() (*Config, error) {

	// ── PORT (optional, default: "8080") ──
	//
	// os.Getenv("PORT") reads the environment variable called "PORT".
	// If it's not set, it returns an empty string "".
	// We check if it's empty and use the default.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // default port
	}

	// ── GIN_MODE (optional, default: "debug") ──
	//
	// This controls whether we log in debug mode or production mode.
	// Not strictly required — defaults to "debug".
	ginMode := os.Getenv("GIN_MODE")
	if ginMode == "" {
		ginMode = "debug"
	}

	// ── OPENAI_API_KEY (REQUIRED) ──
	//
	// This is the only required field in Phase 1.
	// If it's missing, the server can't proxy to OpenAI at all.
	// So we return an error and the server won't start.
	openAIKey := os.Getenv("OPENAI_API_KEY")
	if openAIKey == "" {
		// return nil for the config (we don't have a valid one)
		// return an error explaining what went wrong
		return nil, fmt.Errorf("OPENAI_API_KEY is required")
	}

	// ── OPENAI_BASE_URL (optional, default: OpenAI's real URL) ──
	//
	// Why is this configurable? For testing!
	// In tests, we point this to a fake server (httptest.NewServer)
	// instead of the real OpenAI. That way tests don't need real API keys.
	openAIBaseURL := os.Getenv("OPENAI_BASE_URL")
	if openAIBaseURL == "" {
		openAIBaseURL = "https://api.openai.com/v1"
	}

	// ── ANTHROPIC (optional — Phase 2) ──
	//
	// We read these now but don't fail if they're missing.
	// They'll be required later when we add Anthropic support.
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")

	anthropicBaseURL := os.Getenv("ANTHROPIC_BASE_URL")
	if anthropicBaseURL == "" {
		anthropicBaseURL = "https://api.anthropic.com/v1"
	}

	// ── GOOGLE (optional — Phase 2) ──
	googleKey := os.Getenv("GOOGLE_API_KEY")

	// ── DATABASE_URL (optional — Phase 3) ──
	databaseURL := os.Getenv("DATABASE_URL")

	// ── REDIS_URL (optional — Phase 5) ──
	redisURL := os.Getenv("REDIS_URL")

	// ── JWT_SECRET (optional — Phase 3) ──
	jwtSecret := os.Getenv("JWT_SECRET")

	// ── Build and return the Config ──
	//
	// &Config{...} creates a Config struct and returns a pointer to it.
	// The & means "address of" — it gives you a pointer, not a copy.
	// nil means "no error happened".
	return &Config{
		Port:             port,
		GinMode:          ginMode,
		OpenAIAPIKey:     openAIKey,
		OpenAIBaseURL:    openAIBaseURL,
		AnthropicAPIKey:  anthropicKey,
		AnthropicBaseURL: anthropicBaseURL,
		GoogleAPIKey:     googleKey,
		DatabaseURL:      databaseURL,
		RedisURL:         redisURL,
		JWTSecret:        jwtSecret,
	}, nil
}
