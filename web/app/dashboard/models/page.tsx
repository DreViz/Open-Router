"use client";

/**
 * Models Page
 *
 * Grid of model cards showing:
 *   - Model ID and provider badge
 *   - Input/output pricing per 1M tokens
 *   - Context window size
 *   - Capability tags (vision, tools, json_mode)
 *
 * Clicking "Try in Playground" links to the playground.
 */

import Link from "next/link";
import { MODELS } from "@/lib/api";

const providerColors: Record<string, string> = {
  openai: "bg-info/10 text-info border-info/20",
  anthropic: "bg-accent/10 text-accent border-accent/20",
};

const capabilityLabels: Record<string, string> = {
  vision: "Vision",
  tools: "Tools",
  json_mode: "JSON",
};

export default function ModelsPage() {
  // Group models by provider
  const providers = MODELS.reduce((acc, model) => {
    if (!acc[model.provider]) acc[model.provider] = [];
    acc[model.provider].push(model);
    return acc;
  }, {} as Record<string, typeof MODELS>);

  return (
    <div className="p-8 max-w-5xl">
      <h1 className="text-2xl font-bold mb-2">Models</h1>
      <p className="text-foreground-muted mb-8">
        {MODELS.length} models across {Object.keys(providers).length} providers.
        All accessible through a single API endpoint.
      </p>

      {/* Provider sections */}
      {Object.entries(providers).map(([provider, providerModels]) => (
        <div key={provider} className="mb-10">
          <div className="flex items-center gap-2 mb-4">
            <h2 className="text-lg font-semibold capitalize">{provider}</h2>
            <span className="text-sm text-foreground-muted">
              ({providerModels.length} models)
            </span>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {providerModels.map((model) => (
              <div
                key={model.id}
                className="rounded-xl border border-border bg-surface p-5 hover:border-border-hover group"
              >
                {/* Header */}
                <div className="flex items-start justify-between mb-3">
                  <div>
                    <code className="font-mono text-sm font-medium">
                      {model.id}
                    </code>
                    <div className="flex items-center gap-2 mt-2">
                      <span
                        className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs border ${
                          providerColors[model.provider] ||
                          "border-border text-foreground-muted"
                        }`}
                      >
                        {model.provider}
                      </span>
                      {model.capabilities.map((cap) => (
                        <span
                          key={cap}
                          className="inline-flex items-center px-2 py-0.5 rounded-full text-xs border border-border text-foreground-muted"
                        >
                          {capabilityLabels[cap] || cap}
                        </span>
                      ))}
                    </div>
                  </div>
                </div>

                {/* Pricing */}
                <div className="grid grid-cols-2 gap-3 my-4">
                  <div className="rounded-lg bg-background border border-border p-3">
                    <div className="text-xs text-foreground-muted mb-1">
                      Input
                    </div>
                    <div className="font-mono text-sm">
                      {model.input_price > 0
                        ? `$${model.input_price.toFixed(2)}`
                        : "Free"}
                      <span className="text-xs text-foreground-dim ml-1">
                        /1M tok
                      </span>
                    </div>
                  </div>
                  <div className="rounded-lg bg-background border border-border p-3">
                    <div className="text-xs text-foreground-muted mb-1">
                      Output
                    </div>
                    <div className="font-mono text-sm">
                      {model.output_price > 0
                        ? `$${model.output_price.toFixed(2)}`
                        : "Free"}
                      <span className="text-xs text-foreground-dim ml-1">
                        /1M tok
                      </span>
                    </div>
                  </div>
                </div>

                {/* Context window + CTA */}
                <div className="flex items-center justify-between">
                  <div className="text-sm text-foreground-muted">
                    <span className="font-mono">
                      {(model.context_window / 1000).toFixed(0)}K
                    </span>{" "}
                    context
                  </div>
                  <Link
                    href="/dashboard/playground"
                    className="text-sm text-accent hover:underline"
                  >
                    Try in Playground →
                  </Link>
                </div>
              </div>
            ))}
          </div>
        </div>
      ))}

      {/* API usage example */}
      <div className="mt-8 rounded-xl border border-border bg-surface p-6">
        <h3 className="font-semibold mb-3">Using the API</h3>
        <p className="text-sm text-foreground-muted mb-4">
          All models use the same endpoint. Just change the{" "}
          <code className="text-accent">model</code> field.
        </p>
        <div className="rounded-lg bg-background border border-border p-4 font-mono text-sm overflow-x-auto">
          <div>
            <span className="text-accent">POST</span>{" "}
            https://api.openrouter.dev/v1/chat/completions
          </div>
          <div className="mt-2 text-foreground-muted">
            Authorization: Bearer <span className="text-warning">sk-vh-...</span>
          </div>
          <div className="mt-3">
            <span className="text-foreground-dim">{"{"}</span>
          </div>
          <div className="pl-4">
            <span className="text-info">"model"</span>:{" "}
            <span className="text-warning">"{MODELS[0].id}"</span>,
          </div>
          <div className="pl-4">
            <span className="text-info">"messages"</span>: [
            <span className="text-foreground-dim">...</span>],
          </div>
          <div className="pl-4">
            <span className="text-info">"stream"</span>:{" "}
            <span className="text-accent">true</span>
          </div>
          <div>
            <span className="text-foreground-dim">{"}"}</span>
          </div>
        </div>
      </div>
    </div>
  );
}
