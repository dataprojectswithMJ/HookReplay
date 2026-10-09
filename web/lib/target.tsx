"use client";

import { createContext, useContext, useEffect, useState } from "react";

const STORAGE_KEY = "hookreplay_target_url";

const TargetUrlContext = createContext<[string, (v: string) => void]>([
  "",
  () => {},
]);

export function TargetUrlProvider({ children }: { children: React.ReactNode }) {
  const [url, setUrlState] = useState("");

  useEffect(() => {
    setUrlState(
      window.localStorage.getItem(STORAGE_KEY) ?? "http://localhost:3000/webhook"
    );
  }, []);

  function setUrl(v: string) {
    setUrlState(v);
    try {
      window.localStorage.setItem(STORAGE_KEY, v);
    } catch {}
  }

  return (
    <TargetUrlContext.Provider value={[url, setUrl]}>
      {children}
    </TargetUrlContext.Provider>
  );
}

export function useTargetUrl(): [string, (v: string) => void] {
  return useContext(TargetUrlContext);
}

export function isValidTargetUrl(v: string): boolean {
  if (!v) return false;
  try {
    const u = new URL(v);
    return u.protocol === "http:" || u.protocol === "https:";
  } catch {
    return false;
  }
}
