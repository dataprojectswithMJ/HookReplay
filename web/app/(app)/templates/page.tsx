"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import ResultStat from "@/components/ResultStat";
import { CodeBlock } from "@/components/ui";
import {
  Execution,
  Template,
  TemplateDetail,
  deliveryFailed,
  deliveryLabel,
  fireWebhook,
  getTemplate,
  listTemplates,
} from "@/lib/api";
import { useTargetUrl } from "@/lib/target";
import { DELIVER_REASONS } from "@/lib/chainMeta";

const SECRET_DEFAULT = "whsec_dev_test_secret";

const inputCls =
  "mt-1 w-full rounded-md border border-edge bg-base px-3 py-2 font-mono text-sm text-textMuted outline-none focus:border-edgeBright";
const labelCls = "mt-4 block text-xs text-textFaint";

function TemplateCard({ template }: { template: Template }) {
  const id = `${template.provider}/${template.event}`;
  const [open, setOpen] = useState(false);
  const [detail, setDetail] = useState<TemplateDetail | null>(null);
  const [target] = useTargetUrl();
  const [secret, setSecret] = useState(SECRET_DEFAULT);
  const [reason, setReason] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<Execution | null>(null);

  useEffect(() => {
    if (!open || detail) return;
    const [provider, event] = id.split("/");
    getTemplate(provider, event)
      .then(setDetail)
      .catch(() => setDetail(null));
  }, [open, id, detail]);

  async function fire() {
    setLoading(true);
    setError("");
    setResult(null);
    try {
      const exec = await fireWebhook({
        template: id,
        target,
        secret,
        deliver: reason === "" ? undefined : { result: "fail", reason },
      });
      setResult(exec);
    } catch (e: any) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div
      className={`rounded-xl border bg-panel transition-colors ${
        open ? "border-edgeBright" : "border-edge hover:border-edgeBright"
      }`}
    >
      <button
        onClick={() => setOpen(!open)}
        className="flex w-full items-center gap-3 px-4 py-3 text-left"
      >
        <span className="font-mono text-xs text-textFaint">
          {open ? "▾" : "▸"}
        </span>
        <span className="font-semibold capitalize text-text">{template.provider}</span>
        <span className="truncate font-mono text-xs text-textMuted">{template.event}</span>
        <span className="ml-auto shrink-0 font-mono text-[11px] text-textFaint">
          {template.scheme}
        </span>
      </button>

      {open && (
        <div className="border-t border-edge p-4">
          {detail ? (
            <>
              {detail.note && <p className="text-sm text-textMuted">{detail.note}</p>}
              {detail.attempt_intervals && detail.attempt_intervals.length > 0 && (
                <p className="mt-2 font-mono text-[11px] text-textFaint">
                  retries {detail.max_attempts ?? 0} ×{" "}
                  {detail.attempt_intervals.join(", ")}
                  {detail.timeout_s ? ` · ${detail.timeout_s}s timeout` : ""}
                </p>
              )}
              <div className="mt-3">
                <CodeBlock
                  code={JSON.stringify(detail.payload ?? {}, null, 2)}
                  title={`${detail.provider}/${detail.event}`}
                />
              </div>
            </>
          ) : (
            <p className="font-mono text-xs text-textFaint">loading…</p>
          )}

          <div className="mt-4 grid gap-6 lg:grid-cols-2">
            <section className="rounded-xl border border-edge bg-panel p-5">
              <h2 className="text-sm font-semibold uppercase tracking-wide text-textMuted">
                Configure
              </h2>
              <label className={labelCls}>Webhook secret</label>
              <input
                type="password"
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
                placeholder="whsec_..."
                className={inputCls}
              />
              <label className={labelCls}>Failure simulation</label>
              <select
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                className={inputCls}
              >
                <option value="">Success (default)</option>
                {DELIVER_REASONS.map((r) => (
                  <option key={r} value={r}>
                    {r}
                  </option>
                ))}
              </select>
              <button
                onClick={fire}
                disabled={loading}
                className="mt-6 w-full rounded-md bg-brand py-2.5 font-semibold text-black transition hover:bg-brandBright disabled:opacity-50"
              >
                {loading ? "Firing…" : "Fire webhook"}
              </button>
              {error && (
                <p className="mt-4 rounded-md border border-fail/30 bg-fail/10 px-3 py-2 font-mono text-xs text-fail">
                  {error}
                </p>
              )}
            </section>

            <section className="rounded-xl border border-edge bg-panel p-5">
              <h2 className="text-sm font-semibold uppercase tracking-wide text-textMuted">
                Result
              </h2>
              {!result ? (
                <div className="mt-6 grid place-items-center rounded-lg border border-dashed border-edge py-16 font-mono text-xs text-textFaint">
                  Fire a webhook to see the result.
                </div>
              ) : (
                <div className="mt-4 space-y-4">
                  <ResultStat
                    label="Status"
                    value={
                      result.error_code
                        ? result.error_code
                        : `HTTP ${result.response_status}`
                    }
                    mono
                    tone={
                      result.error_code ||
                      deliveryFailed(result.deliver) ||
                      result.response_status >= 400
                        ? "bad"
                        : "good"
                    }
                  />
                  <ResultStat
                    label="Delivery"
                    value={
                      deliveryFailed(result.deliver)
                        ? deliveryLabel(result.deliver)
                        : "success"
                    }
                    mono
                    tone={deliveryFailed(result.deliver) ? "bad" : "good"}
                  />
                  <ResultStat label="Latency" value={`${result.latency_ms} ms`} mono />
                  <ResultStat
                    label="Body SHA-256"
                    value={result.request_body_sha256}
                    mono
                  />
                  <ResultStat
                    label="Body bytes"
                    value={`${result.request_body_len}`}
                    mono
                  />
                  <ResultStat
                    label="Signature"
                    value={JSON.stringify(result.signature_header) || "—"}
                    mono
                  />
                  <Link
                    href={`/executions/${result.id}`}
                    className="inline-block rounded-md border border-edge px-3 py-1.5 font-mono text-xs text-textMuted transition hover:border-edgeBright"
                  >
                    View execution →
                  </Link>
                </div>
              )}
            </section>
          </div>
        </div>
      )}
    </div>
  );
}

export default function TemplatesPage() {
  const [templates, setTemplates] = useState<Template[]>([]);

  useEffect(() => {
    listTemplates()
      .then(setTemplates)
      .catch(() => {});
  }, []);

  return (
    <div className="min-h-screen">
      <main className="mx-auto max-w-6xl px-6 py-12">
        <p className="font-mono text-sm text-textFaint">hookreplay / templates</p>
        <h1 className="mt-2 text-3xl font-semibold tracking-tight text-text">
          Templates
        </h1>
        <p className="mt-2 max-w-2xl text-textMuted">
          Pick a provider event to preview its payload and fire a correctly-signed
          webhook at your target. Expand a card to configure a secret, simulate a
          failure, and see the result.
        </p>

        <div className="mt-8 space-y-2">
          {templates.map((t) => (
            <TemplateCard key={`${t.provider}/${t.event}`} template={t} />
          ))}
        </div>
      </main>
    </div>
  );
}
