export type KV = { key: string; value: string };

// ChainStepForm mirrors the §6 step schema 1:1 (same order, same nesting).
// Only values the form genuinely can't represent (file-path custom payloads,
// non-scalar override/equals/contains, top-level target/environment/concurrency)
// are left to YAML mode.
export type ChainStepForm = {
  name: string;
  type: "webhook" | "api" | "custom";
  // webhook
  webhook?: string; // provider/event
  secret?: string; // env:NAME (step-level)
  override?: KV[]; // dotted path -> scalar value
  // api
  method?: string;
  url?: string;
  apiHeaders?: KV[];
  apiTimeout?: string;
  apiBody?: string; // inline JSON body
  // custom
  payload?: string; // inline JSON object text
  customUrl?: string; // target override (defaults to chain target)
  customHeaders?: KV[];
  // custom.sign
  algorithm?: string;
  signSecret?: string; // env:NAME
  header?: string; // signature header name
  format?: string;
  encoding?: string;
  timestampHeader?: string;
  // deliver
  deliverResult?: string; // "success" | "fail"
  deliverReason?: string;
  // expect
  expectStatus?: string; // "200" | "2xx"
  expectLatencyUnder?: string;
  expectRetries?: string;
  expectDelays?: string; // comma/space separated durations
  jsonPath?: string;
  equals?: string;
  contains?: string;
  exists?: string; // "" | "true" | "false"
  // common
  delay?: string;
};

export type ChainForm = {
  name: string;
  steps: ChainStepForm[];
};

export function emptyStep(): ChainStepForm {
  return { name: "", type: "webhook" };
}

export function emptyForm(): ChainForm {
  return { name: "", steps: [] };
}

