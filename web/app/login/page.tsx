"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { getAuthResult, login, register, setUserToken } from "@/lib/api";

export default function LoginPage() {
  const router = useRouter();
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [cliState, setCliState] = useState("");
  const [showDone, setShowDone] = useState(false);

  useEffect(() => {
    const p = new URLSearchParams(window.location.search);
    const state = p.get("state") || "";
    const done = p.get("done") === "1";
    setCliState(state);
    if (state.startsWith("cli_") && done) setShowDone(true);
  }, []);

  useEffect(() => {
    if (!cliState.startsWith("web_")) return;
    const timer = setInterval(async () => {
      try {
        const res = await getAuthResult(cliState);
        if (res.user_token) {
          setUserToken(res.user_token);
          router.push("/");
        }
      } catch {
        // key not ready yet
      }
    }, 1000);
    return () => clearInterval(timer);
  }, [cliState, router]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const state = cliState.startsWith("cli_") ? cliState : undefined;
      const res =
        mode === "register"
          ? await register(email.trim(), password, state)
          : await login(email.trim(), password, state);
      if (res.user_token) {
        setUserToken(res.user_token);
        router.push("/");
      } else {
        setShowDone(true);
      }
    } catch (err: any) {
      setError(err.message);
    } finally {
      setBusy(false);
    }
  }

  function oauth(provider: "google" | "github") {
    const state = cliState.startsWith("cli_") ? cliState : "web_" + crypto.randomUUID();
    window.location.href = `/v1/auth/oauth/start?provider=${provider}&state=${state}`;
  }

  if (showDone) {
    return (
      <div className="grid min-h-screen place-items-center px-6">
        <div className="w-full max-w-sm text-center">
          <div className="mx-auto grid h-12 w-12 place-items-center rounded-full bg-brand/15 text-2xl text-brand">
            ✓
          </div>
          <h1 className="mt-4 text-xl font-bold text-white">You&apos;re signed in</h1>
          <p className="mt-2 text-sm text-gray-400">
            Return to your terminal — the CLI detects this automatically. You can
            close this tab.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="grid min-h-screen place-items-center px-6">
      <div className="w-full max-w-sm">
        <div className="flex items-center gap-2">
          <span className="grid h-9 w-9 place-items-center rounded-lg bg-brand/15 text-brand">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none">
              <path
                d="M13 2 4 14h6l-1 8 9-12h-6l1-8Z"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinejoin="round"
              />
            </svg>
          </span>
          <span className="text-lg font-semibold tracking-tight text-white">HookReplay</span>
        </div>

        <h1 className="mt-8 text-2xl font-bold text-white">
          {mode === "login" ? "Sign in" : "Create your account"}
        </h1>
        <p className="mt-2 text-sm text-gray-400">
          Fire provider-accurate, correctly-signed webhooks at localhost and staging.
        </p>

        <div className="mt-6 grid grid-cols-2 gap-3">
          <button
            onClick={() => oauth("google")}
            className="rounded-md border border-edge py-2.5 text-sm font-medium text-gray-200 transition hover:bg-white/5"
          >
            Google
          </button>
          <button
            onClick={() => oauth("github")}
            className="rounded-md border border-edge py-2.5 text-sm font-medium text-gray-200 transition hover:bg-white/5"
          >
            GitHub
          </button>
        </div>

        <div className="my-6 flex items-center gap-3 text-xs text-gray-600">
          <span className="h-px flex-1 bg-edge" />
          or with email
          <span className="h-px flex-1 bg-edge" />
        </div>

        <form onSubmit={submit} className="space-y-3">
          <div>
            <label className="block text-xs text-gray-500">Email</label>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@example.com"
              className="mt-1 w-full rounded-md border border-edge bg-panel px-3 py-2 text-sm text-gray-200 outline-none focus:border-brand/50"
            />
          </div>
          <div>
            <label className="block text-xs text-gray-500">Password</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder={mode === "register" ? "min 8 characters" : "••••••••"}
              className="mt-1 w-full rounded-md border border-edge bg-panel px-3 py-2 text-sm text-gray-200 outline-none focus:border-brand/50"
            />
          </div>
          {error && <p className="text-sm text-red-300">{error}</p>}
          <button
            type="submit"
            disabled={busy}
            className="w-full rounded-md bg-brand py-2.5 font-semibold text-black transition hover:bg-emerald-300 disabled:opacity-50"
          >
            {busy ? "Please wait…" : mode === "login" ? "Sign in" : "Create account"}
          </button>
        </form>

        <p className="mt-4 text-center text-sm text-gray-500">
          {mode === "login" ? (
            <>
              No account?{" "}
              <button onClick={() => setMode("register")} className="text-brand hover:underline">
                Create one
              </button>
            </>
          ) : (
            <>
              Already have an account?{" "}
              <button onClick={() => setMode("login")} className="text-brand hover:underline">
                Sign in
              </button>
            </>
          )}
        </p>

        <p className="mt-6 text-xs text-gray-600">
          Prefer the CLI? Run{" "}
          <code className="rounded bg-white/5 px-1.5 py-0.5 font-mono text-[0.85em] text-brand">
            hookreplay login
          </code>{" "}
          and it will open this page.
        </p>
      </div>
    </div>
  );
}

