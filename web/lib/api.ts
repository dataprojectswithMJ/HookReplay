export type Execution = {
  id: string;
  workspace_id: string;
  step_name: string;
  target_kind: string;
  target: string;
  provider: string;
  template_id: string;
  request_body: string; // base64
  request_body_len: number;
  request_body_sha256: string;
  request_headers: Record<string, string>;
  signature_header: Record<string, string>;
  response_status: number;
  response_body: string; // base64
  latency_ms: number;
  attempt: number;
  parent_execution_id?: string;
  error_code?: string;
  created_at: string;
};

export type Template = {
  provider: string;
  event: string;
  scheme: string;
  note: string;
};

export type TemplateDetail = {
  provider: string;
  event: string;
  scheme: string;
  note: string;
  signature_header?: string;
  payload?: unknown;
  attempt_intervals?: string[];
  max_attempts?: number;
  timeout_s?: number;
};

export type Me = {
  user_id: string;
  workspace_id: string;
  tier: string;
  email: string;
};

const KEY = "hookreplay_api_key";
// Dev-only default: the API seeds this key (HOOKREPLAY_DEV_API_KEY) so the
// slice works out of the box. Real deployments use browser OAuth instead.
const DEV_API_KEY = "hrk_dev_local_dev_only_key";

export function getApiKey(): string {
  if (typeof window === "undefined") return "";
  return window.localStorage.getItem(KEY) ?? DEV_API_KEY;
}

export function setApiKey(k: string) {
  window.localStorage.setItem(KEY, k);
}

export function clearApiKey() {
  window.localStorage.removeItem(KEY);
}

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...((init?.headers as Record<string, string>) ?? {}),
  };
  const key = getApiKey();
  if (key) headers["Authorization"] = `Bearer ${key}`;

  const res = await fetch(path, { ...init, headers });
  let data: any = null;
  try {
    data = await res.json();
  } catch {
    data = null;
  }
  if (!res.ok) {
    throw new Error(data?.error?.message ?? `HTTP ${res.status}`);
  }
  return data as T;
}

export function listTemplates() {
  return apiFetch<Template[]>("/v1/templates");
}

export function getTemplate(provider: string, event: string) {
  return apiFetch<TemplateDetail>(`/v1/templates/${provider}/${event}`);
}

export function getMe() {
  return apiFetch<Me>("/v1/me");
}

export function fireWebhook(body: unknown) {
  return apiFetch<Execution>("/v1/dispatch", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

export function listExecutions() {
  return apiFetch<Execution[]>("/v1/executions");
}

export function getExecution(id: string) {
  return apiFetch<Execution>(`/v1/executions/${id}`);
}

export function decodeBase64(b64: string): string {
  if (!b64) return "";
  if (typeof window === "undefined") return "";
  try {
    const bin = window.atob(b64);
    const bytes = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
    return new TextDecoder().decode(bytes);
  } catch {
    return "";
  }
}

export type APIKey = {
  id: string;
  prefix: string;
  name?: string;
  scopes: string[];
  last_used_at?: string;
  created_at: string;
};

export type Workspace = {
  id: string;
  name: string;
  tier: string;
  role: string;
};

export function listAPIKeys() {
  return apiFetch<APIKey[]>("/v1/api-keys");
}

export function createAPIKey(name: string, scopes?: string[]) {
  return apiFetch<{ id: string; api_key: string }>("/v1/api-keys", {
    method: "POST",
    body: JSON.stringify({ name, scopes }),
  });
}

export function deleteAPIKey(id: string) {
  return apiFetch<{ status: string }>(`/v1/api-keys/${id}`, { method: "DELETE" });
}

export function listWorkspaces() {
  return apiFetch<Workspace[]>("/v1/workspaces");
}

export function createWorkspace(name: string) {
  return apiFetch<{ id: string; name: string }>("/v1/workspaces", {
    method: "POST",
    body: JSON.stringify({ name }),
  });
}

// OAuth: the dashboard redirects to the start URL; the CLI polls the result.
export function oauthStartURL(state: string) {
  return `/v1/auth/oauth/start?state=${state}`;
}

export function register(email: string, password: string, state?: string) {
  return apiFetch<{ api_key?: string; status?: string }>("/v1/auth/register", {
    method: "POST",
    body: JSON.stringify({ email, password, state }),
  });
}

export function login(email: string, password: string, state?: string) {
  return apiFetch<{ api_key?: string; status?: string }>("/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password, state }),
  });
}

export function getAuthResult(state: string) {
  return apiFetch<{ api_key: string }>(`/v1/auth/result?state=${state}`);
}

export type ChainStepResult = {
  name: string;
  status: number;
  body?: string; // base64
  latency_ms: number;
  retries?: number;
  delays?: string[];
  error?: string;
  skipped: boolean;
  failed: boolean;
  expect_errors?: string[];
};

export type ChainRunResult = {
  name: string;
  steps: ChainStepResult[];
  passed: boolean;
};

export function executeChain(yamlText: string, target: string, env: string) {
  return apiFetch<ChainRunResult>("/v1/chains/execute", {
    method: "POST",
    body: JSON.stringify({ yaml_text: yamlText, target, environment: env }),
  });
}

export type SecretName = { env: string; name: string; updated_at: string };

export function listSecrets() {
  return apiFetch<SecretName[]>("/v1/secrets");
}

export function setSecret(env: string, name: string, value: string) {
  return apiFetch<{ status: string }>(`/v1/secrets/${env}/${name}`, {
    method: "PUT",
    body: JSON.stringify({ value }),
  });
}

export function deleteSecret(env: string, name: string) {
  return apiFetch<{ status: string }>(`/v1/secrets/${env}/${name}`, {
    method: "DELETE",
  });
}

export type Usage = {
  period: string;
  executions: number;
  replays: number;
  tier: string;
  exec_limit: number | null;
  chains_limit: number;
};

export function getUsage() {
  return apiFetch<Usage>("/v1/usage");
}

export type Domain = {
  id: string;
  domain: string;
  verification_token: string;
  verified_at?: string;
  method: string;
};

export function listDomains() {
  return apiFetch<Domain[]>("/v1/domains");
}

export function createDomain(domain: string) {
  return apiFetch<Domain>("/v1/domains", {
    method: "POST",
    body: JSON.stringify({ domain }),
  });
}

export function verifyDomain(id: string) {
  return apiFetch<{ verified: boolean }>(`/v1/domains/${id}/verify`, {
    method: "POST",
  });
}

export type Chain = {
  id: string;
  source: string;
  name: string;
  yaml_text: string;
  version: number;
  updated_at: string;
};

export function listChains() {
  return apiFetch<Chain[]>("/v1/chains");
}

export function createChain(name: string, yamlText: string) {
  return apiFetch<Chain>("/v1/chains", {
    method: "POST",
    body: JSON.stringify({ name, yaml_text: yamlText }),
  });
}

export function deleteChain(id: string) {
  return apiFetch<{ status: string }>(`/v1/chains/${id}`, { method: "DELETE" });
}

