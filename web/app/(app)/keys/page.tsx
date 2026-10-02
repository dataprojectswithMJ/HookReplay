"use client";

import { useEffect, useState } from "react";
import { CopyButton, Spinner } from "@/components/ui";
import { APIKey, createAPIKey, deleteAPIKey, listAPIKeys } from "@/lib/api";

export default function KeysPage() {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [error, setError] = useState("");
  const [newKey, setNewKey] = useState("");
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);

  function load() {
    listAPIKeys()
      .then(setKeys)
      .catch((e: any) => setError(e.message))
      .finally(() => setLoading(false));
  }

  useEffect(load, []);

  async function onCreate() {
    setBusy(true);
    setError("");
    setNewKey("");
    try {
      const res = await createAPIKey();
      setNewKey(res.api_key);
      load();
    } catch (e: any) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }

  async function onRevoke(id: string) {
    try {
      await deleteAPIKey(id);
      load();
    } catch (e: any) {
      setError(e.message);
    }
  }

  return (
    <div className="min-h-screen">
      <main className="mx-auto max-w-5xl px-6 py-12">
        <div className="flex items-center justify-between">
          <div>
            <p className="font-mono text-sm text-brand">hookreplay / keys</p>
            <h1 className="mt-2 text-3xl font-bold text-white">API keys</h1>
          </div>
          <button
            onClick={onCreate}
            disabled={busy}
            className="rounded-md bg-brand px-4 py-2 font-semibold text-black hover:bg-emerald-300 disabled:opacity-50"
          >
            Create key
          </button>
        </div>

        {error && (
          <p className="mt-4 rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-300">
            {error}
          </p>
        )}

        {newKey && (
          <div className="mt-6 rounded-lg border border-brand/40 bg-brand/5 p-4">
            <p className="text-sm font-semibold text-brand">
              Copy your new key now — it won&apos;t be shown again:
            </p>
            <div className="mt-2 flex items-start gap-2">
              <pre className="flex-1 break-all rounded bg-base p-3 font-mono text-sm text-gray-200">
                {newKey}
              </pre>
              <CopyButton text={newKey} />
            </div>
          </div>
        )}

        {loading ? (
          <div className="mt-6 grid place-items-center rounded-xl border border-edge py-20">
            <Spinner />
          </div>
        ) : (
        <div className="mt-6 overflow-hidden rounded-xl border border-edge">
          <table className="w-full text-left text-sm">
            <thead className="bg-panel text-xs uppercase tracking-wide text-gray-500">
              <tr>
                <th className="px-4 py-3">Prefix</th>
                <th className="px-4 py-3">Scopes</th>
                <th className="px-4 py-3">Last used</th>
                <th className="px-4 py-3"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-edge bg-base">
              {keys.map((k) => (
                <tr key={k.id}>
                  <td className="px-4 py-3 font-mono text-gray-200">{k.prefix}…</td>
                  <td className="px-4 py-3 font-mono text-xs text-gray-400">
                    {k.scopes.join(", ")}
                  </td>
                  <td className="px-4 py-3 text-gray-400">
                    {k.last_used_at
                      ? new Date(k.last_used_at).toLocaleString()
                      : "never"}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button
                      onClick={() => onRevoke(k.id)}
                      className="text-red-400 hover:underline"
                    >
                      Revoke
                    </button>
                  </td>
                </tr>
              ))}
              {keys.length === 0 && (
                <tr>
                  <td colSpan={4} className="px-4 py-8 text-center text-gray-600">
                    No keys yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
        )}
      </main>
    </div>
  );
}
