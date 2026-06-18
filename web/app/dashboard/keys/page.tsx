"use client";

/**
 * API Keys Page
 *
 * Features:
 *   - List all keys with prefix, name, status, creation date
 *   - Create new keys (shows full key ONCE in a modal)
 *   - Deactivate (revoke) keys
 *   - Copy key to clipboard
 */

import { useState, useEffect } from "react";
import { listKeys, createKey, deleteKey, type APIKey } from "@/lib/api";

export default function KeysPage() {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreate, setShowCreate] = useState(false);
  const [newKeyName, setNewKeyName] = useState("");
  const [newKey, setNewKey] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    fetchKeys();
  }, []);

  async function fetchKeys() {
    try {
      const data = await listKeys();
      setKeys(data.keys || []);
    } catch {
      setError("Failed to load keys");
    } finally {
      setLoading(false);
    }
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    try {
      const data = await createKey(newKeyName || "unnamed-key");
      setNewKey(data.key);
      setNewKeyName("");
      setShowCreate(false);
      fetchKeys();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create key");
    }
  }

  async function handleDelete(id: string) {
    if (!confirm("Deactivate this API key? This cannot be undone.")) return;
    try {
      await deleteKey(id);
      fetchKeys();
    } catch {
      setError("Failed to deactivate key");
    }
  }

  function copyToClipboard(text: string) {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div className="p-8 max-w-4xl">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold">API Keys</h1>
          <p className="text-foreground-muted mt-1">
            Create and manage keys for programmatic access.
          </p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="px-4 py-2 rounded-lg bg-accent text-accent-foreground font-medium hover:bg-accent-hover"
        >
          + Create Key
        </button>
      </div>

      {error && (
        <div className="mb-4 rounded-lg border border-danger/30 bg-danger/10 px-4 py-3 text-sm text-danger">
          {error}
        </div>
      )}

      {/* Keys table */}
      {loading ? (
        <div className="text-foreground-muted">Loading...</div>
      ) : keys.length === 0 ? (
        <div className="rounded-xl border border-border bg-surface p-12 text-center">
          <div className="text-4xl mb-4">🔑</div>
          <div className="font-medium mb-1">No API keys yet</div>
          <div className="text-sm text-foreground-muted">
            Create your first key to start making API calls.
          </div>
        </div>
      ) : (
        <div className="rounded-xl border border-border overflow-hidden">
          <table className="w-full">
            <thead>
              <tr className="border-b border-border bg-surface">
                <th className="text-left text-xs font-medium text-foreground-muted uppercase tracking-wider px-4 py-3">
                  Name
                </th>
                <th className="text-left text-xs font-medium text-foreground-muted uppercase tracking-wider px-4 py-3">
                  Key
                </th>
                <th className="text-left text-xs font-medium text-foreground-muted uppercase tracking-wider px-4 py-3">
                  Status
                </th>
                <th className="text-left text-xs font-medium text-foreground-muted uppercase tracking-wider px-4 py-3">
                  Created
                </th>
                <th className="px-4 py-3"></th>
              </tr>
            </thead>
            <tbody>
              {keys.map((key) => (
                <tr
                  key={key.id}
                  className="border-b border-border last:border-0 hover:bg-surface"
                >
                  <td className="px-4 py-3 font-medium">{key.name}</td>
                  <td className="px-4 py-3">
                    <code className="font-mono text-sm text-foreground-muted">
                      {key.key_prefix}...
                    </code>
                  </td>
                  <td className="px-4 py-3">
                    {key.active ? (
                      <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs bg-accent/10 text-accent border border-accent/20">
                        <span className="w-1.5 h-1.5 rounded-full bg-accent" />
                        Active
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs bg-foreground-dim/10 text-foreground-dim border border-foreground-dim/20">
                        <span className="w-1.5 h-1.5 rounded-full bg-foreground-dim" />
                        Revoked
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-sm text-foreground-muted">
                    {new Date(key.created_at).toLocaleDateString()}
                  </td>
                  <td className="px-4 py-3 text-right">
                    {key.active && (
                      <button
                        onClick={() => handleDelete(key.id)}
                        className="text-sm text-foreground-muted hover:text-danger"
                      >
                        Revoke
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* ─── Create Key Modal ─── */}
      {showCreate && (
        <div
          className="fixed inset-0 bg-black/60 flex items-center justify-center z-50"
          onClick={() => setShowCreate(false)}
        >
          <div
            className="w-full max-w-md rounded-xl border border-border bg-surface p-6 m-6"
            onClick={(e) => e.stopPropagation()}
          >
            <h2 className="text-lg font-bold mb-4">Create API Key</h2>
            <form onSubmit={handleCreate}>
              <label className="block text-sm font-medium mb-1.5">
                Key name
              </label>
              <input
                type="text"
                value={newKeyName}
                onChange={(e) => setNewKeyName(e.target.value)}
                placeholder="e.g. production-server"
                className="w-full rounded-lg border border-border bg-background px-4 py-2.5 text-sm outline-none focus:border-accent mb-4"
                autoFocus
              />
              <div className="flex gap-3">
                <button
                  type="submit"
                  className="flex-1 rounded-lg bg-accent py-2.5 font-medium text-accent-foreground hover:bg-accent-hover"
                >
                  Create
                </button>
                <button
                  type="button"
                  onClick={() => setShowCreate(false)}
                  className="px-4 rounded-lg border border-border hover:border-border-hover"
                >
                  Cancel
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* ─── New Key Display Modal ─── */}
      {newKey && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="w-full max-w-lg rounded-xl border border-border bg-surface p-6 m-6">
            <div className="flex items-center gap-3 mb-2">
              <span className="text-2xl">✅</span>
              <h2 className="text-lg font-bold">Key created!</h2>
            </div>
            <p className="text-sm text-foreground-muted mb-4">
              Copy your key now. For security, it will not be shown again.
            </p>
            <div className="flex items-center gap-2">
              <code className="flex-1 rounded-lg bg-background border border-border px-4 py-3 text-sm font-mono break-all">
                {newKey}
              </code>
              <button
                onClick={() => copyToClipboard(newKey)}
                className="shrink-0 rounded-lg bg-accent px-4 py-3 font-medium text-accent-foreground hover:bg-accent-hover"
              >
                {copied ? "Copied!" : "Copy"}
              </button>
            </div>
            <button
              onClick={() => setNewKey(null)}
              className="mt-4 w-full rounded-lg border border-border py-2.5 hover:border-border-hover"
            >
              Done
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
