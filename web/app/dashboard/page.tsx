"use client";

/**
 * Dashboard Overview — the landing page after login.
 *
 * Shows quick stats and links to key features.
 */

import Link from "next/link";
import { getApiKey } from "@/lib/api";

export default function DashboardOverview() {
  const apiKey = typeof window !== "undefined" ? getApiKey() : null;

  return (
    <div className="p-8 max-w-5xl">
      <h1 className="text-2xl font-bold mb-2">Dashboard</h1>
      <p className="text-foreground-muted mb-8">
        Manage your API keys, test models, and track usage.
      </p>

      {/* Quick stats */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-8">
        <div className="rounded-xl border border-border bg-surface p-6">
          <div className="text-sm text-foreground-muted mb-1">Credits</div>
          <div className="text-3xl font-bold text-accent">$5.00</div>
          <div className="text-xs text-foreground-dim mt-1">Free tier</div>
        </div>
        <div className="rounded-xl border border-border bg-surface p-6">
          <div className="text-sm text-foreground-muted mb-1">Models</div>
          <div className="text-3xl font-bold">6</div>
          <div className="text-xs text-foreground-dim mt-1">2 providers</div>
        </div>
        <div className="rounded-xl border border-border bg-surface p-6">
          <div className="text-sm text-foreground-muted mb-1">Rate Limit</div>
          <div className="text-3xl font-bold">20<span className="text-base text-foreground-muted">/min</span></div>
          <div className="text-xs text-foreground-dim mt-1">Per API key</div>
        </div>
      </div>

      {/* Quick actions */}
      <h2 className="text-lg font-semibold mb-4">Quick Start</h2>
      <div className="space-y-3">
        <Link
          href="/dashboard/playground"
          className="block rounded-xl border border-border bg-surface p-5 hover:border-border-hover group"
        >
          <div className="flex items-center justify-between">
            <div>
              <div className="font-medium group-hover:text-accent">
                Try the Playground →
              </div>
              <div className="text-sm text-foreground-muted mt-1">
                Chat with any model in real-time with streaming
              </div>
            </div>
            <span className="text-2xl">💬</span>
          </div>
        </Link>

        <Link
          href="/dashboard/keys"
          className="block rounded-xl border border-border bg-surface p-5 hover:border-border-hover group"
        >
          <div className="flex items-center justify-between">
            <div>
              <div className="font-medium group-hover:text-accent">
                Manage API Keys →
              </div>
              <div className="text-sm text-foreground-muted mt-1">
                Create, view, and revoke your API keys
              </div>
            </div>
            <span className="text-2xl">🔑</span>
          </div>
        </Link>

        <Link
          href="/dashboard/models"
          className="block rounded-xl border border-border bg-surface p-5 hover:border-border-hover group"
        >
          <div className="flex items-center justify-between">
            <div>
              <div className="font-medium group-hover:text-accent">
                Browse Models →
              </div>
              <div className="text-sm text-foreground-muted mt-1">
                Compare pricing, context windows, and capabilities
              </div>
            </div>
            <span className="text-2xl">⚡</span>
          </div>
        </Link>
      </div>

      {/* Your API key (from signup) */}
      {apiKey && (
        <div className="mt-8 rounded-xl border border-border bg-surface p-5">
          <div className="text-sm font-medium mb-2">Your default API key</div>
          <div className="flex items-center gap-3">
            <code className="flex-1 rounded-lg bg-background px-4 py-2.5 text-sm font-mono text-foreground-muted overflow-x-auto">
              {apiKey.slice(0, 12)}{"***".repeat(4)}
            </code>
          </div>
          <div className="text-xs text-foreground-dim mt-2">
            This key was generated when you signed up. Create more in{" "}
            <Link href="/dashboard/keys" className="text-accent hover:underline">
              API Keys
            </Link>
            .
          </div>
        </div>
      )}
    </div>
  );
}
