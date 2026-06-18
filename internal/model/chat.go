package model

// ─── OpenAI-Compatible Request/Response Types ───
//
// These structs define the wire format for our gateway's API.
// We expose an OpenAI-compatible surface so any client that works
// with OpenAI works with our gateway too — just swap the base URL.
//
// EXERCISE: Complete the TODO fields. Look at OpenAI's docs:
// https://platform.openai.com/docs/api-reference/chat/create
//
// Think about:
//   - Which fields are required vs optional? (optional → pointer type + omitempty)
//   - What's the zero value of each type? (0 for int, "" for string)
//   - Why use pointers for optional fields? (so we can distinguish "not set" from "set to zero")
//   - When to use omitempty? (when the zero value should be omitted from JSON)

// Message represents a single message in a chat conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is what the client sends to our gateway.
// Matches OpenAI's POST /v1/chat/completions request body.
type ChatRequest struct {
	Model       string     `json:"model"`                  // which model to use, e.g. "gpt-4o"
	Messages    []Message  `json:"messages"`               // the conversation history
	MaxTokens   *int       `json:"max_tokens,omitempty"`   // pointer: nil = not set, 0 = "set to zero"
	Temperature *float64   `json:"temperature,omitempty"`  // how creative (0.0 = boring, 1.0 = wild)
	TopP        *float64   `json:"top_p,omitempty"`        // alternative to temperature
	Stream      bool       `json:"stream,omitempty"`        // true = stream response chunk by chunk
}

// Choice represents one completion option in the response.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage tracks token consumption for billing.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResponse is what we return to the client.
// Matches OpenAI's response format exactly.
type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// ─── Error Types ───

// ErrorResponse follows OpenAI's error format.
// Your gateway should always return errors in this shape.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

// ─── Provider Interface Types (Phase 2) ───
//
// These define the internal contract between the gateway
// and any provider (OpenAI, Anthropic, Google, etc.).
// Each provider translates these to their native format.
//
// Don't implement these yet — they're here so you see the full picture.
// You'll use them starting Phase 2.

// CompletionRequest is the gateway's internal representation.
// All provider adapters translate from ChatRequest → CompletionRequest.
type CompletionRequest struct {
	Model       string
	Messages    []Message
	MaxTokens   int
	Temperature float64
	Stream      bool
}

// CompletionResponse is the normalized response from any provider.
// Each provider adapter translates their native format → CompletionResponse.
type CompletionResponse struct {
	ID      string
	Model   string
	Choices []Choice
	Usage   Usage
}

// StreamChunk represents a single SSE chunk during streaming.
//
// The last chunk in a stream may include Usage (token counts for billing).
// OpenAI sends this when stream_options.include_usage is true.
type StreamChunk struct {
	ID      string  `json:"id"`
	Object  string  `json:"object"`
	Created int64   `json:"created"`
	Model   string  `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *Usage `json:"usage,omitempty"`
}

// ─── Model Registry Types (Phase 2) ───

// ModelConfig holds pricing and capability info for a model.
type ModelConfig struct {
	ID             string   `json:"id"`              // e.g. "anthropic/claude-3.5-sonnet"
	Provider       string   `json:"provider"`        // e.g. "anthropic"
	InputPrice     float64  `json:"input_price"`     // per 1M tokens, USD
	OutputPrice    float64  `json:"output_price"`    // per 1M tokens, USD
	ContextWindow  int      `json:"context_window"`  // max tokens
	Capabilities   []string `json:"capabilities"`    // ["vision", "tools", "json_mode"]
}
