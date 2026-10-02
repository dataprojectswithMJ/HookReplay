"use client";

import { useEffect, useState } from "react";
import { Spinner } from "@/components/ui";
import { Domain, createDomain, listDomains, verifyDomain } from "@/lib/api";

export default function DomainsPage() {
  const [domains, setDomains] = useState<Domain[]>([]);
  const [domain, setDomain] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  function load() {
    listDomains()
      .then(setDomains)
      .catch((e: any) => setError(e.message))
      .finally(() => setLoading(false));
  }
  useEffect(load, []);

  async function onAdd() {
    if (!domain) return;
    setError("");
    try {
      const d = await createDomain(domain);
      setDomain("");
      setError(
        `Serve the token "${d.verification_token}" at https://${d.domain}/.well-known/hookreplay-verify, then click Verify.`
      );
      load();
    } catch (e: any) {
      setError(e.message);
    }
  }

  async function onVerify(id: string) {
    setError("");
    try {
      await verifyDomain(id);
      load();
    } catch (e: any) {
      setError(e.message);
    }
  }

  return (
    <div className="min-h-screen">
      <main className="mx-auto max-w-5xl px-6 py-12">
        <p className="font-mono text-sm text-brand">hookreplay / domains</p>
        <h1 className="mt-2 text-3xl font-bold text-white">Domains</h1>
        <p className="mt-2 text-sm text-gray-400">
          Verify a domain to fire webhooks at public URLs (Pro feature).
        </p>

        {error && (
          <p className="mt-4 rounded-md border border-brand/40 bg-brand/5 px-3 py-2 text-sm text-brand">
            {error}
          </p>
        )}

        <div className="mt-6 flex items-end gap-3 rounded-xl border border-edge bg-panel p-5">
          <div className="flex-1">
            <label className="block text-xs text-gray-500">Domain</label>
            <input
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
              placeholder="staging.example.com"
              className="mt-1 w-full rounded-md border border-edge bg-base px-3 py-2 font-mono text-sm text-gray-200 outline-none focus:border-brand/50"
            />
          </div>
          <button
            onClick={onAdd}
            className="rounded-md bg-brand px-4 py-2 font-semibold text-black hover:bg-emerald-300"
          >
            Add domain
          </button>
        </div>

        {loading ? (
          <div className="mt-6 grid place-items-center rounded-xl border border-edge py-20">
            <Spinner />
          </div>
        ) : (
        <div className="mt-6 overflow-hidden rounded-xl border border-edge">
          <table className="w-full text-left text-sm">
            <thead className="bg-panel text-xs uppercase tracking-wide text-gray-500">
              <tr>
                <th className="px-4 py-3">Domain</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3">Token</th>
                <th className="px-4 py-3"></th>
              </tr>
            </thead>
            <tbody className="divide-y divide-edge bg-base">
              {domains.map((d) => (
                <tr key={d.id}>
                  <td className="px-4 py-3 font-mono text-gray-200">{d.domain}</td>
                  <td className="px-4 py-3">
                    {d.verified_at ? (
                      <span className="rounded-full bg-brand/15 px-2.5 py-0.5 text-xs text-brand">
                        verified
                      </span>
                    ) : (
                      <span className="rounded-full bg-amber-500/15 px-2.5 py-0.5 text-xs text-amber-300">
                        pending
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-gray-500">
                    {d.verification_token}
                  </td>
                  <td className="px-4 py-3 text-right">
                    {!d.verified_at && (
                      <button
                        onClick={() => onVerify(d.id)}
                        className="text-brand hover:underline"
                      >
                        Verify
                      </button>
                    )}
                  </td>
                </tr>
              ))}
              {domains.length === 0 && (
                <tr>
                  <td colSpan={4} className="px-4 py-8 text-center text-gray-600">
                    No domains yet.
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
