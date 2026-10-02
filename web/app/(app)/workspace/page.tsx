"use client";

import { useEffect, useState } from "react";
import {
  Button,
  Card,
  EmptyState,
  ErrorBanner,
  PageHeader,
  Spinner,
} from "@/components/ui";
import { Workspace, createWorkspace, listWorkspaces } from "@/lib/api";

export default function WorkspacePage() {
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);

  function load() {
    listWorkspaces()
      .then(setWorkspaces)
      .catch((e: any) => setError(e.message))
      .finally(() => setLoading(false));
  }
  useEffect(load, []);

  async function onCreate() {
    if (!name.trim()) return;
    setBusy(true);
    setError("");
    try {
      await createWorkspace(name.trim());
      setName("");
      load();
    } catch (e: any) {
      setError(e.message);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="mx-auto max-w-5xl px-6 py-10">
      <PageHeader
        eyebrow="hookreplay / workspace"
        title="Workspaces"
        subtitle="Create and switch between isolated workspaces."
      />

      {error && (
        <div className="mt-4">
          <ErrorBanner message={error} />
        </div>
      )}

      <Card className="mt-6 p-5">
        <label className="block text-xs text-gray-500">New workspace name</label>
        <div className="mt-2 flex gap-2">
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && onCreate()}
            placeholder="my-team"
            className="flex-1 rounded-md border border-edge bg-base px-3 py-2 font-mono text-sm text-gray-200 outline-none focus:border-brand/50"
          />
          <Button onClick={onCreate} disabled={busy || !name.trim()}>
            {busy ? "Creating…" : "Create workspace"}
          </Button>
        </div>
      </Card>

      {loading ? (
        <div className="mt-6 grid place-items-center rounded-xl border border-edge py-20">
          <Spinner />
        </div>
      ) : workspaces.length === 0 ? (
        <div className="mt-6">
          <EmptyState
            title="No workspaces"
            description="Create your first workspace above."
          />
        </div>
      ) : (
        <div className="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {workspaces.map((ws) => (
            <Card key={ws.id} className="p-5">
              <div className="font-semibold text-white">{ws.name}</div>
              <div className="mt-1 font-mono text-xs text-gray-500">{ws.id}</div>
              <div className="mt-3 flex items-center gap-2">
                <span className="rounded-full bg-white/10 px-2.5 py-0.5 text-xs text-gray-300">
                  {ws.tier}
                </span>
                <span className="rounded-full bg-brand/15 px-2.5 py-0.5 text-xs text-brand">
                  {ws.role}
                </span>
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}

