package anthropic

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
// AnthropicProvider implements the Provider interface for Anthropic.
//
// THIS IS WHERE THE ADAPTER PATTERN SHINES.
//
// Compare this file with the OpenAI provider. The interface is the same
// (Name, Complete, Models) but the HTTP request format is DIFFERENT:
//
//   OpenAI format:
//     {"model": "gpt-4o", "messages": [{"role": "user", "content": "Hi"}]}
//
//   Anthropic format:
//     {"model": "claude-3-5-sonnet-20241022", "max_tokens": 1024,
//      "messages": [{"role": "user", "content": "Hi"}]}
//
// Key differences:
//   1. Anthropic requires max_tokens (it's not optional)
//   2. Anthropic uses "system" as a top-level field, not a message
//   3. Anthropic uses x-api-key header instead of Authorization: Bearer
//   4. Anthropic requires anthropic-version header
//
// Our handler doesn't need to know ANY of this.
// It just calls provider.Complete() and gets back a CompletionResponse.
// ──────────────────────────────────────────────────────

// AnthropicProvider handles requests to Anthropic's API.
type AnthropicProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(apiKey, baseURL string) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Name returns the provider identifier.
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// Complete sends a request to Anthropic and returns the response.
func (p *AnthropicProvider) Complete(ctx context.Context, req *model.CompletionRequest) (*model.CompletionResponse, error) {

	// ── STEP 1: Build Anthropic's request format ──
	//
	// Anthropic's API has a different shape than OpenAI.
	// Notice: max_tokens is REQUIRED (not optional like OpenAI).
	// System message goes in a separate "system" field, not in messages.

	type anthropicMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type anthropicRequest struct {
		Model     string             `json:"model"`
		MaxTokens int                `json:"max_tokens"`              // REQUIRED by Anthropic
		Messages  []anthropicMessage `json:"messages"`
		System    string             `json:"system,omitempty"`        // System prompt (separate from messages)
	}

	// Separate system messages from regular messages
	var systemPrompt string
	var messages []anthropicMessage

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content // Anthropic: system is a separate field
		} else {
			messages = append(messages, anthropicMessage{
				Role:    msg.Role,
				Content: msg.Content,
			})
		}
	}

	// Default max_tokens if not set (Anthropic requires it)
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096 // sensible default
	}

	body := anthropicRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		Messages:  messages,
		System:    systemPrompt,
	}

	// ── STEP 2: Marshal to JSON ──
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal anthropic request: %w", err)
	}

	// ── STEP 3: Create the HTTP request ──
	url := p.baseURL + "/messages"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Anthropic uses different headers than OpenAI:
	//   - x-api-key instead of Authorization: Bearer
	//   - anthropic-version is required
	//   - Content-Type is the same
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
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

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// ── STEP 6: Parse Anthropic's response format ──
	//
	// Anthropic's response is also different from OpenAI:
	//
	//   OpenAI:    {"choices": [{"message": {"role": "assistant", "content": "Hi"}}]}
	//   Anthropic: {"content": [{"type": "text", "text": "Hi"}]}
	//
	// We parse Anthropic's format and convert to our internal CompletionResponse.

	type anthropicContent struct {
		Type string `json:"type"` // "text"
		Text string `json:"text"`
	}

	type anthropicUsage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	}

	type anthropicResponse struct {
		ID      string             `json:"id"`
		Model   string             `json:"model"`
		Role    string             `json:"role"`
		Content []anthropicContent `json:"content"`
		Usage   anthropicUsage     `json:"usage"`
		StopReason string          `json:"stop_reason"`
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert Anthropic's content array to a single message string
	var content string
	for _, c := range anthropicResp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	// Convert to our internal format
	return &model.CompletionResponse{
		ID:    anthropicResp.ID,
		Model: anthropicResp.Model,
		Choices: []model.Choice{
			{
				Index: 0,
				Message: model.Message{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: anthropicResp.StopReason,
			},
		},
		Usage: model.Usage{
			PromptTokens:     anthropicResp.Usage.InputTokens,
			CompletionTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:      anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		},
	}, nil
}

// Models returns the models available from Anthropic.
func (p *AnthropicProvider) Models() []model.ModelConfig {
	return []model.ModelConfig{
		{
			ID:            "claude-sonnet-4-20250514",
			Provider:      "anthropic",
			InputPrice:    3.0,
			OutputPrice:   15.0,
			ContextWindow: 200000,
			Capabilities:  []string{"tools", "json_mode", "vision"},
		},
		{
			ID:            "claude-haiku-4-20250414",
			Provider:      "anthropic",
			InputPrice:    0.80,
			OutputPrice:   4.0,
			ContextWindow: 200000,
			Capabilities:  []string{"tools", "json_mode", "vision"},
		},
	}
}
