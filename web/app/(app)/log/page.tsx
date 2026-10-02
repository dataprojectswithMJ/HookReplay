"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { Button, EmptyState, ErrorBanner, Spinner } from "@/components/ui";
import { Execution, listExecutions } from "@/lib/api";

export default function LogPage() {
  const [rows, setRows] = useState<Execution[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(() => {
    setLoading(true);
    setError("");
    listExecutions()
      .then(setRows)
      .catch((e: any) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  useEffect(load, [load]);

  return (
    <div className="min-h-screen">
      <main className="mx-auto max-w-6xl px-6 py-12">
        <div className="flex items-end justify-between">
          <div>
            <p className="font-mono text-sm text-brand">hookreplay / log</p>
            <h1 className="mt-2 text-3xl font-bold tracking-tight text-white">
              Execution log
            </h1>
          </div>
          <Button variant="ghost" onClick={load} disabled={loading}>
            Refresh
          </Button>
        </div>

        {error && (
          <div className="mt-4">
            <ErrorBanner message={error} />
          </div>
        )}

        {loading ? (
          <div className="mt-10 grid place-items-center py-24">
            <Spinner className="h-6 w-6" />
          </div>
        ) : !error && rows.length === 0 ? (
          <div className="mt-6">
            <EmptyState
              title="No executions yet"
              description="Fire your first webhook and it'll show up here."
              action={
                <Link
                  href="/fire"
                  className="inline-block rounded-md bg-brand px-4 py-2 text-sm font-semibold text-black hover:bg-emerald-300"
                >
                  Fire a webhook
                </Link>
              }
            />
          </div>
        ) : (
          <div className="mt-6 overflow-hidden rounded-xl border border-edge">
            <table className="w-full text-left text-sm">
              <thead className="bg-panel text-xs uppercase tracking-wide text-gray-500">
                <tr>
                  <th className="px-4 py-3">Template</th>
                  <th className="px-4 py-3">Status</th>
                  <th className="px-4 py-3">Latency</th>
                  <th className="px-4 py-3">Attempt</th>
                  <th className="px-4 py-3">When</th>
                  <th className="px-4 py-3"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-edge bg-base">
                {rows.map((e) => (
                  <tr key={e.id} className="hover:bg-panel/60">
                    <td className="px-4 py-3 font-mono text-gray-200">
                      {e.template_id || e.step_name}
                    </td>
                    <td className="px-4 py-3">
                      <StatusPill e={e} />
                    </td>
                    <td className="px-4 py-3 text-gray-400">{e.latency_ms} ms</td>
                    <td className="px-4 py-3 text-gray-400">
                      {e.attempt > 1 ? e.attempt : "—"}
                    </td>
                    <td className="px-4 py-3 text-gray-400">
                      {new Date(e.created_at).toLocaleString()}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <Link
                        href={`/executions/${e.id}`}
                        className="text-brand hover:underline"
                      >
                        View
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </main>
    </div>
  );
}

function StatusPill({ e }: { e: Execution }) {
  if (e.error_code) {
    return (
      <span className="rounded-full bg-red-500/15 px-2.5 py-0.5 font-mono text-xs text-red-300">
        {e.error_code}
      </span>
    );
  }
  const ok = e.response_status >= 200 && e.response_status < 300;
  return (
    <span
      className={`rounded-full px-2.5 py-0.5 font-mono text-xs ${
        ok ? "bg-brand/15 text-brand" : "bg-amber-500/15 text-amber-300"
      }`}
    >
      HTTP {e.response_status}
    </span>
  );
}

