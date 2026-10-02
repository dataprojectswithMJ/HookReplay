"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import FireForm from "@/components/FireForm";
import ResultStat from "@/components/ResultStat";
import { Execution, Template, fireWebhook, listTemplates } from "@/lib/api";

export default function FirePage() {
  const [templates, setTemplates] = useState<Template[]>([]);
  const [template, setTemplate] = useState("stripe/checkout.session.completed");
  const [target, setTarget] = useState("http://localhost:3000/webhook");
  const [secret, setSecret] = useState("whsec_dev_test_secret");
  const [reason, setReason] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<Execution | null>(null);

  useEffect(() => {
    listTemplates()
      .then((t) => setTemplates(t))
      .catch((e: any) => setError(e.message));
  }, []);

  // Pre-select a template when arriving from the library ("Fire this template").
  useEffect(() => {
    const t = new URLSearchParams(window.location.search).get("template");
    if (t) setTemplate(t);
  }, []);

  async function fire() {
    setLoading(true);
    setError("");
    setResult(null);
    try {
      const exec = await fireWebhook({
        template,
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
    <div className="min-h-screen">
      <main className="mx-auto max-w-6xl px-6 py-12">
        <div className="max-w-2xl">
          <p className="font-mono text-sm text-brand">hookreplay / fire</p>
          <h1 className="mt-2 text-4xl font-bold tracking-tight text-white">
            Test webhooks without the chaos.
          </h1>
          <p className="mt-3 text-gray-400">
            Fire a provider-accurate, correctly-signed webhook at localhost or
            staging in one click.
          </p>
          <div className="mt-5 flex flex-wrap gap-3">
            <Link
              href="/templates"
              className="rounded-md border border-edge px-3 py-1.5 text-sm text-gray-300 transition hover:border-brand/50 hover:text-gray-100"
            >
              Browse templates
            </Link>
            <Link
              href="/docs"
              className="rounded-md border border-edge px-3 py-1.5 text-sm text-gray-300 transition hover:border-brand/50 hover:text-gray-100"
            >
              Getting started
            </Link>
          </div>
        </div>

        <div className="mt-10 grid gap-6 lg:grid-cols-2">
          <FireForm
            templates={templates}
            template={template}
            setTemplate={setTemplate}
            target={target}
            setTarget={setTarget}
            secret={secret}
            setSecret={setSecret}
            reason={reason}
            setReason={setReason}
            loading={loading}
            onFire={fire}
            error={error}
          />

          <section className="rounded-xl border border-edge bg-panel p-6">
            <h2 className="text-sm font-semibold uppercase tracking-wide text-gray-400">
              Result
            </h2>
            {!result && (
              <div className="mt-6 grid place-items-center rounded-lg border border-dashed border-edge py-16 text-sm text-gray-600">
                Fire a webhook to see the result.
              </div>
            )}
            {result && (
              <div className="mt-4 space-y-4">
                <ResultStat
                  label="Status"
                  value={
                    result.error_code
                      ? result.error_code
                      : `HTTP ${result.response_status}`
                  }
                  tone={
                    result.error_code || result.response_status >= 400
                      ? "bad"
                      : "good"
                  }
                />
                <ResultStat label="Latency" value={`${result.latency_ms} ms`} />
                <ResultStat
                  label="Body SHA-256"
                  value={result.request_body_sha256}
                  mono
                />
                <ResultStat
                  label="Body bytes"
                  value={`${result.request_body_len}`}
                />
                <ResultStat
                  label="Signature"
                  value={JSON.stringify(result.signature_header) || "—"}
                  mono
                />
                <Link
                  href={`/executions/${result.id}`}
                  className="inline-block rounded-md border border-brand/40 px-3 py-1.5 text-sm text-brand hover:bg-brand/10"
                >
                  View execution →
                </Link>
              </div>
            )}
          </section>
        </div>
      </main>
    </div>
  );
}
