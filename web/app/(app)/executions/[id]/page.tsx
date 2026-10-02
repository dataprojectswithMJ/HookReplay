"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import {
  Badge,
  CodeBlock,
  CopyButton,
  ErrorBanner,
  Spinner,
  Stat,
} from "@/components/ui";
import { Execution, decodeBase64, getExecution } from "@/lib/api";

export default function ExecutionDetailPage() {
  const params = useParams<{ id: string }>();
  const [e, setE] = useState<Execution | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!params?.id) return;
    getExecution(params.id)
      .then(setE)
      .catch((err: any) => setError(err.message));
  }, [params?.id]);

  const statusTone: "neutral" | "good" | "bad" =
    e && (e.error_code || e.response_status >= 400)
      ? "bad"
      : e && e.response_status >= 200
      ? "good"
      : "neutral";

  return (
    <div className="min-h-screen">
      <main className="mx-auto max-w-6xl px-6 py-12">
        <Link href="/log" className="text-sm text-gray-400 hover:text-brand">
          ← back to log
        </Link>

        {error && (
          <div className="mt-4">
            <ErrorBanner message={error} />
          </div>
        )}

        {!e && !error && (
          <div className="mt-10 grid place-items-center py-24">
            <Spinner className="h-6 w-6" />
          </div>
        )}

        {e && (
          <>
            <div className="mt-4 flex flex-wrap items-center gap-3">
              <h1 className="text-2xl font-bold text-white">
                {e.template_id || e.step_name}
              </h1>
              <span className="font-mono text-sm text-gray-500">{e.id}</span>
              <CopyButton text={e.id} />
              {e.attempt > 1 && <Badge tone="amber">attempt {e.attempt}</Badge>}
            </div>

            {e.parent_execution_id && (
              <div className="mt-3 text-sm text-gray-400">
                Retry of{" "}
                <Link
                  href={`/executions/${e.parent_execution_id}`}
                  className="font-mono text-brand hover:underline"
                >
                  {e.parent_execution_id}
                </Link>
              </div>
            )}

            <div className="mt-6 grid gap-4 sm:grid-cols-3">
              <Stat
                label="Status"
                value={e.error_code || `HTTP ${e.response_status}`}
                tone={statusTone}
              />
              <Stat label="Latency" value={`${e.latency_ms} ms`} />
              <Stat label="Target" value={e.target} mono />
              <Stat label="Body SHA-256" value={e.request_body_sha256} mono />
              <Stat label="Body bytes" value={`${e.request_body_len}`} />
              <Stat label="Attempt" value={`${e.attempt}`} />
            </div>

            <Section title="Signature header">
              <Kv data={e.signature_header} />
            </Section>

            <Section title="Request headers">
              <Kv data={e.request_headers} />
            </Section>

            <Section title="Request body (raw)">
              <CodeBlock code={pretty(decodeBase64(e.request_body))} />
            </Section>

            {e.response_body && (
              <Section title="Response body">
                <CodeBlock code={pretty(decodeBase64(e.response_body))} />
              </Section>
            )}
          </>
        )}
      </main>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mt-6">
      <h2 className="mb-2 text-xs font-semibold uppercase tracking-wide text-gray-500">
        {title}
      </h2>
      {children}
    </section>
  );
}

function Kv({ data }: { data: Record<string, string> }) {
  const entries = Object.entries(data || {});
  if (entries.length === 0) return <p className="text-sm text-gray-600">—</p>;
  return (
    <div className="overflow-hidden rounded-lg border border-edge bg-base">
      {entries.map(([k, v]) => (
        <div key={k} className="flex gap-4 border-b border-edge px-4 py-2 last:border-0">
          <span className="w-56 shrink-0 font-mono text-xs text-gray-400">{k}</span>
          <span className="break-all font-mono text-xs text-gray-200">{v}</span>
        </div>
      ))}
    </div>
  );
}

function pretty(s: string): string {
  if (!s) return "";
  try {
    return JSON.stringify(JSON.parse(s), null, 2);
  } catch {
    return s;
  }
}

