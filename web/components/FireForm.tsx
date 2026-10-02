import { Template } from "@/lib/api";

const FAILURE_REASONS = [
  { value: "", label: "Success (default)" },
  { value: "invalid_signature", label: "invalid_signature" },
  { value: "missing_signature", label: "missing_signature" },
  { value: "stale_signature", label: "stale_signature" },
  { value: "malformed_payload", label: "malformed_payload" },
  { value: "connection_refused", label: "connection_refused" },
];

export default function FireForm(props: {
  templates: Template[];
  template: string;
  setTemplate: (v: string) => void;
  target: string;
  setTarget: (v: string) => void;
  secret: string;
  setSecret: (v: string) => void;
  reason: string;
  setReason: (v: string) => void;
  loading: boolean;
  onFire: () => void;
  error: string;
}) {
  const inputCls =
    "mt-1 w-full rounded-md border border-edge bg-base px-3 py-2 font-mono text-sm text-gray-200 outline-none focus:border-brand/50";
  const labelCls = "mt-4 block text-xs text-gray-500";

  return (
    <section className="rounded-xl border border-edge bg-panel p-6">
      <h2 className="text-sm font-semibold uppercase tracking-wide text-gray-400">
        Configure
      </h2>

      <label className="mt-5 block text-xs text-gray-500">Template</label>
      <select
        value={props.template}
        onChange={(e) => props.setTemplate(e.target.value)}
        className={inputCls}
      >
        {props.templates.map((t) => (
          <option key={`${t.provider}/${t.event}`} value={`${t.provider}/${t.event}`}>
            {t.provider}/{t.event}
          </option>
        ))}
      </select>

      <label className={labelCls}>Target URL</label>
      <input
        value={props.target}
        onChange={(e) => props.setTarget(e.target.value)}
        className={inputCls}
        placeholder="http://localhost:3000/webhook"
      />

      <label className={labelCls}>Webhook secret</label>
      <input
        type="password"
        value={props.secret}
        onChange={(e) => props.setSecret(e.target.value)}
        className={inputCls}
        placeholder="whsec_..."
      />

      <label className={labelCls}>Failure simulation</label>
      <select
        value={props.reason}
        onChange={(e) => props.setReason(e.target.value)}
        className={inputCls}
      >
        {FAILURE_REASONS.map((r) => (
          <option key={r.value} value={r.value}>
            {r.label}
          </option>
        ))}
      </select>

      <button
        onClick={props.onFire}
        disabled={props.loading}
        className="mt-6 w-full rounded-md bg-brand py-2.5 font-semibold text-black transition hover:bg-emerald-300 disabled:opacity-50"
      >
        {props.loading ? "Firing…" : "Fire webhook"}
      </button>

      {props.error && (
        <p className="mt-4 rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-300">
          {props.error}
        </p>
      )}
    </section>
  );
}
