"use client";

import { useEffect, useState } from "react";
import {
  Chain,
  ChainRunResult,
  createChain,
  deleteChain,
  executeChain,
  listChains,
} from "@/lib/api";

const SAMPLE = `name: payment-onboarding-flow
steps:
  - name: payment-received
    webhook: stripe/checkout.session.completed
    secret: env:STRIPE_WEBHOOK_SECRET
    expect:
      status: 200
  - name: order-must-exist
    api:
      method: GET
      url: "{{target}}/orders/{{steps.payment-received.body.data.object.id}}"
    expect:
      status: 200
      body:
        json_path: status
        equals: paid
`;

export default function ChainsPage() {
  const [yaml, setYaml] = useState(SAMPLE);
  const [target, setTarget] = useState("http://localhost:8080/t/your-tunnel/");
  const [env, setEnv] = useState("local");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<ChainRunResult | null>(null);
  const [saved, setSaved] = useState<Chain[]>([]);
  const [saveName, setSaveName] = useState("");

  function loadSaved() {
    listChains()
      .then(setSaved)
      .catch(() => {});
  }
  useEffect(loadSaved, []);

  async function save() {
    setError("");
    try {
      await createChain(saveName || "", yaml);
      setSaveName("");
      loadSaved();
    } catch (e: any) {
      setError(e.message);
    }
  }

  async function remove(id: string) {
    try {
      await deleteChain(id);
      loadSaved();
    } catch (e: any) {
      setError(e.message);
    }
  }

  async function run() {
    setLoading(true);
    setError("");
    setResult(null);
    try {
      const r = await executeChain(yaml, target, env);
      setResult(r);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen">
      <main className="mx-auto max-w-6xl px-6 py-12">
        <p className="font-mono text-sm text-brand">hookreplay / chains</p>
        <h1 className="mt-2 text-3xl font-bold text-white">Chain runner</h1>
        <p className="mt-2 text-gray-400">
          Author a chain as YAML, set the target, and run it end-to-end.
        </p>

        <div className="mt-6 grid gap-6 lg:grid-cols-2">
          <section className="rounded-xl border border-edge bg-panel p-5">
            <label className="block text-xs text-gray-500">Target URL</label>
            <input
              value={target}
              onChange={(e) => setTarget(e.target.value)}
              className="mt-1 w-full rounded-md border border-edge bg-base px-3 py-2 font-mono text-sm text-gray-200 outline-none focus:border-brand/50"
              placeholder="http://localhost:8080/t/your-tunnel/"
            />

            <label className="mt-4 block text-xs text-gray-500">
              Environment
            </label>
            <select
              value={env}
              onChange={(e) => setEnv(e.target.value)}
              className="mt-1 w-full rounded-md border border-edge bg-base px-3 py-2 font-mono text-sm text-gray-200 outline-none focus:border-brand/50"
            >
              <option value="local">local</option>
              <option value="staging">staging</option>
              <option value="production">production</option>
            </select>

            <label className="mt-4 block text-xs text-gray-500">Chain YAML</label>
            <textarea
              value={yaml}
              onChange={(e) => setYaml(e.target.value)}
              spellCheck={false}
              rows={20}
              className="mt-1 w-full rounded-md border border-edge bg-base px-3 py-2 font-mono text-xs text-gray-200 outline-none focus:border-brand/50"
            />

            <button
              onClick={run}
              disabled={loading}
              className="mt-4 w-full rounded-md bg-brand py-2.5 font-semibold text-black hover:bg-emerald-300 disabled:opacity-50"
            >
              {loading ? "Running…" : "Run chain"}
            </button>

            <div className="mt-3 flex gap-2">
              <input
                value={saveName}
                onChange={(e) => setSaveName(e.target.value)}
                placeholder="Chain name (optional)"
                className="flex-1 rounded-md border border-edge bg-base px-3 py-2 font-mono text-sm text-gray-200 outline-none focus:border-brand/50"
              />
              <button
                onClick={save}
                className="rounded-md border border-edge px-4 py-2 text-sm text-gray-300 hover:border-brand/50"
              >
                Save
              </button>
            </div>

            {error && (
              <p className="mt-4 rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-300">
                {error}
              </p>
            )}
          </section>

          <section className="rounded-xl border border-edge bg-panel p-5">
            <h2 className="text-sm font-semibold uppercase tracking-wide text-gray-400">
              Result
            </h2>
            {!result && (
              <div className="mt-4 grid place-items-center rounded-lg border border-dashed border-edge py-16 text-sm text-gray-600">
                Run a chain to see per-step results.
              </div>
            )}
            {result && (
              <div className="mt-4 space-y-2">
                <div
                  className={`rounded-md px-3 py-2 text-sm font-semibold ${
                    result.passed
                      ? "bg-brand/15 text-brand"
                      : "bg-red-500/15 text-red-300"
                  }`}
                >
                  {result.name} — {result.passed ? "PASS" : "FAIL"}
                </div>
                {result.steps.map((s) => (
                  <div
                    key={s.name}
                    className="rounded-md border border-edge bg-base px-3 py-2"
                  >
                    <div className="flex items-center gap-2">
                      <span className="text-gray-200">{s.name}</span>
                      <span className="ml-auto font-mono text-xs text-gray-400">
                        {s.skipped
                          ? "skipped"
                          : s.failed
                          ? "FAILED"
                          : `HTTP ${s.status} · ${s.latency_ms}ms`}
                      </span>
                    </div>
                    {s.expect_errors && s.expect_errors.length > 0 && (
                      <div className="mt-1 font-mono text-xs text-red-300">
                        {s.expect_errors.join("; ")}
                      </div>
                    )}
                    {s.error && !s.expect_errors?.length && (
                      <div className="mt-1 font-mono text-xs text-red-300">
                        {s.error}
                      </div>
                    )}
                    {s.retries ? (
                      <div className="mt-1 font-mono text-xs text-amber-300">
                        {s.retries} retries · delays {JSON.stringify(s.delays ?? [])}
                      </div>
                    ) : null}
                  </div>
                ))}
              </div>
            )}
          </section>
        </div>

        <section className="mt-6 rounded-xl border border-edge bg-panel p-5">
          <h2 className="text-sm font-semibold uppercase tracking-wide text-gray-400">
            Saved chains
          </h2>
          {saved.length === 0 ? (
            <p className="mt-3 text-sm text-gray-600">
              No saved chains yet — author one above and hit Save.
            </p>
          ) : (
            <div className="mt-3 divide-y divide-edge">
              {saved.map((c) => (
                <div key={c.id} className="flex items-center gap-3 py-2">
                  <div className="min-w-0">
                    <div className="truncate font-mono text-sm text-gray-200">
                      {c.name}
                    </div>
                    <div className="text-xs text-gray-500">
                      {c.source} · v{c.version} ·{" "}
                      {new Date(c.updated_at).toLocaleString()}
                    </div>
                  </div>
                  <div className="ml-auto flex gap-2">
                    <button
                      onClick={() => setYaml(c.yaml_text)}
                      className="rounded-md border border-edge px-3 py-1 text-xs text-gray-300 hover:border-brand/50"
                    >
                      Load
                    </button>
                    <button
                      onClick={() => remove(c.id)}
                      className="rounded-md border border-edge px-3 py-1 text-xs text-red-400 hover:border-red-500/50"
                    >
                      Delete
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>
      </main>
    </div>
  );
}
