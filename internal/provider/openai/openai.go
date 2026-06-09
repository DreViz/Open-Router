package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dreviz/openrouter/internal/model"
)

// ──────────────────────────────────────────────────────
// OpenAIProvider implements the Provider interface for
// OpenAI-compatible APIs (OpenAI, Z.AI, etc.)
//
// HOW TO READ THIS:
// This struct "implements" the Provider interface by having
// all the methods the interface requires: Name(), Complete(), Models()
//
// In Go, you don't explicitly say "implements Provider".
// If your struct has all the methods, Go automatically recognizes it.
// This is called "structural typing" or "duck typing."
// ──────────────────────────────────────────────────────

// OpenAIProvider handles requests to OpenAI-compatible APIs.
type OpenAIProvider struct {
	apiKey  string       // API key for authentication
	baseURL string       // e.g. "https://api.z.ai/api/paas/v4"
	client  *http.Client // HTTP client with timeout
}

// NewOpenAIProvider creates a new OpenAI-compatible provider.
//
// Parameters:
//   - apiKey: your API key (e.g. "f35393c0...")
//   - baseURL: the API base URL (e.g. "https://api.z.ai/api/paas/v4")
func NewOpenAIProvider(apiKey, baseURL string) *OpenAIProvider {
	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Name returns the provider identifier.
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// ──────────────────────────────────────────────────────
// Complete sends a request and returns the response.
//
// This is where the TRANSLATION happens:
//   1. Take our internal CompletionRequest
//   2. Convert it to OpenAI's JSON format
//   3. Send the HTTP request
//   4. Parse OpenAI's JSON response
//   5. Convert it back to our internal CompletionResponse
//
// The handler never sees OpenAI's format — it only works
// with our internal types. This is the adapter pattern.
// ──────────────────────────────────────────────────────
func (p *OpenAIProvider) Complete(ctx context.Context, req *model.CompletionRequest) (*model.CompletionResponse, error) {

	// ── STEP 1: Build the OpenAI-compatible request body ──
	//
	// This is the JSON that OpenAI (and Z.AI) expects.
	// We translate from our internal CompletionRequest to this format.
	//
	// openaiRequest is a local struct — we only use it here
	// to create the correct JSON shape for this specific provider.
	// Other providers (Anthropic, Google) have different request shapes.

	type openaiMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type openaiRequest struct {
		Model       string          `json:"model"`
		Messages    []openaiMessage `json:"messages"`
		MaxTokens   int             `json:"max_tokens,omitempty"`
		Temperature float64         `json:"temperature,omitempty"`
		Stream      bool            `json:"stream,omitempty"`
	}

	// Convert our internal messages to OpenAI's message format
	messages := make([]openaiMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = openaiMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Build the request body
	body := openaiRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
	}

	// ── STEP 2: Marshal to JSON ──
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal openai request: %w", err)
	}

	// ── STEP 3: Create the HTTP request ──
	url := p.baseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	// ── STEP 4: Send the request ──
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// ── STEP 5: Read the response ──
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// If the status code isn't 200, it's an error
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("provider returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// ── STEP 6: Parse the OpenAI response into our internal format ──
	//
	// We parse the JSON into OpenAI's response format,
	// then convert it to our CompletionResponse.
	// This way the handler always gets the same shape regardless of provider.

	type openaiChoice struct {
		Index        int              `json:"index"`
		Message      openaiMessage    `json:"message"`
		FinishReason string           `json:"finish_reason"`
	}

	type openaiUsage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	}

	type openaiResponse struct {
		ID      string         `json:"id"`
		Object  string         `json:"object"`
		Created int64          `json:"created"`
		Model   string         `json:"model"`
		Choices []openaiChoice `json:"choices"`
		Usage   openaiUsage    `json:"usage"`
	}

	var openaiResp openaiResponse
	if err := json.Unmarshal(respBody, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert OpenAI's response → our internal CompletionResponse
	choices := make([]model.Choice, len(openaiResp.Choices))
	for i, c := range openaiResp.Choices {
		choices[i] = model.Choice{
			Index: c.Index,
			Message: model.Message{
				Role:    c.Message.Role,
				Content: c.Message.Content,
			},
			FinishReason: c.FinishReason,
		}
	}

	return &model.CompletionResponse{
		ID:      openaiResp.ID,
		Model:   openaiResp.Model,
		Choices: choices,
		Usage: model.Usage{
			PromptTokens:     openaiResp.Usage.PromptTokens,
			CompletionTokens: openaiResp.Usage.CompletionTokens,
			TotalTokens:      openaiResp.Usage.TotalTokens,
		},
	}, nil
}

// Models returns the models available from this provider.
func (p *OpenAIProvider) Models() []model.ModelConfig {
	return []model.ModelConfig{
		{
			ID:            "glm-5",
			Provider:      "openai",
			InputPrice:    0.0,  // TODO: add real pricing
			OutputPrice:   0.0,
			ContextWindow: 128000,
			Capabilities:  []string{"tools", "json_mode", "vision"},
		},
		{
			ID:            "glm-4.5-air",
			Provider:      "openai",
			InputPrice:    0.0,
			OutputPrice:   0.0,
			ContextWindow: 128000,
			Capabilities:  []string{"tools", "json_mode"},
		},
		{
			ID:            "gpt-4o",
			Provider:      "openai",
			InputPrice:    2.50,
			OutputPrice:   10.0,
			ContextWindow: 128000,
			Capabilities:  []string{"tools", "json_mode", "vision"},
		},
		{
			ID:            "gpt-4o-mini",
			Provider:      "openai",
			InputPrice:    0.15,
			OutputPrice:   0.60,
			ContextWindow: 128000,
			Capabilities:  []string{"tools", "json_mode", "vision"},
		},
	}
}
