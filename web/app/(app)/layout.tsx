"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import Sidebar from "@/components/Sidebar";
import { TargetUrlProvider, isValidTargetUrl, useTargetUrl } from "@/lib/target";
import { clearApiKey, getMe, hasValidSession } from "@/lib/api";

function TargetUrlBar() {
  const [url, setUrl] = useTargetUrl();
  const invalid = url !== "" && !isValidTargetUrl(url);
  return (
    <div className="border-b border-edge bg-panel/40">
      <div className="mx-auto flex max-w-6xl items-center gap-3 px-6 py-2.5">
        <label
          htmlFor="global-target-url"
          className="font-mono text-[11px] uppercase tracking-wider text-textFaint"
        >
          Target
        </label>
        <input
          id="global-target-url"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          spellCheck={false}
          placeholder="http://localhost:3000/webhook"
          className={`min-w-0 flex-1 rounded-md border bg-base px-3 py-1.5 font-mono text-xs text-textMuted outline-none transition-colors ${
            invalid
              ? "border-fail/60 focus:border-fail"
              : "border-edge focus:border-edgeBright"
          }`}
        />
        {invalid && (
          <span className="shrink-0 font-mono text-[11px] text-fail">
            invalid URL
          </span>
        )}
      </div>
    </div>
  );
}

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();

  useEffect(() => {
    let cancelled = false;

    function redirectIfUnauthenticated() {
      if (!hasValidSession()) {
        clearApiKey();
        router.replace("/login");
      }
    }

    redirectIfUnauthenticated();
    const id = setInterval(redirectIfUnauthenticated, 30 * 1000);

    // Verify against the server once on mount so a session revoked or expired
    // server-side (e.g. TTL drift) is caught immediately.
    getMe().catch(() => {
      if (!cancelled) {
        clearApiKey();
        router.replace("/login");
      }
    });

    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [router]);

  return (
    <TargetUrlProvider>
      <div className="flex min-h-screen">
        <Sidebar />
        <div className="flex min-w-0 flex-1 flex-col">
          <TargetUrlBar />
          <div className="min-w-0 flex-1">{children}</div>
        </div>
      </div>
    </TargetUrlProvider>
  );
}
