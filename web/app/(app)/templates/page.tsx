"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Badge, CodeBlock, ErrorBanner, PageHeader, Spinner } from "@/components/ui";
import { Template, TemplateDetail, getTemplate, listTemplates } from "@/lib/api";

export default function TemplatesPage() {
  const [templates, setTemplates] = useState<Template[]>([]);
  const [selected, setSelected] = useState("");
  const [detail, setDetail] = useState<TemplateDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [detailLoading, setDetailLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    listTemplates()
      .then((t) => {
        setTemplates(t);
        if (t.length) setSelected(`${t[0].provider}/${t[0].event}`);
      })
      .catch((e: any) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    if (!selected) return;
    setDetailLoading(true);
    const [provider, event] = selected.split("/");
    getTemplate(provider, event)
      .then(setDetail)
      .catch((e: any) => setError(e.message))
      .finally(() => setDetailLoading(false));
  }, [selected]);

  return (
    <div className="min-h-screen">
      <main className="mx-auto max-w-6xl px-6 py-12">
        <PageHeader
          eyebrow="hookreplay / templates"
          title="Template library"
          subtitle="Provider-accurate payloads, signing schemes, and retry schedules."
        />

        {error && (
          <div className="mt-4">
            <ErrorBanner message={error} />
          </div>
        )}

        {loading ? (
          <div className="mt-10 grid place-items-center py-24">
            <Spinner className="h-6 w-6" />
          </div>
        ) : (
          <div className="mt-8 grid gap-6 lg:grid-cols-[1fr_400px]">
            <div className="grid content-start gap-3 sm:grid-cols-2">
              {templates.map((t) => {
                const id = `${t.provider}/${t.event}`;
                const active = id === selected;
                return (
                  <button
                    key={id}
                    onClick={() => setSelected(id)}
                    className={`rounded-xl border p-4 text-left transition ${
                      active ? "border-brand/50 bg-brand/5" : "border-edge bg-panel hover:border-brand/30"
                    }`}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className="font-semibold capitalize text-white">{t.provider}</span>
                      <Badge tone="brand">{t.scheme}</Badge>
                    </div>
                    <div className="mt-1 font-mono text-xs text-gray-400">{t.event}</div>
                    {t.note && <div className="mt-2 line-clamp-2 text-xs text-gray-500">{t.note}</div>}
                  </button>
                );
              })}
            </div>

            <aside className="lg:sticky lg:top-20 lg:self-start">
              {detailLoading ? (
                <div className="grid place-items-center rounded-xl border border-edge bg-panel py-24">
                  <Spinner />
                </div>
              ) : detail ? (
                <div className="space-y-5 rounded-xl border border-edge bg-panel p-5">
                  <div>
                    <div className="flex items-baseline gap-2">
                      <h2 className="text-lg font-semibold capitalize text-white">{detail.provider}</h2>
                      <span className="font-mono text-sm text-gray-400">{detail.event}</span>
                    </div>
                    <div className="mt-2 flex flex-wrap gap-2">
                      <Badge tone="brand">{detail.scheme}</Badge>
                      {detail.signature_header && <Badge tone="neutral">header: {detail.signature_header}</Badge>}
                    </div>
                    {detail.note && <p className="mt-3 text-sm text-gray-400">{detail.note}</p>}
                  </div>

                  {detail.attempt_intervals && detail.attempt_intervals.length > 0 && (
                    <div>
                      <h3 className="text-xs font-semibold uppercase tracking-wide text-gray-500">
                        Retry schedule
                      </h3>
                      <div className="mt-2 space-y-1 font-mono text-xs text-gray-300">
                        <div>
                          {detail.attempt_intervals.map((d, i) => (i === 0 ? "now" : `+${d}`)).join("  →  ")}
                        </div>
                        <div>max attempts: {detail.max_attempts ?? detail.attempt_intervals.length}</div>
                        {!!detail.timeout_s && <div>timeout: {detail.timeout_s}s</div>}
                      </div>
                    </div>
                  )}

                  <div>
                    <h3 className="text-xs font-semibold uppercase tracking-wide text-gray-500">Payload</h3>
                    <div className="mt-2">
                      <CodeBlock code={JSON.stringify(detail.payload ?? {}, null, 2)} />
                    </div>
                  </div>

                  <Link
                    href={`/fire?template=${detail.provider}/${detail.event}`}
                    className="block rounded-md bg-brand px-4 py-2 text-center font-semibold text-black transition hover:bg-emerald-300"
                  >
                    Fire this template →
                  </Link>
                </div>
              ) : (
                <div className="grid place-items-center rounded-xl border border-dashed border-edge py-24 text-center text-sm text-gray-600">
                  Select a template.
                </div>
              )}
            </aside>
          </div>
        )}
      </main>
    </div>
  );
}
