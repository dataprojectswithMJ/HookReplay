"use client";

import { useState } from "react";

export function Spinner({ className = "" }: { className?: string }) {
  return (
    <span
      className={`inline-block h-4 w-4 animate-spin rounded-full border-2 border-white/15 border-t-brand ${className}`}
    />
  );
}

export function Button({
  variant = "primary",
  className = "",
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "ghost" | "danger";
}) {
  const base =
    "inline-flex items-center justify-center gap-2 rounded-md px-4 py-2 text-sm font-semibold transition disabled:opacity-50 disabled:cursor-not-allowed";
  const variants = {
    primary: "bg-brand text-black hover:bg-brandBright",
    ghost: "border border-edge text-textMuted hover:border-brand/50 hover:text-white",
    danger: "border border-fail/30 text-fail hover:bg-fail/10",
  };
  return <button className={`${base} ${variants[variant]} ${className}`} {...props} />;
}

export function Badge({
  tone = "neutral",
  children,
}: {
  tone?: "brand" | "amber" | "red" | "neutral" | "accent";
  children: React.ReactNode;
}) {
  const tones = {
    brand: "bg-brand/15 text-brand",
    amber: "bg-warn/15 text-warn",
    red: "bg-fail/15 text-fail",
    neutral: "bg-white/10 text-textMuted",
    accent: "bg-brand/15 text-brand",
  };
  return (
    <span className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${tones[tone]}`}>
      {children}
    </span>
  );
}

export function Card({
  children,
  className = "",
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={`rounded-xl border border-edge bg-panel ${className}`}>{children}</div>
  );
}

export function EmptyState({
  title,
  description,
  action,
}: {
  title: string;
  description?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="grid place-items-center rounded-xl border border-dashed border-edge py-16 text-center">
      <div className="max-w-sm px-6">
        <div className="text-3xl">◌</div>
        <h3 className="mt-3 font-semibold text-white">{title}</h3>
        {description && <p className="mt-1 text-sm text-textFaint">{description}</p>}
        {action && <div className="mt-4">{action}</div>}
      </div>
    </div>
  );
}

export function ErrorBanner({ message }: { message: string }) {
  if (!message) return null;
  return (
    <div className="rounded-md border border-fail/30 bg-fail/10 px-3 py-2 text-sm text-fail">
      {message}
    </div>
  );
}

export function PageHeader({
  eyebrow,
  title,
  subtitle,
  action,
}: {
  eyebrow: string;
  title: string;
  subtitle?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="flex flex-wrap items-end justify-between gap-4">
      <div>
        <p className="font-mono text-sm text-brand">{eyebrow}</p>
        <h1 className="mt-2 text-3xl font-bold tracking-tight text-white">{title}</h1>
        {subtitle && <p className="mt-2 max-w-xl text-textMuted">{subtitle}</p>}
      </div>
      {action && <div className="flex items-center gap-2">{action}</div>}
    </div>
  );
}

export function Stat({
  label,
  value,
  mono = false,
  tone = "neutral",
}: {
  label: string;
  value: string;
  mono?: boolean;
  tone?: "neutral" | "good" | "bad";
}) {
  const toneClass =
    tone === "good" ? "text-pass" : tone === "bad" ? "text-fail" : "text-textMuted";
  return (
    <div className="rounded-lg border border-edge bg-panel p-4">
      <div className="text-xs text-textFaint">{label}</div>
      <div className={`mt-1 break-all text-sm ${mono ? "font-mono" : ""} ${toneClass}`}>
        {value}
      </div>
    </div>
  );
}

export function CopyButton({ text, className = "" }: { text: string; className?: string }) {
  const [copied, setCopied] = useState(false);
  async function copy() {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      const ta = document.createElement("textarea");
      ta.value = text;
      document.body.appendChild(ta);
      ta.select();
      document.execCommand("copy");
      document.body.removeChild(ta);
    }
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }
  return (
    <button
      onClick={copy}
      className={`rounded border border-edge px-2 py-0.5 text-xs text-textMuted transition hover:border-brand/50 hover:text-white ${className}`}
    >
      {copied ? "copied" : "copy"}
    </button>
  );
}

export function CodeBlock({
  code,
  title,
}: {
  code: string;
  title?: string;
}) {
  return (
    <div className="overflow-hidden rounded-lg border border-edge bg-base">
      {title && (
        <div className="flex items-center justify-between border-b border-edge px-3 py-2">
          <span className="font-mono text-xs text-textFaint">{title}</span>
          <CopyButton text={code} />
        </div>
      )}
      {!title && (
        <div className="flex justify-end px-3 pt-2">
          <CopyButton text={code} />
        </div>
      )}
      <pre className="overflow-x-auto p-4 font-mono text-xs leading-relaxed text-textMuted">
        {code}
      </pre>
    </div>
  );
}
