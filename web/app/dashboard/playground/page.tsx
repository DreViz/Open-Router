"use client";

/**
 * Chat Playground
 *
 * Interactive chat interface that:
 *   - Lets you pick a model from the dropdown
 *   - Sends messages to /v1/chat/completions with streaming
 *   - Shows tokens appearing in real-time (SSE)
 *   - Maintains conversation history
 *
 * Uses the API key stored in localStorage (from signup or keys page).
 */

import { useState, useRef, useEffect } from "react";
import { chatStream, getApiKey, MODELS, type ChatMessage } from "@/lib/api";

export default function PlaygroundPage() {
  const [model, setModel] = useState(MODELS[0].id);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState("");
  const [streaming, setStreaming] = useState(false);
  const [error, setError] = useState("");
  const [systemPrompt, setSystemPrompt] = useState("");
  const [showSystem, setShowSystem] = useState(false);

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const apiKey = typeof window !== "undefined" ? getApiKey() : null;

  // Auto-scroll to bottom when new content arrives
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  async function handleSend() {
    if (!input.trim() || streaming) return;

    if (!apiKey) {
      setError("No API key found. Create one in the Keys page.");
      return;
    }

    setError("");
    const userMessage: ChatMessage = { role: "user", content: input.trim() };

    // Build the full message list (including system prompt if set)
    const allMessages: ChatMessage[] = [];
    if (systemPrompt.trim()) {
      allMessages.push({ role: "system", content: systemPrompt.trim() });
    }
    allMessages.push(...messages, userMessage);

    // Add user message + empty assistant message to the UI
    setMessages((prev) => [...prev, userMessage, { role: "assistant", content: "" }]);
    setInput("");
    setStreaming(true);

    // Stream the response
    await chatStream(
      model,
      allMessages,
      apiKey,
      // onChunk: append content to the last (assistant) message
      (content) => {
        setMessages((prev) => {
          const updated = [...prev];
          const last = updated[updated.length - 1];
          updated[updated.length - 1] = {
            ...last,
            content: last.content + content,
          };
          return updated;
        });
      },
      // onDone: stop streaming state
      () => {
        setStreaming(false);
      },
      // onError: show error, remove empty assistant message
      (err) => {
        setError(err);
        setStreaming(false);
        setMessages((prev) => {
          const last = prev[prev.length - 1];
          if (last.role === "assistant" && last.content === "") {
            return prev.slice(0, -1);
          }
          return prev;
        });
      }
    );
  }

  function handleKeyDown(e: React.KeyboardEvent) {
    // Enter to send, Shift+Enter for newline
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }

  function clearChat() {
    setMessages([]);
    setError("");
  }

  return (
    <div className="flex flex-col h-screen">
      {/* ─── Header bar ─── */}
      <div className="border-b border-border px-6 py-4 flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold">Playground</h1>
        </div>
        <div className="flex items-center gap-3">
          {/* Model selector */}
          <select
            value={model}
            onChange={(e) => setModel(e.target.value)}
            className="rounded-lg border border-border bg-surface px-3 py-2 text-sm outline-none focus:border-accent cursor-pointer"
          >
            {MODELS.map((m) => (
              <option key={m.id} value={m.id} className="bg-surface">
                {m.id}
              </option>
            ))}
          </select>

          {/* System prompt toggle */}
          <button
            onClick={() => setShowSystem(!showSystem)}
            className={`px-3 py-2 rounded-lg border text-sm ${
              showSystem || systemPrompt
                ? "border-accent/30 text-accent bg-accent/5"
                : "border-border text-foreground-muted hover:text-foreground"
            }`}
          >
            System
          </button>

          {/* Clear button */}
          {messages.length > 0 && (
            <button
              onClick={clearChat}
              className="px-3 py-2 rounded-lg border border-border text-sm text-foreground-muted hover:text-foreground"
            >
              Clear
            </button>
          )}
        </div>
      </div>

      {/* ─── System prompt (collapsible) ─── */}
      {showSystem && (
        <div className="border-b border-border px-6 py-3">
          <textarea
            value={systemPrompt}
            onChange={(e) => setSystemPrompt(e.target.value)}
            placeholder="System prompt — set the assistant's behavior (e.g., 'You are a helpful coding assistant')"
            rows={2}
            className="w-full rounded-lg border border-border bg-surface px-4 py-2.5 text-sm outline-none focus:border-accent resize-none"
          />
        </div>
      )}

      {/* ─── Messages area ─── */}
      <div className="flex-1 overflow-y-auto px-6 py-6">
        {messages.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-center">
            <div className="text-5xl mb-4">💬</div>
            <h2 className="text-xl font-semibold mb-2">
              Start a conversation
            </h2>
            <p className="text-foreground-muted max-w-md">
              Send a message to {model}. Responses stream in real-time with
              Server-Sent Events.
            </p>
          </div>
        ) : (
          <div className="max-w-3xl mx-auto space-y-6">
            {messages.map((msg, i) => (
              <div
                key={i}
                className={`flex gap-3 ${
                  msg.role === "user" ? "flex-row-reverse" : ""
                }`}
              >
                {/* Avatar */}
                <div
                  className={`w-8 h-8 rounded-lg flex items-center justify-center text-sm font-medium shrink-0 ${
                    msg.role === "user"
                      ? "bg-info/10 text-info"
                      : "bg-accent/10 text-accent"
                  }`}
                >
                  {msg.role === "user" ? "U" : "AI"}
                </div>

                {/* Message bubble */}
                <div
                  className={`rounded-xl px-4 py-3 max-w-[80%] ${
                    msg.role === "user"
                      ? "bg-info/10"
                      : "bg-surface border border-border"
                  }`}
                >
                  <div className="text-sm whitespace-pre-wrap break-words">
                    {msg.content}
                    {streaming &&
                      i === messages.length - 1 &&
                      msg.role === "assistant" && (
                        <span className="inline-block w-2 h-4 bg-accent ml-1 animate-pulse" />
                      )}
                  </div>
                </div>
              </div>
            ))}
            <div ref={messagesEndRef} />
          </div>
        )}
      </div>

      {/* ─── Error banner ─── */}
      {error && (
        <div className="px-6 py-2">
          <div className="rounded-lg border border-danger/30 bg-danger/10 px-4 py-2 text-sm text-danger">
            {error}
          </div>
        </div>
      )}

      {/* ─── Input area ─── */}
      <div className="border-t border-border px-6 py-4">
        <div className="max-w-3xl mx-auto flex items-end gap-3">
          <textarea
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={`Message ${model}...`}
            rows={1}
            className="flex-1 rounded-xl border border-border bg-surface px-4 py-3 text-sm outline-none focus:border-accent resize-none max-h-32"
            style={{ minHeight: "48px" }}
          />
          <button
            onClick={handleSend}
            disabled={!input.trim() || streaming}
            className="rounded-xl bg-accent px-5 py-3 font-medium text-accent-foreground hover:bg-accent-hover disabled:opacity-50 shrink-0"
          >
            {streaming ? "..." : "Send"}
          </button>
        </div>
        <div className="max-w-3xl mx-auto mt-2 text-xs text-foreground-dim text-center">
          Press Enter to send · Shift+Enter for newline · Streaming via SSE
        </div>
      </div>
    </div>
  );
}
