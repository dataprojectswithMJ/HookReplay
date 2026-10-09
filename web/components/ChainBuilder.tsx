"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import {
  ChainForm,
  ChainStepForm,
  KV,
  compileChainYaml,
  emptyStep,
  parseChainYaml,
} from "@/lib/chainYaml";
import { Template } from "@/lib/api";
import {
  API_METHODS,
  DELIVER_REASONS,
  DELIVER_RESULTS,
  SIGN_ALGORITHMS,
  SIGN_ENCODINGS,
} from "@/lib/chainMeta";
import FieldHelp from "@/components/FieldHelp";

const inputCls =
  "mt-1 w-full rounded-md border border-edge bg-base px-3 py-2 font-mono text-sm text-textMuted outline-none focus:border-edgeBright";
const labelCls = "mt-3 block text-xs text-textFaint";
const toggleBase = "rounded px-3 py-1.5 text-xs font-medium transition";
const blockCls = "mt-2 rounded-md border-l-2 border-edge bg-panel/40 px-3 py-2";

// A repeatable key → value editor for maps like override and headers.
function KvEditor({
  label,
  kvs,
  onChange,
  keyPlaceholder = "key",
  valuePlaceholder = "value",
  help,
}: {
  label: string;
  kvs: KV[];
  onChange: (next: KV[]) => void;
  keyPlaceholder?: string;
  valuePlaceholder?: string;
  help?: React.ReactNode;
}) {
  return (
    <div className="mt-2 rounded-md border-l-2 border-edge bg-base/40 px-3 py-2">
      <div className="flex items-center justify-between">
        <span className="text-xs text-textFaint">
          {label}
          {help}
        </span>
        <button
          onClick={() => onChange([...kvs, { key: "", value: "" }])}
          className="rounded-md border border-edge px-2 py-0.5 font-mono text-xs text-textMuted transition hover:border-edgeBright"
        >
          + add
        </button>
      </div>
      <div className="mt-1.5 space-y-1.5">
        {kvs.map((kv, k) => (
          <div key={k} className="flex gap-1.5">
            <input
              value={kv.key}
              onChange={(e) =>
                onChange(kvs.map((x, j) => (j === k ? { ...x, key: e.target.value } : x)))
              }
              placeholder={keyPlaceholder}
              className="w-2/5 rounded-md border border-edge bg-panel px-2 py-1.5 font-mono text-xs text-textMuted outline-none focus:border-edgeBright"
            />
            <input
              value={kv.value}
              onChange={(e) =>
                onChange(kvs.map((x, j) => (j === k ? { ...x, value: e.target.value } : x)))
              }
              placeholder={valuePlaceholder}
              className="w-3/5 rounded-md border border-edge bg-panel px-2 py-1.5 font-mono text-xs text-textMuted outline-none focus:border-edgeBright"
            />
            <button
              onClick={() => onChange(kvs.filter((_, j) => j !== k))}
              className="shrink-0 rounded-md border border-edge px-1.5 py-0.5 font-mono text-xs text-fail transition hover:border-fail/50"
            >
              ✕
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}

export default function ChainBuilder({
  value,
  onChange,
  templates = [],
}: {
  value: string;
  onChange: (v: string) => void;
  templates?: Template[];
}) {
  const initial = useMemo(() => parseChainYaml(value), []);
  const [mode, setMode] = useState<"form" | "yaml">("form");
  const [form, setForm] = useState<ChainForm>(initial.form);
  const [advanced, setAdvanced] = useState<string[]>(initial.advanced);
  const [bodyOpenSteps, setBodyOpenSteps] = useState<number[]>([]);
  const [deliverOpenSteps, setDeliverOpenSteps] = useState<number[]>([]);
  const lastEmitted = useRef(value);

  const compiled = useMemo(() => compileChainYaml(form), [form]);

  const groupedTemplates = useMemo(() => {
    const m = new Map<string, Template[]>();
    for (const t of templates) {
      const list = m.get(t.provider) ?? [];
      list.push(t);
      m.set(t.provider, list);
    }
    return Array.from(m.entries());
  }, [templates]);

  // Catch external value changes (e.g. loading a saved chain) and re-sync the
  // form. If the chain uses fields the form can't model, drop into YAML mode.
  useEffect(() => {
    if (value === lastEmitted.current) return;
    const r = parseChainYaml(value);
    setForm(r.form);
    setAdvanced(r.advanced);
    if (r.advanced.length > 0) setMode("yaml");
    lastEmitted.current = value;
  }, [value]);

  function sync(next: ChainForm) {
    const y = compileChainYaml(next);
    lastEmitted.current = y;
    setForm(next);
    onChange(y);
  }

  function switchTo(m: "form" | "yaml") {
    if (m === "form") {
      const r = parseChainYaml(value);
      setForm(r.form);
      setAdvanced(r.advanced);
    } else {
      const y = compileChainYaml(form);
      lastEmitted.current = y;
      onChange(y);
    }
    setMode(m);
  }

  function updateStep(i: number, patch: Partial<ChainStepForm>) {
    const steps = form.steps.slice();
    steps[i] = { ...steps[i], ...patch };
    sync({ ...form, steps });
  }
  function addStep() {
    const n = form.steps.length + 1;
    sync({ ...form, steps: [...form.steps, { ...emptyStep(), name: `step-${n}` }] });
  }
  function removeStep(i: number) {
    sync({ ...form, steps: form.steps.filter((_, j) => j !== i) });
    setBodyOpenSteps((idx) => idx.filter((j) => j !== i).map((j) => (j > i ? j - 1 : j)));
    setDeliverOpenSteps((idx) => idx.filter((j) => j !== i).map((j) => (j > i ? j - 1 : j)));
  }

  const hasBodyAssertion = (s: ChainStepForm) =>
    !!(
      s.jsonPath?.trim() ||
      s.equals?.trim() ||
      s.contains?.trim() ||
      (s.exists ?? "").trim()
    );
  const showBody = (i: number, s: ChainStepForm) =>
    bodyOpenSteps.includes(i) || hasBodyAssertion(s);
  const showDeliver = (i: number, s: ChainStepForm) =>
    s.type === "webhook" && (deliverOpenSteps.includes(i) || !!s.deliverResult?.trim());

  function removeBodyAssertion(i: number) {
    updateStep(i, { jsonPath: "", equals: "", contains: "", exists: "" });
    setBodyOpenSteps((idx) => idx.filter((j) => j !== i));
  }
  function removeDeliver(i: number) {
    updateStep(i, { deliverResult: "", deliverReason: "" });
    setDeliverOpenSteps((idx) => idx.filter((j) => j !== i));
  }

  function download() {
    const blob = new Blob([value], { type: "application/x-yaml" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    const base = (form.name || "chain").replace(/[^a-z0-9-]+/gi, "-") || "chain";
    a.download = `${base}.yml`;
    a.click();
    URL.revokeObjectURL(url);
  }

  return (
    <div>
      <div className="flex items-center justify-between gap-2">
        <div className="inline-flex rounded-md border border-edge p-0.5">
          <button
            onClick={() => switchTo("form")}
            className={`${toggleBase} ${
              mode === "form" ? "bg-brand/15 text-brand" : "text-textMuted hover:text-white"
            }`}
          >
            Form
          </button>
          <button
            onClick={() => switchTo("yaml")}
            className={`${toggleBase} ${
              mode === "yaml" ? "bg-brand/15 text-brand" : "text-textMuted hover:text-white"
            }`}
          >
            YAML
          </button>
        </div>
        <button
          onClick={download}
          className="rounded-md border border-edge px-3 py-1.5 font-mono text-xs text-textMuted transition hover:border-edgeBright"
        >
          Download .yml
        </button>
      </div>

      {advanced.length > 0 && (
        <p className="mt-3 rounded-md border border-warn/30 bg-warn/10 px-3 py-2 font-mono text-xs text-warn">
          This chain uses fields the form can&apos;t edit ({advanced.join(", ")}). Switch to YAML to keep them.
        </p>
      )}

      {mode === "yaml" ? (
        <textarea
          value={value}
          onChange={(e) => onChange(e.target.value)}
          spellCheck={false}
          rows={16}
          className="mt-3 w-full rounded-md border border-edge bg-base px-3 py-2 font-mono text-xs text-textMuted outline-none focus:border-edgeBright"
        />
      ) : (
        <div className="mt-3 space-y-3">
          <div>
            <label className="block text-xs text-textFaint">Chain name</label>
            <input
              value={form.name}
              onChange={(e) => sync({ ...form, name: e.target.value })}
              placeholder="payment-onboarding-flow"
              className={inputCls}
            />
          </div>

          {form.steps.map((s, i) => (
            <div key={i} className="rounded-lg border border-edge bg-base p-3">
              <div className="flex items-center gap-2">
                <span className="font-mono text-[11px] text-textFaint">step {i + 1}</span>
                <span className="flex min-w-0 flex-1 items-center">
                  <input
                    value={s.name}
                    onChange={(e) => updateStep(i, { name: e.target.value })}
                    placeholder="step name"
                    className="min-w-0 flex-1 rounded-md border border-edge bg-panel px-2 py-1.5 font-mono text-sm text-textMuted outline-none focus:border-edgeBright"
                  />
                  <FieldHelp
                    title="Step name"
                    items={[
                      { term: "referenced by", desc: "{{steps.<name>.status}}, {{steps.<name>.body.<path>}} in later steps' URLs and assertions" },
                    ]}
                    example="payment-received"
                  />
                </span>
                <select
                  value={s.type}
                  onChange={(e) => updateStep(i, { type: e.target.value as ChainStepForm["type"] })}
                  className="rounded-md border border-edge bg-panel px-2 py-1.5 font-mono text-sm text-textMuted outline-none focus:border-edgeBright"
                >
                  <option value="webhook">webhook</option>
                  <option value="api">api</option>
                  <option value="custom">custom</option>
                </select>
                <button
                  onClick={() => removeStep(i)}
                  className="rounded-md border border-edge px-2 py-1 font-mono text-xs text-fail transition hover:border-fail/50"
                >
                  ✕
                </button>
              </div>

              {s.type === "webhook" && (
                <div className={blockCls}>
                  <label className="block text-xs text-textFaint">
                    webhook
                    <FieldHelp
                      title="Webhook template"
                      items={[
                        { term: "provider/event", desc: "a template from the library" },
                      ]}
                      example="stripe/checkout.session.completed"
                    >
                      Replays a provider-accurate, correctly-signed event. The
                      backend rejects unknown values.
                    </FieldHelp>
                  </label>
                  <select
                    value={s.webhook || ""}
                    onChange={(e) => updateStep(i, { webhook: e.target.value })}
                    className={inputCls}
                  >
                    <option value="">Select a template…</option>
                    {groupedTemplates.map(([provider, list]) => (
                      <optgroup key={provider} label={provider}>
                        {list.map((t) => (
                          <option
                            key={`${t.provider}/${t.event}`}
                            value={`${t.provider}/${t.event}`}
                          >
                            {t.provider}/{t.event}
                          </option>
                        ))}
                      </optgroup>
                    ))}
                  </select>
                </div>
              )}

              {s.type === "api" && (
                <div className={blockCls}>
                  <div className="text-xs text-textFaint">api</div>
                  <label className={labelCls}>
                    method
                    <FieldHelp items={[{ term: API_METHODS.join(" | "), desc: "HTTP method for the call" }]} />
                  </label>
                  <select
                    value={s.method || "GET"}
                    onChange={(e) => updateStep(i, { method: e.target.value })}
                    className={inputCls}
                  >
                    {API_METHODS.map((m) => (
                      <option key={m} value={m}>
                        {m}
                      </option>
                    ))}
                  </select>
                  <label className={labelCls}>
                    url
                    <FieldHelp
                      title="API step URL"
                      items={[
                        { term: "{{target}}", desc: "the run target" },
                        { term: "{{steps.<name>.body.<path>}}", desc: "a prior step value" },
                      ]}
                      example="{{target}}/orders/{{steps.payment-received.body.data.object.id}}"
                    />
                  </label>
                  <input
                    value={s.url || ""}
                    onChange={(e) => updateStep(i, { url: e.target.value })}
                    placeholder="{{target}}/orders/{{steps.payment-received.body.data.object.id}}"
                    className={inputCls}
                  />
                  <KvEditor
                    label="headers"
                    kvs={s.apiHeaders ?? []}
                    onChange={(v) => updateStep(i, { apiHeaders: v })}
                    keyPlaceholder="Authorization"
                    valuePlaceholder="Bearer …"
                  />
                  <label className={labelCls}>
                    timeout
                    <FieldHelp
                      items={[{ term: "500ms | 3s | 1m", desc: "Go duration" }]}
                      example="10s"
                    />
                  </label>
                  <input
                    value={s.apiTimeout || ""}
                    onChange={(e) => updateStep(i, { apiTimeout: e.target.value })}
                    placeholder="10s"
                    className={inputCls}
                  />
                  <label className={labelCls}>
                    body (JSON)
                    <FieldHelp
                      items={[{ term: "inline JSON", desc: "sent as the request body for POST/PUT" }]}
                      example='{"key":"value"}'
                    />
                  </label>
                  <textarea
                    value={s.apiBody || ""}
                    onChange={(e) => updateStep(i, { apiBody: e.target.value })}
                    placeholder='{"key":"value"}'
                    rows={3}
                    className="mt-1 w-full rounded-md border border-edge bg-base px-3 py-2 font-mono text-sm text-textMuted outline-none focus:border-edgeBright"
                  />
                </div>
              )}

              {s.type === "custom" && (
                <div className={blockCls}>
                  <div className="text-xs text-textFaint">custom</div>
                  <label className={labelCls}>
                    payload (JSON)
                    <FieldHelp
                      items={[{ term: "inline JSON", desc: "sent verbatim as the request body" }]}
                      example='{"hello":"world"}'
                    />
                  </label>
                  <textarea
                    value={s.payload || ""}
                    onChange={(e) => updateStep(i, { payload: e.target.value })}
                    placeholder='{"hello":"world"}'
                    rows={3}
                    className="mt-1 w-full rounded-md border border-edge bg-base px-3 py-2 font-mono text-sm text-textMuted outline-none focus:border-edgeBright"
                  />
                  <label className={labelCls}>
                    url (target override)
                    <FieldHelp
                      items={[{ term: "absolute URL | env:NAME", desc: "fire this custom webhook at a URL other than the chain target" }]}
                      example="env:INTERNAL_WEBHOOK_URL"
                    />
                  </label>
                  <input
                    value={s.customUrl || ""}
                    onChange={(e) => updateStep(i, { customUrl: e.target.value })}
                    placeholder="env:INTERNAL_WEBHOOK_URL"
                    className={inputCls}
                  />
                  <KvEditor
                    label="headers"
                    kvs={s.customHeaders ?? []}
                    onChange={(v) => updateStep(i, { customHeaders: v })}
                    keyPlaceholder="X-Custom"
                    valuePlaceholder="value"
                  />
                  <div className="mt-2 rounded-md border-l-2 border-edge bg-base/40 px-3 py-2">
                    <label className="block text-xs text-textFaint">
                      sign
                      <FieldHelp
                        title="Signing (custom)"
                        items={[
                          { term: "algorithm", desc: SIGN_ALGORITHMS.join(" | ") },
                          { term: "secret", desc: "env:NAME reference" },
                          { term: "encoding", desc: SIGN_ENCODINGS.join(" | ") },
                        ]}
                        example={
                          "algorithm: hmac-sha256\nsecret: env:MY_SIGNING_SECRET\nheader: X-Signature"
                        }
                      />
                    </label>
                    <select
                      value={s.algorithm || ""}
                      onChange={(e) => updateStep(i, { algorithm: e.target.value })}
                      className={inputCls}
                    >
                      <option value="">none</option>
                      {SIGN_ALGORITHMS.map((a) => (
                        <option key={a} value={a}>
                          {a}
                        </option>
                      ))}
                    </select>
                    {s.algorithm && s.algorithm !== "none" && (
                      <>
                        <label className={labelCls}>secret (env:NAME)</label>
                        <input
                          value={s.signSecret || ""}
                          onChange={(e) => updateStep(i, { signSecret: e.target.value })}
                          placeholder="env:MY_SIGNING_SECRET"
                          className={inputCls}
                        />
                        <label className={labelCls}>header</label>
                        <input
                          value={s.header || ""}
                          onChange={(e) => updateStep(i, { header: e.target.value })}
                          placeholder="X-Signature"
                          className={inputCls}
                        />
                        <label className={labelCls}>format</label>
                        <input
                          value={s.format || ""}
                          onChange={(e) => updateStep(i, { format: e.target.value })}
                          placeholder="hex | base64"
                          className={inputCls}
                        />
                        <label className={labelCls}>encoding</label>
                        <select
                          value={s.encoding || ""}
                          onChange={(e) => updateStep(i, { encoding: e.target.value })}
                          className={inputCls}
                        >
                          <option value="">—</option>
                          {SIGN_ENCODINGS.map((enc) => (
                            <option key={enc} value={enc}>
                              {enc}
                            </option>
                          ))}
                        </select>
                        <label className={labelCls}>timestamp_header</label>
                        <input
                          value={s.timestampHeader || ""}
                          onChange={(e) => updateStep(i, { timestampHeader: e.target.value })}
                          placeholder="X-Timestamp"
                          className={inputCls}
                        />
                      </>
                    )}
                  </div>
                </div>
              )}

              {s.type === "webhook" && (
                <>
                  <label className={labelCls}>
                    secret (env:NAME)
                    <FieldHelp
                      title="Secret"
                      items={[{ term: "env:NAME", desc: "name only — never a literal value" }]}
                      example="env:STRIPE_WEBHOOK_SECRET"
                    >
                      Resolves from the shell environment, then the workspace
                      secret store.
                    </FieldHelp>
                  </label>
                  <input
                    value={s.secret || ""}
                    onChange={(e) => updateStep(i, { secret: e.target.value })}
                    placeholder="env:STRIPE_WEBHOOK_SECRET"
                    className={inputCls}
                  />
                  <KvEditor
                    label="override (patch template payload)"
                    kvs={s.override ?? []}
                    onChange={(v) => updateStep(i, { override: v })}
                    keyPlaceholder="data.object.amount_total"
                    valuePlaceholder="20000"
                    help={
                      <FieldHelp
                        title="Override template fields"
                        items={[
                          { term: "dotted.path", desc: "patched into the generated payload before signing" },
                        ]}
                        example={"data.object.amount_total: 20000"}
                      />
                    }
                  />
                </>
              )}

              {s.type === "webhook" &&
                (showDeliver(i, s) ? (
                  <div className={blockCls}>
                    <div className="flex items-center justify-between">
                      <span className="text-xs text-textFaint">
                        deliver (failure simulation)
                        <FieldHelp
                          title="Deliver"
                          items={[
                            { term: "result", desc: DELIVER_RESULTS.join(" | ") },
                            { term: "reason", desc: DELIVER_REASONS.join(" | ") },
                          ]}
                          example={"result: fail\nreason: invalid_signature"}
                        />
                      </span>
                      <button
                        onClick={() => removeDeliver(i)}
                        className="rounded-md border border-edge px-1.5 py-0.5 font-mono text-xs text-fail transition hover:border-fail/50"
                      >
                        remove
                      </button>
                    </div>
                    <select
                      value={s.deliverResult || ""}
                      onChange={(e) =>
                        updateStep(i, {
                          deliverResult: e.target.value,
                          deliverReason: e.target.value === "fail" ? s.deliverReason : "",
                        })
                      }
                      className={inputCls}
                    >
                      <option value="">—</option>
                      {DELIVER_RESULTS.map((r) => (
                        <option key={r} value={r}>
                          {r}
                        </option>
                      ))}
                    </select>
                    {s.deliverResult === "fail" && (
                      <>
                        <label className={labelCls}>
                          reason
                          <FieldHelp
                            items={[
                              { term: DELIVER_REASONS.join(" | "), desc: "why the target rejected the delivery" },
                            ]}
                          />
                        </label>
                        <select
                          value={s.deliverReason || ""}
                          onChange={(e) => updateStep(i, { deliverReason: e.target.value })}
                          className={inputCls}
                        >
                          <option value="">Select…</option>
                          {DELIVER_REASONS.map((r) => (
                            <option key={r} value={r}>
                              {r}
                            </option>
                          ))}
                        </select>
                      </>
                    )}
                  </div>
                ) : (
                  <button
                    onClick={() => setDeliverOpenSteps((idx) => [...idx, i])}
                    className="mt-2 rounded-md border border-dashed border-edge px-3 py-1.5 font-mono text-xs text-textMuted transition hover:border-edgeBright"
                  >
                    + Add failure simulation
                  </button>
                ))}

              <div className={blockCls}>
                <div className="text-xs text-textFaint">expect</div>
                <label className={labelCls}>
                  status
                  <FieldHelp
                    title="Expect status"
                    items={[
                      { term: "200", desc: "exact code" },
                      { term: "2xx", desc: "any 2xx" },
                    ]}
                  />
                </label>
                <input
                  value={s.expectStatus || ""}
                  onChange={(e) => updateStep(i, { expectStatus: e.target.value })}
                  placeholder="200"
                  className={inputCls}
                />
                <label className={labelCls}>
                  latency_under
                  <FieldHelp
                    title="Latency ceiling"
                    items={[{ term: "500ms | 3s", desc: "Go duration — step must respond faster than this" }]}
                    example="500ms"
                  />
                </label>
                <input
                  value={s.expectLatencyUnder || ""}
                  onChange={(e) => updateStep(i, { expectLatencyUnder: e.target.value })}
                  placeholder="500ms"
                  className={inputCls}
                />
                <label className={labelCls}>
                  retries
                  <FieldHelp
                    title="Expected retry count"
                    items={[{ term: "3", desc: "how many times the target re-delivers before giving up" }]}
                  />
                </label>
                <input
                  value={s.expectRetries || ""}
                  onChange={(e) => updateStep(i, { expectRetries: e.target.value })}
                  placeholder="3"
                  className={inputCls}
                />
                <label className={labelCls}>
                  delays
                  <FieldHelp
                    title="Expected re-delivery delays"
                    items={[{ term: "5m, 30m, 2h", desc: "comma/space separated" }]}
                    example="5m, 30m"
                  >
                    Backoff schedule driven by this rehearsal. Each observed
                    delay must land within ±20% of the value you set.
                  </FieldHelp>
                </label>
                <input
                  value={s.expectDelays || ""}
                  onChange={(e) => updateStep(i, { expectDelays: e.target.value })}
                  placeholder="5m, 30m"
                  className={inputCls}
                />
                {showBody(i, s) ? (
                  <div className="mt-2 rounded-md border-l-2 border-edge bg-base/40 px-3 py-2">
                    <div className="flex items-center justify-between">
                      <span className="text-xs text-textFaint">body</span>
                      {!hasBodyAssertion(s) && (
                        <button
                          onClick={() => removeBodyAssertion(i)}
                          className="rounded-md border border-edge px-1.5 py-0.5 font-mono text-xs text-fail transition hover:border-fail/50"
                        >
                          remove
                        </button>
                      )}
                    </div>
                  <label className={labelCls}>
                    json_path
                    <FieldHelp
                      title="Response body JSON path"
                      items={[{ term: "data.object.id", desc: "dotted path" }]}
                      example="status"
                    />
                  </label>
                  <input
                    value={s.jsonPath || ""}
                    onChange={(e) => updateStep(i, { jsonPath: e.target.value })}
                    placeholder="status"
                    className={inputCls}
                  />
                  <label className={labelCls}>
                    equals
                    <FieldHelp example="equals: paid" />
                  </label>
                  <input
                    value={s.equals || ""}
                    onChange={(e) => updateStep(i, { equals: e.target.value })}
                    placeholder="paid"
                    className={inputCls}
                  />
                  <label className={labelCls}>
                    contains
                    <FieldHelp example="contains: succeeded" />
                  </label>
                  <input
                    value={s.contains || ""}
                    onChange={(e) => updateStep(i, { contains: e.target.value })}
                    placeholder="succeeded"
                    className={inputCls}
                  />
                  <label className={labelCls}>
                    exists
                    <FieldHelp
                      items={[
                        { term: "true", desc: "path must exist" },
                        { term: "false", desc: "path must be absent" },
                      ]}
                    />
                  </label>
                  <select
                    value={s.exists || ""}
                    onChange={(e) => updateStep(i, { exists: e.target.value })}
                    className={inputCls}
                  >
                    <option value="">—</option>
                    <option value="true">true</option>
                    <option value="false">false</option>
                  </select>
                  </div>
                ) : (
                  <button
                    onClick={() => setBodyOpenSteps((idx) => [...idx, i])}
                    className="mt-2 rounded-md border border-dashed border-edge px-3 py-1.5 font-mono text-xs text-textMuted transition hover:border-edgeBright"
                  >
                    + Add body assertion
                  </button>
                )}
              </div>

              <label className={labelCls}>
                delay
                <FieldHelp
                  items={[{ term: "500ms | 3s | 1m", desc: "Go duration" }]}
                  example="500ms"
                >
                  Waits before this step.
                </FieldHelp>
              </label>
              <input
                value={s.delay || ""}
                onChange={(e) => updateStep(i, { delay: e.target.value })}
                placeholder="500ms"
                className={inputCls}
              />

            </div>
          ))}

          <button
            onClick={addStep}
            className="rounded-md border border-dashed border-edge px-3 py-2 font-mono text-xs text-textMuted transition hover:border-edgeBright"
          >
            + Add step
          </button>

          <div>
            <span className="text-xs text-textFaint">Compiled YAML</span>
            <pre className="mt-1 overflow-x-auto rounded-md border border-edge bg-base p-3 font-mono text-xs leading-relaxed text-textMuted">
              {compiled}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
}

