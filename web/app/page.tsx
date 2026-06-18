"use client";

import Link from "next/link";
import { useAuth } from "@/lib/auth";

/**
 * Landing Page — the marketing front door.
 *
 * Shows what the product does and has CTA buttons to sign up / log in.
 */

const features = [
  {
    title: "Unified API",
    desc: "One endpoint for OpenAI, Anthropic, and more. Switch models with one parameter.",
    icon: "⚡",
  },
  {
    title: "Streaming Support",
    desc: "Server-Sent Events for real-time token-by-token output. No more waiting.",
    icon: "🌊",
  },
  {
    title: "Built-in Billing",
    desc: "Credit-based billing with atomic deductions. Track usage down to the token.",
    icon: "💳",
  },
  {
    title: "Rate Limiting",
    desc: "Per-user rate limits protect your upstream APIs from abuse.",
    icon: "🛡️",
  },
  {
    title: "API Key Management",
    desc: "Create, list, and revoke keys. Keys are hashed — a DB breach doesn't expose them.",
    icon: "🔑",
  },
  {
    title: "Production Ready",
    desc: "PostgreSQL, Docker, graceful shutdown, structured logging. The real deal.",
    icon: "🚀",
  },
];

export default function LandingPage() {
  const { isLoggedIn } = useAuth();

  return (
    <div className="flex flex-col min-h-screen">
      {/* ─── Navbar ─── */}
      <nav className="border-b border-border px-6 py-4">
        <div className="max-w-6xl mx-auto flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg bg-accent flex items-center justify-center font-bold text-accent-foreground text-sm">
              OR
            </div>
            <span className="font-semibold text-lg">OpenRouter</span>
          </div>
          <div className="flex items-center gap-4">
            {isLoggedIn ? (
              <Link
                href="/dashboard"
                className="px-4 py-2 rounded-lg bg-accent text-accent-foreground font-medium hover:bg-accent-hover"
              >
                Dashboard
              </Link>
            ) : (
              <>
                <Link
                  href="/login"
                  className="px-4 py-2 text-foreground-muted hover:text-foreground"
                >
                  Log in
                </Link>
                <Link
                  href="/signup"
                  className="px-4 py-2 rounded-lg bg-accent text-accent-foreground font-medium hover:bg-accent-hover"
                >
                  Sign up
                </Link>
              </>
            )}
          </div>
        </div>
      </nav>

      {/* ─── Hero ─── */}
      <section className="flex-1 flex items-center justify-center px-6 py-20">
        <div className="max-w-3xl text-center">
          <div className="inline-block px-3 py-1 rounded-full border border-border text-xs text-foreground-muted mb-6">
            Open source · Go · PostgreSQL
          </div>
          <h1 className="text-5xl md:text-6xl font-bold tracking-tight mb-6">
            One API for{" "}
            <span className="text-accent">Any Model</span>
          </h1>
          <p className="text-lg text-foreground-muted mb-10 max-w-2xl mx-auto">
            A unified gateway for LLM providers. Route requests to OpenAI,
            Anthropic, and more through a single OpenAI-compatible endpoint —
            with billing, streaming, and rate limiting built in.
          </p>
          <div className="flex items-center justify-center gap-4">
            <Link
              href="/signup"
              className="px-6 py-3 rounded-lg bg-accent text-accent-foreground font-medium hover:bg-accent-hover"
            >
              Get started — $5 free credits
            </Link>
            <Link
              href="/dashboard/playground"
              className="px-6 py-3 rounded-lg border border-border hover:border-border-hover text-foreground"
            >
              Try the playground
            </Link>
          </div>

          {/* Code snippet */}
          <div className="mt-16 text-left rounded-xl border border-border bg-surface p-6 font-mono text-sm overflow-x-auto">
            <div className="text-foreground-dim mb-2"># Works with any OpenAI SDK</div>
            <div>
              <span className="text-accent">curl</span> https://api.openrouter.dev/v1/chat/completions{" "}
              <span className="text-foreground-dim">\</span>
            </div>
            <div className="pl-4">
              -H <span className="text-warning">"Authorization: Bearer sk-vh-..."</span>{" "}
              <span className="text-foreground-dim">\</span>
            </div>
            <div className="pl-4">
              -H <span className="text-warning">"Content-Type: application/json"</span>{" "}
              <span className="text-foreground-dim">\</span>
            </div>
            <div className="pl-4">
              -d{" "}
              <span className="text-warning">
                {"'"}{"{"}"model":"gpt-4o","messages":[...]{"'"}
              </span>
            </div>
          </div>
        </div>
      </section>

      {/* ─── Features Grid ─── */}
      <section className="px-6 py-20 border-t border-border">
        <div className="max-w-6xl mx-auto">
          <h2 className="text-3xl font-bold text-center mb-4">
            Everything you need to ship
          </h2>
          <p className="text-foreground-muted text-center mb-12 max-w-2xl mx-auto">
            Built with production-grade infrastructure, not a toy project.
          </p>
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {features.map((feature) => (
              <div
                key={feature.title}
                className="rounded-xl border border-border bg-surface p-6 hover:border-border-hover"
              >
                <div className="text-3xl mb-4">{feature.icon}</div>
                <h3 className="font-semibold mb-2">{feature.title}</h3>
                <p className="text-sm text-foreground-muted">{feature.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* ─── Footer ─── */}
      <footer className="border-t border-border px-6 py-8">
        <div className="max-w-6xl mx-auto text-center text-sm text-foreground-dim">
          Built with Go, Next.js, and PostgreSQL · {new Date().getFullYear()}
        </div>
      </footer>
    </div>
  );
}
