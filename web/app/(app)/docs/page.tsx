import Link from "next/link";

const SECTIONS = [
  { id: "quickstart", label: "Quickstart" },
  { id: "cli", label: "CLI reference" },
  { id: "chains", label: "Chain YAML" },
  { id: "providers", label: "Providers & signing" },
  { id: "concepts", label: "Concepts" },
  { id: "troubleshooting", label: "Troubleshooting" },
];

const TUNNEL_CMD = `hookreplay tunnel --port 3000
# → Tunnel ready: http://localhost:8080/t/hrk-xxxxxxxx/`;

const FIRE_CMD = `export HOOKREPLAY_API_KEY=hrk_dev_local_dev_only_key
hookreplay init          # scaffold hookreplay.yml
hookreplay run           # dispatch through the API`;

const CHAIN_YAML = `name: payment-onboarding-flow
target: \${TUNNEL_URL}
environment: local
steps:
  - name: payment-received
    webhook: stripe/checkout.session.completed
    secret: env:STRIPE_WEBHOOK_SECRET
    expect:
      status: 200
  - name: order-must-exist
    api:
      method: GET
      url: "{{target}}/api/orders/{{steps.payment-received.body.data.object.id}}"
    expect:
      status: 200
      body:
        json_path: status
        equals: paid`;

export default function DocsPage() {
  return (
    <div className="min-h-screen">
      <div className="mx-auto max-w-6xl px-6 py-12 lg:grid lg:grid-cols-[230px_minmax(0,1fr)] lg:gap-12">
        <aside className="hidden lg:block">
          <nav className="sticky top-24 space-y-1">
            {SECTIONS.map((s) => (
              <a
                key={s.id}
                href={`#${s.id}`}
                className="block rounded-md px-3 py-1.5 text-sm text-gray-400 transition-colors hover:bg-white/5 hover:text-gray-200"
              >
                {s.label}
              </a>
            ))}
          </nav>
        </aside>

        <article className="min-w-0">
          <header className="mb-12">
            <p className="font-mono text-sm text-brand">hookreplay / docs</p>
            <h1 className="mt-2 text-4xl font-bold tracking-tight text-white">
              Getting started
            </h1>
            <p className="mt-3 max-w-xl text-gray-400">
              Fire a provider-accurate, correctly-signed webhook at localhost or
              staging in under two minutes — then build multi-step chains to
              rehearse the whole conversation.
            </p>
          </header>

          <Section id="quickstart" title="Quickstart">
            <P>
              The shortest path is five steps: get a key, open a tunnel, fire a
              webhook, verify the signature, and watch it land in the log.
            </P>

            <H3>1. Get an API key</H3>
            <P>
              The local dev build seeds a workspace with the key{" "}
              <Mono>hrk_dev_local_dev_only_key</Mono>. For your own key, use the{" "}
              <Link href="/keys" className="text-brand hover:underline">
                Keys
              </Link>{" "}
              page, or sign in with GitHub:
            </P>
            <Code>hookreplay login</Code>

            <H3>2. Open a tunnel to localhost</H3>
            <P>
              The tunnel relays inbound webhooks to your local handler (here on
              port 3000):
            </P>
            <Code>{TUNNEL_CMD}</Code>
            <P>
              Copy the printed URL — it&apos;s your <Mono>target</Mono>.
            </P>

            <H3>3. Fire a webhook</H3>
            <P>
              From the dashboard{" "}
              <Link href="/fire" className="text-brand hover:underline">
                Fire
              </Link>{" "}
              page, pick a template, paste the tunnel URL as the target, set a
              secret, and hit <strong className="text-white">Fire</strong>. Or
              from the CLI:
            </P>
            <Code>{FIRE_CMD}</Code>

            <H3>4. Verify the signature in your handler</H3>
            <P>
              Payloads are signed at dispatch time over the raw bytes — so your
              handler&apos;s Stripe, GitHub, Paystack, or Slack signature check
              passes without a real provider account.
            </P>

            <H3>5. Watch it in the log</H3>
            <P>
              The{" "}
              <Link href="/log" className="text-brand hover:underline">
                Log
              </Link>{" "}
              page lists every execution with the raw body, signature header,
              SHA-256, and byte length (the trust signals).
            </P>
          </Section>

          <Section id="cli" title="CLI reference">
            <P>
              Install with <Mono>go install ./cmd/cli</Mono> (binary:{" "}
              <Mono>hookreplay</Mono>) — requires Go 1.25+. Sign in with{" "}
              <Mono>hookreplay login --key &lt;your-api-key&gt;</Mono> (no GitHub
              needed) or <Mono>hookreplay login</Mono> (GitHub OAuth). Every
              command accepts <Mono>--api-key</Mono>, <Mono>--api-base</Mono>,
              and <Mono>--json</Mono>.
            </P>
            <Table
              rows={[
                ["login", "Sign in via GitHub OAuth (or --key to store a key)"],
                ["init", "Scaffold a hookreplay.yml"],
                ["fmt [file]", "Normalize a chain to block-style YAML (--check)"],
                ["validate [file]", "Lint a chain against the schema"],
                ["tunnel", "Open a WebSocket tunnel to localhost (--port)"],
                ["run", "Run a hookreplay.yml chain through the API"],
                ["templates list", "List every template"],
                ["templates show <p/e>", "Show one template (provider/event)"],
                ["secrets set/list/delete", "Manage write-only secrets (--env)"],
                ["api-keys create/list/revoke", "Manage API keys"],
                ["workspaces list", "List workspaces"],
                ["keys generate", "Generate an Ed25519 keypair (custom sign)"],
                ["chains push/list/pull", "Sync chains with the workspace"],
              ]}
            />
          </Section>

          <Section id="chains" title="Chain YAML">
            <P>
              A chain is a <Mono>hookreplay.yml</Mono> that rehearses a
              multi-step conversation — webhook in, API calls to your app,
              custom signed events out. Steps run in order;{" "}
              <Mono>when</Mono> gates a step, <Mono>expect</Mono> asserts its
              outcome.
            </P>
            <Code>{CHAIN_YAML}</Code>
            <Table
              rows={[
                ["steps[].webhook", "provider/event to sign and fire"],
                ["steps[].api", "HTTP call to your app (method, url, headers)"],
                ["steps[].custom", "Local payload with a custom sign block"],
                ["secret", "env:NAME or an inline value"],
                ["override", "Dotted-path patches to the template payload"],
                ["delay", "Wait before this step (e.g. 3s)"],
                ["deliver", "Simulate a failure: invalid_signature, malformed_payload, …"],
                ["when", "Run only if: ==, !=, contains"],
                ["expect.status", "int or range (2xx)"],
                ["expect.latency_under", "Fail if the step was slower than this"],
                ["expect.retries", "Assert the re-delivery count (rehearsed)"],
                ["expect.delays", "Assert the exact backoff sequence"],
                ["expect.body", "json_path + equals / contains / exists"],
              ]}
            />
            <P>
              Steps can read earlier steps:{" "}
              <Mono>{"{{steps.payment-received.body.data.object.id}}"}</Mono> and{" "}
              <Mono>{"{{target}}"}</Mono>.
            </P>
          </Section>

          <Section id="providers" title="Providers & signing">
            <P>
              Templates bundle a payload, a <Mono>sign.yml</Mono> scheme, and a{" "}
              <Mono>delivery.yml</Mono> retry schedule. Every payload is signed
              at dispatch time, and a separate <Mono>Verify</Mono> implementation
              checks each scheme in tests.
            </P>
            <Table
              rows={[
                ["stripe", "checkout.session.completed — HMAC-SHA256"],
                ["github", "push — HMAC-SHA256"],
                ["paystack", "charge.success — HMAC-SHA512 (hex)"],
                ["slack", "event_callback — HMAC-SHA256 (v0)"],
                ["meta", "leadgen — HMAC-SHA256"],
                ["razorpay", "payment.captured — HMAC-SHA256"],
                ["flutterwave", "charge.completed — HMAC-SHA256"],
                ["resend", "email.delivered — Ed25519"],
              ]}
            />
            <P>
              Additional signing schemes are available for custom steps —{" "}
              <Mono>klarna</Mono>, <Mono>adyen</Mono>, and a generic{" "}
              <Mono>custom</Mono> block (HMAC-SHA1/256/512, Ed25519, or none).
              Generate a keypair with <Mono>hookreplay keys generate</Mono>.
            </P>
          </Section>

          <Section id="concepts" title="Concepts">
            <Table
              rows={[
                ["Executions", "Every fire is stored byte-exact: raw body, signature, SHA-256, byte length."],
                ["Secrets", "Write-only, AES-256-GCM encrypted at rest. Values are never returned."],
                ["API keys", "Scoped: dispatch, read, secrets, admin. Create once, shown once."],
                ["Workspaces", "Isolated environment + tier + secret scope."],
                ["Retry rehearsal", "expect.retries/delays re-fires per the provider's delivery.yml backoff and asserts what you observe."],
                ["Tunnel", "WebSocket relay that forwards to your localhost handler."],
              ]}
            />
          </Section>

          <Section id="troubleshooting" title="Troubleshooting">
            <Table
              rows={[
                ["403 tier_required", "Free tier fires at tunnel targets only; verify a domain to fire at public URLs."],
                ["signature mismatch", "The secret used at dispatch must match your handler's verification key."],
                ["secret_resolution_failed", "The env:NAME secret isn't set in the shell or workspace."],
                ["No API key", "Set it in the top-right field, or run `hookreplay login`."],
                ["tunnel shows nothing", "Run `hookreplay tunnel --port <your handler port>` and use the printed URL as the target."],
              ]}
            />
          </Section>
        </article>
      </div>
    </div>
  );
}

