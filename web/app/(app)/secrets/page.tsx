"use client";

import { useEffect, useState } from "react";
import { Spinner } from "@/components/ui";
import { SecretName, deleteSecret, listSecrets, setSecret } from "@/lib/api";

export default function SecretsPage() {
  const [secrets, setSecrets] = useState<SecretName[]>([]);
  const [name, setName] = useState("");
  const [env, setEnv] = useState("local");
  const [kind, setKind] = useState<"secret" | "variable">("secret");
  const [value, setValue] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  function load() {
    listSecrets()
      .then(setSecrets)
      .catch((e: any) => setError(e.message))
      .finally(() => setLoading(false));
  }
  useEffect(load, []);

  async function onAdd() {
    if (!name || !value) return;
    setError("");
    try {
      await setSecret(env, name, value, kind);
      setName("");
      setValue("");
      load();
    } catch (e: any) {
      setError(e.message);
    }
  }

  async function onDelete(env: string, name: string) {
    try {
      await deleteSecret(env, name);
      load();
    } catch (e: any) {
      setError(e.message);
    }
  }

  return (
    <div className="min-h-screen">
      <main className="mx-auto max-w-5xl px-6 py-12">
        <p className="font-mono text-sm text-brand">hookreplay / variables</p>
        <h1 className="mt-2 text-3xl font-bold text-white">Variables &amp; secrets</h1>
        <p className="mt-2 text-sm text-textMuted">
          Secrets are encrypted and masked (write-only). Variables are visible and editable.
        </p>

        {error && (
          <p className="mt-4 rounded-md border border-fail/30 bg-fail/10 px-3 py-2 text-sm text-fail">
            {error}
          </p>
        )}

        <div className="mt-6 flex flex-wrap items-end gap-3 rounded-xl border border-edge bg-panel p-5">
          <div>
            <label className="block text-xs text-textFaint">Name</label>
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="STRIPE_WEBHOOK_SECRET"
              className="mt-1 rounded-md border border-edge bg-base px-3 py-2 font-mono text-sm text-textMuted outline-none focus:border-brand/50"
            />
          </div>
          <div>
            <label className="block text-xs text-textFaint">Environment</label>
            <select
              value={env}
              onChange={(e) => setEnv(e.target.value)}
              className="mt-1 rounded-md border border-edge bg-base px-3 py-2 font-mono text-sm text-textMuted outline-none focus:border-brand/50"
            >
              <option value="local">local</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>
          </div>
          <div>
            <label className="block text-xs text-textFaint">Kind</label>
            <select
              value={kind}
              onChange={(e) => setKind(e.target.value as "secret" | "variable")}
              className="mt-1 rounded-md border border-edge bg-base px-3 py-2 font-mono text-sm text-textMuted outline-none focus:border-brand/50"
            >
              <option value="secret">secret</option>
              <option value="variable">variable</option>
            </select>
          </div>
          <div>
            <label className="block text-xs text-textFaint">Value</label>
            <input
              type={kind === "secret" ? "password" : "text"}
              value={value}
              onChange={(e) => setValue(e.target.value)}
              placeholder={kind === "secret" ? "whsec_…" : "https://…"}
              className="mt-1 rounded-md border border-edge bg-base px-3 py-2 font-mono text-sm text-textMuted outline-none focus:border-brand/50"
            />
          </div>
          <button
            onClick={onAdd}
            className="rounded-md bg-brand px-4 py-2 font-semibold text-black hover:bg-brandBright"
          >
            Add
          </button>
        </div>

        {loading ? (
          <div className="mt-6 grid place-items-center rounded-xl border border-edge py-20">
            <Spinner />
          </div>
        ) : (
        <div className="mt-6 overflow-hidden rounded-xl border border-edge">
          <table className="w-full text-left text-sm">
            <thead className="bg-panel text-xs uppercase tracking-wide text-textFaint">
              <tr>
                <th className="px-4 py-3">Name</th>
                <th className="px-4 py-3">Kind</th>
                <th className="px-4 py-3">Value</th>
                <th className="px-4 py-3">Environment</th>
                <th className="px-4 py-3">Updated</th>
                <th className="px-4 py-3"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-edge bg-base">
              {secrets.map((s) => (
                <tr key={`${s.env}/${s.name}`}>
                  <td className="px-4 py-3 font-mono text-textMuted">{s.name}</td>
                  <td className="px-4 py-3 font-mono text-xs text-textMuted">{s.kind}</td>
                  <td className="px-4 py-3 font-mono text-xs text-textMuted">
                    {s.kind === "variable" ? s.value : "••••••••"}
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-textMuted">{s.env}</td>
                  <td className="px-4 py-3 text-textMuted">
                    {new Date(s.updated_at).toLocaleString()}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button
                      onClick={() => onDelete(s.env, s.name)}
                      className="text-fail hover:underline"
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
              {secrets.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-4 py-8 text-center text-textFaint">
                    No variables yet.
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
