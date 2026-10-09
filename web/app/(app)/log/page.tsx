"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { Button, EmptyState, ErrorBanner, Spinner } from "@/components/ui";
import { RunStepLineage } from "@/components/ChainRunDetail";
import {
  ChainRun,
  Execution,
  deliveryFailed,
  deliveryLabel,
  getChainRun,
  getRunExecutions,
  listExecutions,
  listRuns,
} from "@/lib/api";

type Tab = "webhooks" | "chains";

export default function LogPage() {
  const [tab, setTab] = useState<Tab>("webhooks");
  const [rows, setRows] = useState<Execution[]>([]);
  const [runs, setRuns] = useState<ChainRun[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(() => {
    setLoading(true);
    setError("");
    const p =
      tab === "webhooks" ? listExecutions().then(setRows) : listRuns().then(setRuns);
    p.catch((e: any) => setError(e.message)).finally(() => setLoading(false));
  }, [tab]);

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

        <div className="mt-6 flex gap-1 rounded-lg border border-edge bg-panel p-1">
          <button
            onClick={() => setTab("webhooks")}
            className={`flex-1 rounded-md px-4 py-2 text-sm font-semibold transition ${
              tab === "webhooks" ? "bg-brand text-black" : "text-textMuted hover:text-text"
            }`}
          >
            Webhooks
          </button>
          <button
            onClick={() => setTab("chains")}
            className={`flex-1 rounded-md px-4 py-2 text-sm font-semibold transition ${
              tab === "chains" ? "bg-brand text-black" : "text-textMuted hover:text-text"
            }`}
          >
            Chains
          </button>
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
        ) : tab === "webhooks" ? (
          !error && rows.length === 0 ? (
            <div className="mt-6">
              <EmptyState
                title="No executions yet"
                description="Fire your first webhook and it'll show up here."
                action={
                  <Link
                    href="/templates"
                    className="inline-block rounded-md bg-brand px-4 py-2 text-sm font-semibold text-black hover:bg-brandBright"
                  >
                    Fire a webhook
                  </Link>
                }
              />
            </div>
          ) : (
            <div className="mt-6 overflow-hidden rounded-xl border border-edge">
              <table className="w-full text-left text-sm">
                <thead className="bg-panel text-xs uppercase tracking-wide text-textFaint">
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
                      <td className="px-4 py-3 font-mono text-textMuted">
                        {e.template_id || e.step_name}
                      </td>
                      <td className="px-4 py-3">
                        <StatusPill e={e} />
                      </td>
                      <td className="px-4 py-3 text-textMuted">{e.latency_ms} ms</td>
                      <td className="px-4 py-3 text-textMuted">
                        {e.attempt > 1 ? e.attempt : "—"}
                      </td>
                      <td className="px-4 py-3 text-textMuted">
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
          )
        ) : !error && runs.length === 0 ? (
          <div className="mt-6">
            <EmptyState
              title="No chain runs yet"
              description="Run a chain and its attempts will show up here."
              action={
                <Link
                  href="/chains"
                  className="inline-block rounded-md bg-brand px-4 py-2 text-sm font-semibold text-black hover:bg-brandBright"
                >
                  Run a chain
                </Link>
              }
            />
          </div>
        ) : (
          <div className="mt-6 space-y-2">
            {runs.map((run) => (
              <ChainRunRow key={run.id} run={run} />
            ))}
          </div>
        )}
      </main>
    </div>
  );
}

function statusClass(status: string): string {
  if (status === "passed") return "text-pass";
  if (status === "running") return "text-warn";
  return "text-fail";
}

// One workspace chain run, expandable into step → execution lineage.
function ChainRunRow({ run }: { run: ChainRun }) {
  const [open, setOpen] = useState(false);
  const [detail, setDetail] = useState<ChainRun | null>(null);
  const [execs, setExecs] = useState<Execution[]>([]);

  async function toggle() {
    const next = !open;
    setOpen(next);
    if (next && !detail) {
      try {
        const [d, e] = await Promise.all([getChainRun(run.id), getRunExecutions(run.id)]);
        setDetail(d);
        setExecs(e);
      } catch {}
    }
  }

  return (
    <div className="rounded-md border border-edge bg-base">
      <button
        onClick={toggle}
        className="flex w-full flex-wrap items-center gap-3 px-3 py-2 text-left"
      >
        <span className="font-mono text-[11px] text-textFaint">
          {open ? "▾" : "▸"}
        </span>
        <span className={`font-mono text-xs font-bold ${statusClass(run.status)}`}>
          {run.status.toUpperCase()}
        </span>
        <span className="font-mono text-xs text-textMuted">{run.name}</span>
        <span className="ml-auto font-mono text-[11px] text-textFaint">
          {new Date(run.started_at).toLocaleString()} · {run.duration_ms}ms
        </span>
      </button>
      {open && detail && (
        <div className="border-t border-edge p-4">
          <RunStepLineage run={detail} executions={execs} />
        </div>
      )}
    </div>
  );
}

function StatusPill({ e }: { e: Execution }) {
  if (e.error_code) {
    return (
      <span className="rounded-full bg-fail/15 px-2.5 py-0.5 font-mono text-xs text-fail">
        {e.error_code}
      </span>
    );
  }
  if (deliveryFailed(e.deliver)) {
    return (
      <span className="rounded-full bg-fail/15 px-2.5 py-0.5 font-mono text-xs text-fail">
        {deliveryLabel(e.deliver)}
      </span>
    );
  }
  const ok = e.response_status >= 200 && e.response_status < 300;
  return (
    <span
      className={`rounded-full px-2.5 py-0.5 font-mono text-xs ${
        ok ? "bg-brand/15 text-brand" : "bg-warn/15 text-warn"
      }`}
    >
      HTTP {e.response_status}
    </span>
  );
}

