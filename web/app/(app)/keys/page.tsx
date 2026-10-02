"use client";

import { useEffect, useState } from "react";
import { CopyButton, Spinner } from "@/components/ui";
import { APIKey, createAPIKey, deleteAPIKey, listAPIKeys } from "@/lib/api";

export default function KeysPage() {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [error, setError] = useState("");
  const [modalOpen, setModalOpen] = useState(false);
  const [modalError, setModalError] = useState("");
  const [newKey, setNewKey] = useState("");
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [loading, setLoading] = useState(true);

  function load() {
    listAPIKeys()
      .then(setKeys)
      .catch((e: any) => setError(e.message))
      .finally(() => setLoading(false));
  }

  useEffect(load, []);

  function openModal() {
    setName("");
    setNewKey("");
    setModalError("");
    setModalOpen(true);
  }

  async function onCreate() {
    setBusy(true);
    setModalError("");
    try {
      const res = await createAPIKey(name.trim());
      setName("");
      setNewKey(res.api_key);
      load();
    } catch (e: any) {
      setModalError(e.message);
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
            onClick={openModal}
            className="rounded-md bg-brand px-4 py-2 font-semibold text-black hover:bg-emerald-300"
          >
            Create key
          </button>
        </div>

        {error && (
          <p className="mt-4 rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-300">
            {error}
          </p>
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
                <th className="px-4 py-3">Name</th>
                <th className="px-4 py-3">Prefix</th>
                <th className="px-4 py-3">Scopes</th>
                <th className="px-4 py-3">Last used</th>
                <th className="px-4 py-3"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-edge bg-base">
              {keys.map((k) => (
                <tr key={k.id}>
                  <td className="px-4 py-3 text-gray-200">{k.name || "—"}</td>
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
                  <td colSpan={5} className="px-4 py-8 text-center text-gray-600">
                    No keys yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
        )}
      </main>

      {modalOpen && (
        <div
          className="fixed inset-0 z-50 grid place-items-center bg-black/60 p-4"
          onClick={() => setModalOpen(false)}
        >
          <div
            className="w-full max-w-md rounded-xl border border-edge bg-panel p-6"
            onClick={(e) => e.stopPropagation()}
          >
            {newKey ? (
              <>
                <h2 className="text-lg font-semibold text-white">Copy your new key</h2>
                <p className="mt-1 text-sm text-gray-400">
                  This is the only time it&apos;s shown:
                </p>
                <div className="mt-3 flex items-start gap-2">
                  <pre className="flex-1 break-all rounded bg-base p-3 font-mono text-sm text-gray-200">
                    {newKey}
                  </pre>
                  <CopyButton text={newKey} />
                </div>
                <button
                  onClick={() => setModalOpen(false)}
                  className="mt-4 w-full rounded-md bg-brand py-2 font-semibold text-black hover:bg-emerald-300"
                >
                  Done
                </button>
              </>
            ) : (
              <>
                <h2 className="text-lg font-semibold text-white">Create API key</h2>
                <p className="mt-1 text-sm text-gray-400">
                  Give it a name so you can switch to it later with{" "}
                  <code className="rounded bg-white/5 px-1.5 py-0.5 font-mono text-xs text-brand">
                    hookreplay api-keys use &lt;name&gt;
                  </code>
                  .
                </p>
                <label className="mt-4 block text-xs text-gray-500">Key name</label>
                <input
                  autoFocus
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="e.g. staging"
                  className="mt-1 w-full rounded-md border border-edge bg-base px-3 py-2 text-sm text-gray-200 outline-none focus:border-brand/50"
                />
                {modalError && (
                  <p className="mt-2 text-sm text-red-300">{modalError}</p>
                )}
                <div className="mt-4 flex justify-end gap-2">
                  <button
                    onClick={() => setModalOpen(false)}
                    className="rounded-md border border-edge px-4 py-2 text-sm text-gray-300 hover:bg-white/5"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={onCreate}
                    disabled={busy || !name.trim()}
                    className="rounded-md bg-brand px-4 py-2 text-sm font-semibold text-black hover:bg-emerald-300 disabled:opacity-50"
                  >
                    {busy ? "Creating…" : "Create key"}
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
