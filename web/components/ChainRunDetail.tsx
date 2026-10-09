"use client";

import Link from "next/link";
import { ChainRun, ChainRunStep, Execution } from "@/lib/api";

// Groups a run's executions by step name, oldest attempt first.
export function groupExecutionsByStep(execs: Execution[]): Map<string, Execution[]> {
  const m = new Map<string, Execution[]>();
  for (const e of execs) {
    const list = m.get(e.step_name) ?? [];
    list.push(e);
    m.set(e.step_name, list);
  }
  for (const list of Array.from(m.values())) {
    list.sort((a, b) => a.attempt - b.attempt);
  }
  return m;
}

// A single webhook attempt: links to the full execution detail. Shows the
// error_code for failed deliveries, HTTP status otherwise.
export function AttemptChip({ e }: { e: Execution }) {
  const ok = !e.error_code && e.response_status >= 200 && e.response_status < 300;
  const label = e.error_code ? e.error_code : `HTTP ${e.response_status}`;
  return (
    <Link
      href={`/executions/${e.id}`}
      title={new Date(e.created_at).toLocaleString()}
      className={`rounded-full border px-2 py-0.5 font-mono text-[11px] transition hover:border-edgeBright ${
        ok
          ? "border-brand/30 bg-brand/10 text-brand"
          : "border-fail/30 bg-fail/10 text-fail"
      }`}
    >
      attempt {e.attempt} · {label}
    </Link>
  );
}

function StepLineageRow({
  name,
  step,
  execs,
}: {
  name: string;
  step?: ChainRunStep;
  execs: Execution[];
}) {
  return (
    <div className="rounded-md border border-edge bg-base px-3 py-2">
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-mono text-xs text-textMuted">{name}</span>
        {step?.step_type && (
          <span className="font-mono text-[10px] text-textFaint">{step.step_type}</span>
        )}
        {step && (
          <span
            className={`font-mono text-[11px] font-bold ${
              step.passed ? "text-pass" : "text-fail"
            }`}
          >
            {step.passed ? "passed" : "failed"}
          </span>
        )}
        <span className="ml-auto font-mono text-[11px] text-textFaint">
          {step
            ? `${step.response_status ? `${step.response_status} · ` : ""}+${step.duration_ms}ms`
            : ""}
        </span>
      </div>
      {step?.error && <p className="mt-1 font-mono text-[11px] text-fail">{step.error}</p>}
      {execs.length > 0 && (
        <div className="mt-1.5 flex flex-wrap gap-1.5">
          {execs.map((e, i) => (
            <span key={e.id} className="flex items-center gap-1.5">
              {i > 0 && (
                <span className="font-mono text-[10px] text-textFaint">
                  +{Math.max(0, Math.round(
                    (new Date(e.created_at).getTime() -
                      new Date(execs[i - 1].created_at).getTime()) /
                      1000
                  ))}s
                </span>
              )}
              <AttemptChip e={e} />
            </span>
          ))}
        </div>
      )}
    </div>
  );
}

// Step → webhook-attempt lineage for a finalized (or in-flight) run: finalized
// steps from the run itself (covers api/custom steps that produce no
// executions, plus the pass/fail verdict per step) merged with the run's
// executions grouped by step_name, in step order.
export function RunStepLineage({
  run,
  executions,
}: {
  run: ChainRun;
  executions: Execution[];
}) {
  const byStep = groupExecutionsByStep(executions);
  const ordered: string[] = [];
  const sorted = [...(run.steps ?? [])].sort((a, b) => a.position - b.position);
  for (const s of sorted) {
    if (!ordered.includes(s.step_name)) ordered.push(s.step_name);
  }
  for (const e of executions) {
    if (!ordered.includes(e.step_name)) ordered.push(e.step_name);
  }
  return (
    <div className="space-y-2">
      {ordered.map((name) => (
        <StepLineageRow
          key={name}
          name={name}
          step={sorted.find((s) => s.step_name === name)}
          execs={byStep.get(name) ?? []}
        />
      ))}
      {run.error && <p className="font-mono text-xs text-fail">{run.error}</p>}
    </div>
  );
}
