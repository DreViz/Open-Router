"use client";

/**
 * Dashboard Layout — wraps all /dashboard/* pages.
 *
 * Provides:
 *   - Sidebar navigation (Keys, Playground, Models)
 *   - Auth guard (redirects to /login if not logged in)
 *   - User info + logout button in the header
 */

import { useAuth } from "@/lib/auth";
import { useRouter, usePathname } from "next/navigation";
import Link from "next/link";
import { useEffect } from "react";

const navItems = [
  { href: "/dashboard", label: "Overview", icon: "▣" },
  { href: "/dashboard/keys", label: "API Keys", icon: "🔑" },
  { href: "/dashboard/playground", label: "Playground", icon: "💬" },
  { href: "/dashboard/models", label: "Models", icon: "⚡" },
];

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const { isLoggedIn, loading, logout } = useAuth();
  const router = useRouter();
  const pathname = usePathname();

  // Redirect to login if not authenticated
  useEffect(() => {
    if (!loading && !isLoggedIn) {
      router.push("/login");
    }
  }, [loading, isLoggedIn, router]);

  // Show nothing while checking auth
  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-foreground-muted">Loading...</div>
      </div>
    );
  }

  if (!isLoggedIn) return null;

  return (
    <div className="flex min-h-screen">
      {/* ─── Sidebar ─── */}
      <aside className="w-64 border-r border-border flex flex-col">
        {/* Logo */}
        <Link
          href="/dashboard"
          className="flex items-center gap-2 px-6 py-5 border-b border-border"
        >
          <div className="w-8 h-8 rounded-lg bg-accent flex items-center justify-center font-bold text-accent-foreground text-sm">
            OR
          </div>
          <span className="font-semibold">OpenRouter</span>
        </Link>

        {/* Nav items */}
        <nav className="flex-1 px-3 py-4 space-y-1">
          {navItems.map((item) => {
            const active = pathname === item.href;
            return (
              <Link
                key={item.href}
                href={item.href}
                className={`flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium ${
                  active
                    ? "bg-accent/10 text-accent border border-accent/20"
                    : "text-foreground-muted hover:text-foreground hover:bg-surface-hover"
                }`}
              >
                <span className="text-base">{item.icon}</span>
                {item.label}
              </Link>
            );
          })}
        </nav>

        {/* User section */}
        <div className="px-3 py-4 border-t border-border">
          <button
            onClick={() => {
              logout();
              router.push("/");
            }}
            className="w-full flex items-center gap-3 px-3 py-2 rounded-lg text-sm font-medium text-foreground-muted hover:text-danger hover:bg-surface-hover"
          >
            <span className="text-base">⏏</span>
            Log out
          </button>
        </div>
      </aside>

      {/* ─── Main content ─── */}
      <main className="flex-1 overflow-y-auto">{children}</main>
    </div>
  );
}
