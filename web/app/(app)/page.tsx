"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Badge, Card, PageHeader, Spinner } from "@/components/ui";
import {
  Execution,
  Usage,
  getUsage,
  listChains,
  listExecutions,
  listSecrets,
  listTemplates,
} from "@/lib/api";

export default function DashboardPage() {
  const [execs, setExecs] = useState<Execution[]>([]);
  const [templates, setTemplates] = useState(0);
  const [chains, setChains] = useState(0);
  const [secrets, setSecrets] = useState(0);
  const [usage, setUsage] = useState<Usage | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    Promise.allSettled([
      listExecutions(),
      listTemplates(),
      listChains(),
      listSecrets(),
      getUsage(),
    ]).then(([e, t, c, s, u]) => {
      if (e.status === "fulfilled") setExecs(e.value.slice(0, 5));
      if (t.status === "fulfilled") setTemplates(t.value.length);
      if (c.status === "fulfilled") setChains(c.value.length);
      if (s.status === "fulfilled") setSecrets(s.value.length);
      if (u.status === "fulfilled") setUsage(u.value);
      setLoading(false);
    });
  }, []);

  const stats = [
    { label: "Executions (30d)", value: usage ? String(usage.executions) : "—" },
    { label: "Templates", value: String(templates) },
    { label: "Chains", value: String(chains) },
    { label: "Secrets", value: String(secrets) },
  ];

  return (
    <div className="mx-auto max-w-6xl px-6 py-10">
      <PageHeader
        eyebrow="hookreplay / dashboard"
        title="Overview"
        subtitle="Your webhook testing at a glance."
      />

      <div className="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {stats.map((s) => (
          <Card key={s.label} className="p-5">
            <div className="text-xs text-gray-500">{s.label}</div>
            <div className="mt-1 text-2xl font-semibold text-white">{s.value}</div>
          </Card>
        ))}
      </div>

      <div className="mt-6 grid gap-6 lg:grid-cols-2">
        <Card className="p-5">
          <div className="flex items-center justify-between">
            <h2 className="font-semibold text-white">Recent executions</h2>
            <Link href="/log" className="text-sm text-brand hover:underline">
              View all
            </Link>
          </div>
          {loading ? (
            <div className="grid place-items-center py-16">
              <Spinner />
            </div>
          ) : execs.length === 0 ? (
            <p className="mt-4 text-sm text-gray-600">
              No executions yet.{" "}
              <Link href="/fire" className="text-brand hover:underline">
                Fire your first webhook
              </Link>
              .
            </p>
          ) : (
            <ul className="mt-4 divide-y divide-edge">
              {execs.map((e) => (
                <li key={e.id}>
                  <Link
                    href={`/executions/${e.id}`}
                    className="flex items-center justify-between gap-2 py-2.5"
                  >
                    <span className="truncate font-mono text-sm text-gray-200">
                      {e.template_id || e.step_name}
                    </span>
                    <span className="flex shrink-0 items-center gap-2">
                      <Badge tone={e.error_code || e.response_status >= 400 ? "red" : "brand"}>
                        {e.error_code || `HTTP ${e.response_status}`}
                      </Badge>
                      <span className="text-xs text-gray-500">
                        {new Date(e.created_at).toLocaleTimeString()}
                      </span>
                    </span>
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </Card>

        <Card className="p-5">
          <h2 className="font-semibold text-white">Quick actions</h2>
          <div className="mt-4 grid gap-3 sm:grid-cols-2">
            <Action href="/fire" title="Fire a webhook" desc="Sign and dispatch a single event" />
            <Action href="/chains" title="Run a chain" desc="Rehearse a multi-step flow" />
            <Action href="/templates" title="Browse templates" desc="Stripe, GitHub, Slack, and more" />
            <Action href="/docs" title="Read the docs" desc="Getting started guide" />
          </div>
        </Card>
      </div>
    </div>
  );
}

function Action({ href, title, desc }: { href: string; title: string; desc: string }) {
  return (
    <Link
      href={href}
      className="rounded-lg border border-edge p-4 transition hover:border-brand/40 hover:bg-white/[0.02]"
    >
      <div className="font-medium text-gray-100">{title}</div>
      <div className="mt-1 text-xs text-gray-500">{desc}</div>
    </Link>
  );
}
