"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { Me, clearApiKey, getMe } from "@/lib/api";

const NAV: { section: string; items: { href: string; label: string; exact?: boolean }[] }[] = [
  {
    section: "Overview",
    items: [
      { href: "/", label: "Dashboard", exact: true },
      { href: "/fire", label: "Fire" },
      { href: "/templates", label: "Templates" },
      { href: "/log", label: "Log" },
      { href: "/chains", label: "Chains" },
    ],
  },
  {
    section: "Workspace",
    items: [
      { href: "/keys", label: "API keys" },
      { href: "/secrets", label: "Secrets" },
      { href: "/workspace", label: "Workspaces" },
      { href: "/usage", label: "Usage" },
      { href: "/domains", label: "Domains" },
    ],
  },
  {
    section: "Help",
    items: [{ href: "/docs", label: "Getting started" }],
  },
];

export default function Sidebar() {
  const pathname = usePathname();
  const [me, setMe] = useState<Me | null>(null);

  useEffect(() => {
    getMe()
      .then(setMe)
      .catch(() => setMe(null));
  }, []);

  function signOut() {
    clearApiKey();
    window.location.href = "/login";
  }

  return (
    <aside className="sticky top-0 flex h-screen w-60 shrink-0 flex-col border-r border-edge bg-panel/40">
      <Link href="/" className="flex h-14 items-center gap-2 border-b border-edge px-4">
        <span className="grid h-7 w-7 place-items-center rounded-md bg-brand/15 text-brand">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
            <path
              d="M13 2 4 14h6l-1 8 9-12h-6l1-8Z"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinejoin="round"
            />
          </svg>
        </span>
        <span className="font-semibold tracking-tight text-white">HookReplay</span>
      </Link>

      <nav className="flex-1 overflow-y-auto px-3 py-4">
        {NAV.map((group) => (
          <div key={group.section} className="mb-6">
            <p className="px-2 pb-2 text-[11px] font-semibold uppercase tracking-wider text-gray-600">
              {group.section}
            </p>
            <ul className="space-y-0.5">
              {group.items.map((item) => {
                const active = item.exact
                  ? pathname === item.href
                  : pathname.startsWith(item.href);
                return (
                  <li key={item.href}>
                    <Link
                      href={item.href}
                      className={`block rounded-md px-2.5 py-2 text-sm transition-colors ${
                        active
                          ? "bg-brand/10 text-brand"
                          : "text-gray-400 hover:bg-white/5 hover:text-gray-200"
                      }`}
                    >
                      {item.label}
                    </Link>
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </nav>

      <div className="border-t border-edge p-4">
        {me ? (
          <div className="flex items-center gap-2">
            <span className="rounded-full bg-brand/15 px-2 py-0.5 text-xs text-brand">
              {me.tier}
            </span>
            <span className="truncate text-xs text-gray-500">
              {me.email || me.workspace_id}
            </span>
          </div>
        ) : (
          <Link href="/login" className="text-sm text-gray-400 hover:text-brand">
            Sign in
          </Link>
        )}
        <button
          onClick={signOut}
          className="mt-3 block text-xs text-gray-500 transition-colors hover:text-gray-300"
        >
          Sign out
        </button>
      </div>
    </aside>
  );
}