function Section({
  id,
  title,
  children,
}: {
  id: string;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section id={id} className="mb-16 scroll-mt-24">
      <h2 className="text-2xl font-semibold tracking-tight text-white">
        {title}
      </h2>
      <div className="mt-4 space-y-4">{children}</div>
    </section>
  );
}

function H3({ children }: { children: React.ReactNode }) {
  return (
    <h3 className="pt-2 text-base font-semibold text-gray-100">{children}</h3>
  );
}

function P({ children }: { children: React.ReactNode }) {
  return <p className="leading-relaxed text-gray-300">{children}</p>;
}

function Mono({ children }: { children: React.ReactNode }) {
  return (
    <code className="rounded bg-white/5 px-1.5 py-0.5 font-mono text-[0.85em] text-brand">
      {children}
    </code>
  );
}

function Code({ children }: { children: string }) {
  return (
    <pre className="overflow-x-auto rounded-lg border border-edge bg-base p-4 font-mono text-xs leading-relaxed text-gray-200">
      {children}
    </pre>
  );
}

function Table({ rows }: { rows: [string, string][] }) {
  return (
    <div className="overflow-x-auto rounded-lg border border-edge">
      <table className="w-full text-left text-sm">
        <tbody className="divide-y divide-edge">
          {rows.map(([cmd, desc]) => (
            <tr key={cmd}>
              <td className="whitespace-nowrap px-4 py-2.5 font-mono text-xs text-brand">
                {cmd}
              </td>
              <td className="px-4 py-2.5 text-gray-300">{desc}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
