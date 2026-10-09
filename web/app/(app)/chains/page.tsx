"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import {
  Chain,
  ChainRun,
  Execution,
  Template,
  createChain,
  deleteChain,
  executeChainAsync,
  getChainRun,
  getRunExecutions,
  listChainRuns,
  listChains,
  listTemplates,
} from "@/lib/api";
import ChainBuilder from "@/components/ChainBuilder";
import { RunStepLineage, AttemptChip, groupExecutionsByStep } from "@/components/ChainRunDetail";
import { Spinner } from "@/components/ui";
import { ChainStepForm, parseChainYaml } from "@/lib/chainYaml";
import { useTargetUrl } from "@/lib/target";

const POLL_MS = 2000;

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

function statusColor(status: number): string {
  if (status >= 200 && status < 300) return "text-pass";
  if (status >= 400 && status < 500) return "text-warn";
  return "text-fail";
}

function PersistedRunRow({ run }: { run: ChainRun }) {
  const [open, setOpen] = useState(false);
  const [detail, setDetail] = useState<ChainRun | null>(null);

  async function toggle() {
    const next = !open;
    setOpen(next);
    if (next && !detail) {
      try {
        setDetail(await getChainRun(run.id));
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
        <span
          className={`font-mono text-xs font-bold ${
            run.status === "passed" ? "text-pass" : "text-fail"
          }`}
        >
          {run.status.toUpperCase()}
        </span>
        <span className="font-mono text-xs text-textMuted">{run.name}</span>
        <span className="ml-auto font-mono text-[11px] text-textFaint">
          {new Date(run.started_at).toLocaleString()} · {run.duration_ms}ms
        </span>
      </button>
      {open && detail?.steps && (
        <div className="border-t border-edge p-4">
          <div className="flex items-center">
            {detail.steps.map((s, i) => (
              <div key={s.step_name} className="flex flex-1 items-center">
                <div className="flex flex-col items-center">
                  <span
                    className={`h-3.5 w-3.5 rounded-full ${
                      s.passed ? "border-2 border-brand bg-transparent" : "bg-fail"
                    }`}
                  />
                  <span className="mt-2 max-w-24 truncate text-center text-[11px] text-textMuted">
                    {s.step_name}
                  </span>
                  <span
                    className={`font-mono text-[11px] font-bold ${
                      s.passed ? statusColor(s.response_status) : "text-fail"
                    }`}
                  >
                    {s.passed
                      ? `${s.response_status} · +${s.duration_ms}ms`
                      : "failed"}
                  </span>
                  {s.error && (
                    <span className="mt-1 max-w-32 text-center font-mono text-[10px] text-fail">
                      {s.error}
                    </span>
                  )}
                </div>
                {i < detail.steps!.length - 1 && (
                  <div className="mx-1 h-px flex-1 bg-edge" />
                )}
              </div>
            ))}
          </div>
          <div className="mt-4 space-y-2">
            {detail.steps.map((s) => (
              <details key={s.step_name} className="rounded-md border border-edge bg-base px-3 py-2">
                <summary className="cursor-pointer font-mono text-xs text-textMuted">
                  {s.step_name} · {s.request_summary}
                </summary>
                <pre className="mt-2 overflow-x-auto font-mono text-xs text-textMuted">
                  {s.response_body}
                </pre>
              </details>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

// While a run is in flight: per-step live status derived from the executions
// endpoint (matched by step_name). Steps with no executions yet show a
// spinner; attempts land as they happen and link to full execution detail.
function LiveStepStatus({
  steps,
  executions,
}: {
  steps: ChainStepForm[];
  executions: Execution[];
}) {
  const byStep = groupExecutionsByStep(executions);
  return (
    <div className="space-y-2">
      {steps.map((s) => {
        const execs = byStep.get(s.name) ?? [];
        const pending = execs.length === 0;
        return (
          <div key={s.name || "unnamed"} className="rounded-md border border-edge bg-base px-3 py-2">
            <div className="flex items-center gap-2">
              {pending && <Spinner className="h-3 w-3" />}
              <span className="font-mono text-xs text-textMuted">{s.name || "(unnamed)"}</span>
              <span className="font-mono text-[10px] text-textFaint">{s.type}</span>
              {pending && <span className="font-mono text-[11px] text-textFaint">waiting…</span>}
            </div>
            {execs.length > 0 && (
              <div className="mt-1.5 flex flex-wrap gap-1.5">
                {execs.map((e) => (
                  <AttemptChip key={e.id} e={e} />
                ))}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}

export default function ChainsPage() {
  const [yaml, setYaml] = useState(SAMPLE);
  const [target] = useTargetUrl();
  const [env, setEnv] = useState("local");
  const [error, setError] = useState("");
  const [saved, setSaved] = useState<Chain[]>([]);
  const [savedOpen, setSavedOpen] = useState(false);
  const [saveName, setSaveName] = useState("");
  const [loadedId, setLoadedId] = useState<string | null>(null);
  const [history, setHistory] = useState<ChainRun[]>([]);
  const [templates, setTemplates] = useState<Template[]>([]);
  // Live async run state.
  const [runId, setRunId] = useState<string | null>(null);
  const [liveRun, setLiveRun] = useState<ChainRun | null>(null);
  const [liveExecs, setLiveExecs] = useState<Execution[]>([]);
  const [runTick, setRunTick] = useState(0);

  function loadSaved() {
    listChains()
      .then(setSaved)
      .catch(() => {});
  }
  useEffect(loadSaved, []);

  useEffect(() => {
    listTemplates()
      .then(setTemplates)
      .catch(() => {});
  }, []);

  useEffect(() => {
    if (!loadedId) {
      setHistory([]);
      return;
    }
    listChainRuns(loadedId)
      .then(setHistory)
      .catch(() => setHistory([]));
  }, [loadedId, runTick]);

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
      if (loadedId === id) setLoadedId(null);
      loadSaved();
    } catch (e: any) {
      setError(e.message);
    }
  }

  const running = !!runId && (!liveRun || liveRun.status === "running");

  async function run() {
    setError("");
    setLiveRun(null);
    setLiveExecs([]);
    setRunId(null);
    try {
      const { run_id } = await executeChainAsync(yaml, target, env, loadedId ?? undefined);
      setRunId(run_id);
    } catch (e: any) {
      setError(e.message);
    }
  }

  // Poll the run and its live per-attempt executions until the run finalizes.
  useEffect(() => {
    if (!runId) return;
    let stopped = false;
    let timer = 0;
    async function tick() {
      try {
        const [run, execs] = await Promise.all([getChainRun(runId!), getRunExecutions(runId!)]);
        if (stopped) return;
        setLiveRun(run);
        setLiveExecs(execs);
        if (run.status === "running") {
          timer = window.setTimeout(tick, POLL_MS);
        } else {
          setRunTick((t) => t + 1);
        }
      } catch {
        if (!stopped) timer = window.setTimeout(tick, POLL_MS);
      }
    }
    tick();
    return () => {
      stopped = true;
      window.clearTimeout(timer);
    };
  }, [runId]);

  return (
    <div className="min-h-screen">
      <main className="mx-auto max-w-6xl px-6 py-12">
        <p className="font-mono text-sm text-textFaint">hookreplay / chains</p>
        <h1 className="mt-2 text-3xl font-semibold tracking-tight text-text">
          Chain runner
        </h1>
        <p className="mt-2 text-textMuted">
          Author a chain as YAML and run it end-to-end against the target in the
          bar above.
        </p>
        <Link
          href="/docs#chains"
          className="mt-3 inline-block font-mono text-xs text-brand hover:underline"
        >
          Chain YAML reference →
        </Link>

        <section className="mt-8 rounded-xl border border-edge bg-panel">
          <button
            onClick={() => setSavedOpen(!savedOpen)}
            className="flex w-full items-center gap-3 px-5 py-3 text-left"
          >
            <span className="font-mono text-xs text-textFaint">
              {savedOpen ? "▾" : "▸"}
            </span>
            <h2 className="text-sm font-semibold uppercase tracking-wide text-textFaint">
              Saved chains
            </h2>
            <span className="ml-auto font-mono text-[11px] text-textFaint">
              {saved.length}
            </span>
          </button>
          {savedOpen && (
            <div className="border-t border-edge px-5 py-3">
              {saved.length === 0 ? (
                <p className="py-2 text-sm text-textFaint">
                  No saved chains yet — author one below and hit Save.
                </p>
              ) : (
                <div className="divide-y divide-edge">
                  {saved.map((c) => (
                    <div key={c.id} className="flex items-center gap-3 py-2">
                      <div className="min-w-0">
                        <div className="truncate font-mono text-sm text-textMuted">
                          {c.name}
                          {loadedId === c.id && (
                            <span className="ml-2 text-brand">●</span>
                          )}
                        </div>
                        <div className="font-mono text-[11px] text-textFaint">
                          {c.source} · v{c.version} ·{" "}
                          {new Date(c.updated_at).toLocaleString()}
                        </div>
                      </div>
                      <div className="ml-auto flex gap-2">
                        <button
                          onClick={() => setLoadedId(c.id)}
                          className="rounded-md border border-edge px-3 py-1 font-mono text-xs text-textMuted transition hover:border-edgeBright"
                        >
                          Load
                        </button>
                        <button
                          onClick={() => {
                            setYaml(c.yaml_text);
                            setLoadedId(c.id);
                          }}
                          className="rounded-md border border-edge px-3 py-1 font-mono text-xs text-textMuted transition hover:border-edgeBright"
                        >
                          Edit
                        </button>
                        <button
                          onClick={() => remove(c.id)}
                          className="rounded-md border border-edge px-3 py-1 font-mono text-xs text-fail transition hover:border-fail/50"
                        >
                          Delete
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}
        </section>

        <section className="mt-6 rounded-xl border border-edge bg-panel p-5">
          <label className="block text-xs text-textFaint">Environment</label>
          <select
            value={env}
            onChange={(e) => setEnv(e.target.value)}
            className="mt-1 w-48 rounded-md border border-edge bg-base px-3 py-2 font-mono text-sm text-textMuted outline-none focus:border-edgeBright"
          >
            <option value="local">local</option>
            <option value="staging">staging</option>
            <option value="production">production</option>
          </select>

          <label className="mt-4 block text-xs text-textFaint">Chain definition</label>
          <div className="mt-1">
            <ChainBuilder
              value={yaml}
              onChange={(v) => {
                setYaml(v);
                setLoadedId(null);
              }}
              templates={templates}
            />
          </div>

          <button
            onClick={run}
            disabled={running}
            className="mt-4 w-full rounded-md bg-brand py-2.5 font-semibold text-black transition hover:bg-brandBright disabled:opacity-50"
          >
            {running ? "Running…" : "Run chain"}
          </button>

          <div className="mt-3 flex gap-2">
            <input
              value={saveName}
              onChange={(e) => setSaveName(e.target.value)}
              placeholder="Chain name (optional)"
              className="flex-1 rounded-md border border-edge bg-base px-3 py-2 font-mono text-sm text-textMuted outline-none focus:border-edgeBright"
            />
            <button
              onClick={save}
              className="rounded-md border border-edge px-4 py-2 font-mono text-xs text-textMuted transition hover:border-edgeBright"
            >
              Save
            </button>
          </div>

          {error && (
            <p className="mt-4 rounded-md border border-fail/30 bg-fail/10 px-3 py-2 font-mono text-xs text-fail">
              {error}
            </p>
          )}
        </section>

        <section className="mt-6 rounded-xl border border-edge bg-panel p-5">
          <h2 className="text-sm font-semibold uppercase tracking-wide text-textFaint">
            Result
          </h2>
          {!runId && !liveRun && (
            <div className="mt-4 grid place-items-center rounded-lg border border-dashed border-edge py-16 font-mono text-xs text-textFaint">
              Run a chain to see per-step results.
            </div>
          )}
          {runId && (
            <div className="mt-4">
              <div className="mb-4 flex items-center gap-2 font-mono text-xs text-textFaint">
                {running && <Spinner className="h-3.5 w-3.5" />}
                <span>
                  run {runId} · {liveRun?.status ?? "starting"}
                </span>
                {liveRun && !running && (
                  <span
                    className={`font-bold ${
                      liveRun.status === "passed" ? "text-pass" : "text-fail"
                    }`}
                  >
                    {liveRun.name} — {liveRun.status.toUpperCase()} · {liveRun.duration_ms}ms
                  </span>
                )}
              </div>
              {liveRun && !running ? (
                <RunStepLineage run={liveRun} executions={liveExecs} />
              ) : (
                <LiveStepStatus steps={parseChainYaml(yaml).form.steps} executions={liveExecs} />
              )}
            </div>
          )}
        </section>

        {loadedId && history.length > 0 && (
          <section className="mt-6 rounded-xl border border-edge bg-panel p-5">
            <h2 className="text-sm font-semibold uppercase tracking-wide text-textFaint">
              Run history
            </h2>
            <div className="mt-3 space-y-2">
              {history.map((r) => (
                <PersistedRunRow key={r.id} run={r} />
              ))}
            </div>
          </section>
        )}
      </main>
    </div>
  );
}