function needsQuote(s: string): boolean {
  return /[:#\s"'{}[\]&*!|>%@`]/.test(s) || s === "";
}

function yq(s: string): string {
  return needsQuote(s.trim()) ? JSON.stringify(s.trim()) : s.trim();
}

function scalar(v: string): string {
  const t = v.trim();
  if (t === "" || t === "true" || t === "false" || t === "null") return t;
  if (/^-?\d+(\.\d+)?$/.test(t)) return t;
  return yq(t);
}

function inlineJson(s: string): string {
  const t = s.trim();
  try {
    return JSON.stringify(JSON.parse(t)); // minify valid JSON to one line
  } catch {
    return t; // emit raw (may be invalid YAML, but visible to the user)
  }
}

function parseList(s?: string): string[] {
  if (!s) return [];
  return s
    .split(/[\s,]+/)
    .map((x) => x.trim())
    .filter(Boolean);
}

function emitHeaders(L: string[], indent: string, kvs?: KV[]) {
  const rows = (kvs ?? []).filter((kv) => kv.key.trim());
  if (rows.length === 0) return;
  L.push(`${indent}headers:`);
  for (const kv of rows) L.push(`${indent}  ${yq(kv.key.trim())}: ${yq(kv.value.trim())}`);
}

function hasExpect(s: ChainStepForm): boolean {
  return !!(
    s.expectStatus?.trim() ||
    s.expectLatencyUnder?.trim() ||
    s.expectRetries?.trim() ||
    parseList(s.expectDelays).length ||
    s.jsonPath?.trim() ||
    s.equals?.trim() ||
    s.contains?.trim() ||
    (s.exists ?? "").trim()
  );
}

// compileChainYaml renders the form as chain YAML, in schema order.
export function compileChainYaml(f: ChainForm): string {
  const L: string[] = [];
  L.push(`name: ${f.name.trim() ? yq(f.name.trim()) : "unnamed-chain"}`);
  L.push("steps:");
  for (const s of f.steps) {
    // Always emit the step name line (empty name renders as ""), so the YAML
    // visibly updates as steps are added/removed.
    L.push(`  - name: ${s.name.trim() ? yq(s.name.trim()) : '""'}`);

    if (s.type === "webhook") {
      if (s.webhook?.trim()) L.push(`    webhook: ${yq(s.webhook.trim())}`);
    } else if (s.type === "api") {
      L.push("    api:");
      if (s.method?.trim()) L.push(`      method: ${s.method.trim()}`);
      if (s.url?.trim()) L.push(`      url: ${yq(s.url.trim())}`);
      emitHeaders(L, "      ", s.apiHeaders);
      if (s.apiTimeout?.trim()) L.push(`      timeout: ${yq(s.apiTimeout.trim())}`);
      if (s.apiBody?.trim()) L.push(`      body: ${inlineJson(s.apiBody)}`);
    } else if (s.type === "custom") {
      L.push("    custom:");
      if (s.payload?.trim()) L.push(`      payload: ${inlineJson(s.payload)}`);
      if (s.customUrl?.trim()) L.push(`      url: ${yq(s.customUrl.trim())}`);
      emitHeaders(L, "      ", s.customHeaders);
      if (s.algorithm?.trim()) {
        L.push("      sign:");
        L.push(`        algorithm: ${s.algorithm.trim()}`);
        if (s.signSecret?.trim()) L.push(`        secret: ${s.signSecret.trim()}`);
        if (s.header?.trim()) L.push(`        header: ${yq(s.header.trim())}`);
        if (s.format?.trim()) L.push(`        format: ${yq(s.format.trim())}`);
        if (s.encoding?.trim()) L.push(`        encoding: ${s.encoding.trim()}`);
        if (s.timestampHeader?.trim())
          L.push(`        timestamp_header: ${yq(s.timestampHeader.trim())}`);
      }
    }

    if (s.type === "webhook") {
      if (s.secret?.trim()) L.push(`    secret: ${s.secret.trim()}`);
      const ov = (s.override ?? []).filter((kv) => kv.key.trim());
      if (ov.length > 0) {
        L.push("    override:");
        for (const kv of ov) L.push(`      ${yq(kv.key.trim())}: ${scalar(kv.value)}`);
      }
    }

    if (s.type === "webhook" && s.deliverResult?.trim()) {
      L.push("    deliver:");
      L.push(`      result: ${s.deliverResult.trim()}`);
      if (s.deliverReason?.trim()) L.push(`      reason: ${s.deliverReason.trim()}`);
    }

    if (hasExpect(s)) {
      L.push("    expect:");
      if (s.expectStatus?.trim()) L.push(`      status: ${s.expectStatus.trim()}`);
      if (s.expectLatencyUnder?.trim())
        L.push(`      latency_under: ${yq(s.expectLatencyUnder.trim())}`);
      if (s.expectRetries?.trim()) L.push(`      retries: ${s.expectRetries.trim()}`);
      const delays = parseList(s.expectDelays);
      if (delays.length > 0)
        L.push(`      delays: [${delays.map((d) => JSON.stringify(d)).join(", ")}]`);
      if (s.jsonPath?.trim() || s.equals?.trim() || s.contains?.trim() || (s.exists ?? "").trim()) {
        L.push("      body:");
        if (s.jsonPath?.trim()) L.push(`        json_path: ${yq(s.jsonPath.trim())}`);
        if (s.equals?.trim()) L.push(`        equals: ${scalar(s.equals)}`);
        if (s.contains?.trim()) L.push(`        contains: ${scalar(s.contains)}`);
        if ((s.exists ?? "").trim()) L.push(`        exists: ${s.exists!.trim()}`);
      }
    }

    if (s.delay?.trim()) L.push(`    delay: ${yq(s.delay.trim())}`);
  }
  return L.join("\n") + "\n";
}

function unquote(s: string): string {
  const t = s.trim();
  if (t.length >= 2 && ((t[0] === '"' && t[t.length - 1] === '"') || (t[0] === "'" && t[t.length - 1] === "'"))) {
    try {
      return JSON.parse(t);
    } catch {
      return t.slice(1, -1);
    }
  }
  return t;
}

// unquoteList turns a YAML flow list like ["5m", "30m"] into "5m, 30m".
function unquoteList(s: string): string {
  const t = s.trim();
  if (t.startsWith("[") && t.endsWith("]")) {
    return t
      .slice(1, -1)
      .split(",")
      .map((x) => unquote(x))
      .filter(Boolean)
      .join(", ");
  }
  return unquote(t);
}

// parseChainYaml reads the form-modelable subset of a chain back into form
// fields. Anything it can't model is collected in `advanced`, so the UI can
// warn and keep the user in YAML mode rather than silently dropping fields.
export function parseChainYaml(yaml: string): { form: ChainForm; advanced: string[] } {
  const form = emptyForm();
  const advanced: string[] = [];
  const lines = yaml.split("\n");
  let cur: ChainStepForm | null = null;
  let container: string | null = null; // expect | deliver | override | api | custom
  let sub: string | null = null; // body | sign | apiHeaders | customHeaders

  for (const raw of lines) {
    const line = raw.replace(/\r$/, "");
    if (!line.trim() || line.trim().startsWith("#")) continue;
    const indent = line.length - line.trimStart().length;
    const text = line.trim();
    const idx = text.indexOf(":");
    if (idx === -1) {
      advanced.push(text);
      continue;
    }
    let key = text.slice(0, idx).trim();
    const value = text.slice(idx + 1).trim();
    const isListItem = key.startsWith("- ");
    if (isListItem) key = key.slice(2).trim();

    if (indent === 0) {
      container = null;
      sub = null;
      cur = null;
      if (key === "name") form.name = unquote(value);
      else if (key === "steps") {
        /* container */
      } else advanced.push(key);
      continue;
    }

    if (indent === 2 && isListItem && key === "name") {
      container = null;
      sub = null;
      cur = { name: unquote(value), type: "webhook" };
      form.steps.push(cur);
      continue;
    }

    if (!cur) {
      advanced.push(text);
      continue;
    }

    if (indent === 4) {
      container = null;
      sub = null;
      if (key === "webhook") {
        cur.type = "webhook";
        cur.webhook = unquote(value);
      } else if (key === "secret") cur.secret = unquote(value);
      else if (key === "delay") cur.delay = unquote(value);
      else if (key === "expect") container = "expect";
      else if (key === "deliver") container = "deliver";
      else if (key === "override") container = "override";
      else if (key === "api") {
        cur.type = "api";
        container = "api";
      } else if (key === "custom") {
        cur.type = "custom";
        container = "custom";
      } else advanced.push(key);
      continue;
    }

    if (indent === 6) {
      sub = null;
      if (container === "expect") {
        if (key === "status") cur.expectStatus = unquote(value);
        else if (key === "latency_under") cur.expectLatencyUnder = unquote(value);
        else if (key === "retries") cur.expectRetries = unquote(value);
        else if (key === "delays") cur.expectDelays = unquoteList(value);
        else if (key === "body") sub = "body";
        else advanced.push(key);
      } else if (container === "deliver") {
        if (key === "result") cur.deliverResult = unquote(value);
        else if (key === "reason") cur.deliverReason = unquote(value);
        else advanced.push(key);
      } else if (container === "override") {
        cur.override = cur.override ?? [];
        cur.override.push({ key: unquote(key), value: unquote(value) });
      } else if (container === "api") {
        if (key === "method") cur.method = value;
        else if (key === "url") cur.url = unquote(value);
        else if (key === "timeout") cur.apiTimeout = unquote(value);
        else if (key === "body") cur.apiBody = value;
        else if (key === "headers") sub = "apiHeaders";
        else advanced.push(key);
      } else if (container === "custom") {
        if (key === "payload") cur.payload = value;
        else if (key === "url") cur.customUrl = unquote(value);
        else if (key === "headers") sub = "customHeaders";
        else if (key === "sign") sub = "sign";
        else advanced.push(key);
      } else {
        advanced.push(text);
      }
      continue;
    }

    if (indent === 8) {
      if (sub === "body") {
        if (key === "json_path") cur.jsonPath = unquote(value);
        else if (key === "equals") cur.equals = unquote(value);
        else if (key === "contains") cur.contains = unquote(value);
        else if (key === "exists") cur.exists = unquote(value);
        else advanced.push(key);
      } else if (sub === "sign") {
        if (key === "algorithm") cur.algorithm = value;
        else if (key === "secret") cur.signSecret = unquote(value);
        else if (key === "header") cur.header = unquote(value);
        else if (key === "format") cur.format = unquote(value);
        else if (key === "encoding") cur.encoding = unquote(value);
        else if (key === "timestamp_header") cur.timestampHeader = unquote(value);
        else advanced.push(key);
      } else if (sub === "apiHeaders") {
        cur.apiHeaders = cur.apiHeaders ?? [];
        cur.apiHeaders.push({ key: unquote(key), value: unquote(value) });
      } else if (sub === "customHeaders") {
        cur.customHeaders = cur.customHeaders ?? [];
        cur.customHeaders.push({ key: unquote(key), value: unquote(value) });
      } else {
        advanced.push(text);
      }
      continue;
    }

    advanced.push(text);
  }

  // deliver is engine-supported on webhook steps only; preserve it via YAML
  // mode when a pasted chain simulates delivery on an api/custom step.
  for (const s of form.steps) {
    if ((s.deliverResult || s.deliverReason) && s.type !== "webhook") {
      advanced.push("deliver");
      break;
    }
  }

  return { form, advanced };
}
