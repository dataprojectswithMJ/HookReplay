"use client";

import { useEffect, useState } from "react";
import { Spinner } from "@/components/ui";
import { Usage, getUsage } from "@/lib/api";

export default function UsagePage() {
  const [usage, setUsage] = useState<Usage | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getUsage()
      .then(setUsage)
      .catch((e: any) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className="min-h-screen">
      <main className="mx-auto max-w-5xl px-6 py-12">
        <p className="font-mono text-sm text-brand">hookreplay / usage</p>
        <h1 className="mt-2 text-3xl font-bold text-white">Usage</h1>

        {error && (
          <p className="mt-4 rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-300">
            {error}
          </p>
        )}

        {loading ? (
          <div className="mt-6 grid place-items-center rounded-xl border border-edge py-20">
            <Spinner />
          </div>
        ) : usage && (
          <div className="mt-6 grid gap-4 sm:grid-cols-3">
            <Stat label="Tier" value={usage.tier} />
            <Stat
              label="Executions (last 30d)"
              value={usage.executions.toLocaleString()}
            />
            <Stat label="Replays" value={usage.replays.toLocaleString()} />
            <Stat
              label="Execution limit"
              value={usage.exec_limit == null ? "unlimited" : usage.exec_limit.toLocaleString()}
            />
            <Stat
              label="Chains limit"
              value={usage.chains_limit < 0 ? "unlimited" : String(usage.chains_limit)}
            />
          </div>
        )}
      </main>
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-xl border border-edge bg-panel p-5">
      <div className="text-xs text-gray-500">{label}</div>
      <div className="mt-1 text-2xl font-semibold text-white">{value}</div>
    </div>
  );
}
