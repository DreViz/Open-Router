package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dreviz/openrouter/internal/model"
	"github.com/dreviz/openrouter/internal/provider"
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

// ──────────────────────────────────────────────────────
// Stream sends a streaming request to Anthropic.
//
// Anthropic's SSE format is DIFFERENT from OpenAI's:
//
// Anthropic sends events like this:
//
//	event: message_start
//	data: {"type":"message_start","message":{"usage":{"input_tokens":10,"output_tokens":0}}}
//
//	event: content_block_delta
//	data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}
//
//	event: message_delta
//	data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}
//
//	event: message_stop
//	data: {"type":"message_stop"}
//
// We convert these to OpenAI's streaming format (StreamChunk) so the
// handler always sees the same shape regardless of provider.
// ──────────────────────────────────────────────────────
func (p *AnthropicProvider) Stream(ctx context.Context, req *model.CompletionRequest, onChunk provider.StreamCallback) (*model.Usage, error) {

	// ── Build the request body (same as Complete, but stream:true) ──
	type anthropicMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type anthropicRequest struct {
		Model     string             `json:"model"`
		MaxTokens int                `json:"max_tokens"`
		Messages  []anthropicMessage `json:"messages"`
		System    string             `json:"system,omitempty"`
		Stream    bool               `json:"stream"`
	}

	// Separate system messages from regular messages
	var systemPrompt string
	var messages []anthropicMessage
	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
		} else {
			messages = append(messages, anthropicMessage{Role: msg.Role, Content: msg.Content})
		}
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	body := anthropicRequest{
		Model:     req.Model,
		MaxTokens: maxTokens,
		Messages:  messages,
		System:    systemPrompt,
		Stream:    true,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// ── Send the HTTP request ──
	url := p.baseURL + "/messages"
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// ── Read the SSE stream ──
	//
	// Anthropic sends both "event: ..." and "data: ..." lines.
	// We only care about the "data: ..." lines — they contain the JSON.
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var usage model.Usage
	var streamID string

	for scanner.Scan() {
		if ctx.Err() != nil {
			break
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		// Parse Anthropic's event format
		var event struct {
			Type         string `json:"type"`
			Delta        *struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
			Message *struct {
				ID    string `json:"id"`
				Model string `json:"model"`
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Usage *struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "message_start":
			// First event — captures the message ID and input token count
			if event.Message != nil {
				streamID = event.Message.ID
				usage.PromptTokens = event.Message.Usage.InputTokens
			}

		case "content_block_delta":
			// A text chunk — convert to OpenAI's delta format
			if event.Delta != nil && event.Delta.Type == "text_delta" {
				chunk := model.StreamChunk{
					ID:      streamID,
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   req.Model,
				}
				chunk.Choices = append(chunk.Choices, struct {
					Index        int `json:"index"`
					Delta        struct {
						Role    string `json:"role,omitempty"`
						Content string `json:"content,omitempty"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				}{
					Index: 0,
					Delta: struct {
						Role    string `json:"role,omitempty"`
						Content string `json:"content,omitempty"`
					}{
						Content: event.Delta.Text,
					},
				})

				if err := onChunk(&chunk); err != nil {
					return &usage, err
				}
			}

		case "message_delta":
			// Near the end — includes output token count
			if event.Usage != nil {
				usage.CompletionTokens = event.Usage.OutputTokens
			}

		case "message_stop":
			// Stream is done — send a final chunk with finish_reason
			stopReason := "stop"
			chunk := model.StreamChunk{
				ID:      streamID,
				Object:  "chat.completion.chunk",
				Created: time.Now().Unix(),
				Model:   req.Model,
			}
			chunk.Choices = append(chunk.Choices, struct {
				Index        int `json:"index"`
				Delta        struct {
					Role    string `json:"role,omitempty"`
					Content string `json:"content,omitempty"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			}{
				Index:        0,
				FinishReason: &stopReason,
			})
			onChunk(&chunk)
		}
	}

	if err := scanner.Err(); err != nil {
		return &usage, fmt.Errorf("stream read error: %w", err)
	}

	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return &usage, nil
}


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
