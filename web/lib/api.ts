/**
 * API Client — talks to the Go backend.
 *
 * The backend runs at http://localhost:8080.
 * This file wraps fetch() with auth headers and JSON parsing.
 *
 * Two types of auth:
 *   1. JWT  — for dashboard routes (/v1/keys)
 *   2. API key — for chat routes (/v1/chat/completions)
 */

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// ─── Token storage ───

export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("token");
}

export function setToken(token: string): void {
  localStorage.setItem("token", token);
}

export function clearToken(): void {
  localStorage.removeItem("token");
}

export function getApiKey(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("apiKey");
}

export function setApiKey(key: string): void {
  localStorage.setItem("apiKey", key);
}

// ─── Types ───

export interface User {
  id: string;
  email: string;
}

export interface APIKey {
  id: string;
  name: string;
  key_prefix: string;
  active: boolean;
  created_at: string;
}

export interface ModelInfo {
  id: string;
  provider: string;
  input_price: number;
  output_price: number;
  context_window: number;
  capabilities: string[];
}

export interface ChatMessage {
  role: string;
  content: string;
}

// ─── Auth API ───

export async function signup(email: string, password: string) {
  const res = await fetch(`${API_URL}/v1/auth/signup`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error?.message || "Signup failed");
  return data as { token: string; api_key: string; message: string };
}

export async function login(email: string, password: string) {
  const res = await fetch(`${API_URL}/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error?.message || "Login failed");
  return data as { token: string };
}

// ─── API Keys ───

export async function listKeys(): Promise<{ keys: APIKey[] }> {
  const res = await fetch(`${API_URL}/v1/keys`, {
    headers: { Authorization: `Bearer ${getToken()}` },
  });
  if (!res.ok) throw new Error("Failed to fetch keys");
  return res.json();
}

export async function createKey(name: string) {
  const res = await fetch(`${API_URL}/v1/keys`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${getToken()}`,
    },
    body: JSON.stringify({ name }),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error?.message || "Failed to create key");
  return data as {
    id: string;
    name: string;
    key: string;
    key_prefix: string;
    message: string;
  };
}

export async function deleteKey(id: string) {
  const res = await fetch(`${API_URL}/v1/keys/${id}`, {
    method: "DELETE",
    headers: { Authorization: `Bearer ${getToken()}` },
  });
  if (!res.ok) throw new Error("Failed to delete key");
  return res.json();
}

// ─── Chat (non-streaming) ───

export async function chat(
  model: string,
  messages: ChatMessage[],
  apiKey: string
) {
  const res = await fetch(`${API_URL}/v1/chat/completions`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${apiKey}`,
    },
    body: JSON.stringify({ model, messages }),
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error?.message || "Chat failed");
  return data;
}

// ─── Chat (streaming via SSE) ───
//
// This uses fetch + ReadableStream to read SSE chunks.
// The backend sends "data: {json}\n\n" for each token.
// We parse each chunk and call onChunk with the content.

export async function chatStream(
  model: string,
  messages: ChatMessage[],
  apiKey: string,
  onChunk: (content: string) => void,
  onDone: () => void,
  onError: (error: string) => void
) {
  try {
    const res = await fetch(`${API_URL}/v1/chat/completions`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${apiKey}`,
      },
      body: JSON.stringify({ model, messages, stream: true }),
    });

    if (!res.ok) {
      const data = await res.json();
      onError(data.error?.message || "Request failed");
      return;
    }

    const reader = res.body?.getReader();
    if (!reader) {
      onError("No response body");
      return;
    }

    const decoder = new TextDecoder();
    let buffer = "";

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });

      // Process complete SSE lines (separated by \n\n)
      const lines = buffer.split("\n");
      buffer = lines.pop() || ""; // keep incomplete line in buffer

      for (const line of lines) {
        if (!line.startsWith("data: ")) continue;
        const data = line.slice(6).trim();

        if (data === "[DONE]") {
          onDone();
          return;
        }

        try {
          const chunk = JSON.parse(data);
          const content = chunk.choices?.[0]?.delta?.content;
          if (content) onChunk(content);

          if (chunk.error) {
            onError(chunk.error);
            return;
          }
        } catch {
          // skip malformed chunks
        }
      }
    }

    onDone();
  } catch (err) {
    onError(err instanceof Error ? err.message : "Unknown error");
  }
}

// ─── Models (static list matching the Go registry) ───

export const MODELS: ModelInfo[] = [
  {
    id: "glm-5",
    provider: "openai",
    input_price: 0.0,
    output_price: 0.0,
    context_window: 128000,
    capabilities: ["tools", "json_mode", "vision"],
  },
  {
    id: "glm-4.5-air",
    provider: "openai",
    input_price: 0.0,
    output_price: 0.0,
    context_window: 128000,
    capabilities: ["tools", "json_mode"],
  },
  {
    id: "gpt-4o",
    provider: "openai",
    input_price: 2.5,
    output_price: 10.0,
    context_window: 128000,
    capabilities: ["tools", "json_mode", "vision"],
  },
  {
    id: "gpt-4o-mini",
    provider: "openai",
    input_price: 0.15,
    output_price: 0.6,
    context_window: 128000,
    capabilities: ["tools", "json_mode", "vision"],
  },
  {
    id: "claude-sonnet-4-20250514",
    provider: "anthropic",
    input_price: 3.0,
    output_price: 15.0,
    context_window: 200000,
    capabilities: ["tools", "json_mode", "vision"],
  },
  {
    id: "claude-haiku-4-20250414",
    provider: "anthropic",
    input_price: 0.8,
    output_price: 4.0,
    context_window: 200000,
    capabilities: ["tools", "json_mode", "vision"],
  },
];
