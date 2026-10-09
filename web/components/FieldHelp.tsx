// Structured hover tooltip for intricate chain fields. Pure CSS (no deps) — the
// popover appears above the "?" and is non-interactive so it never flickers.
// Three optional building blocks, rendered in order:
//   title   — a one-line heading
//   items   — mono term + optional description, one per line
//   example — a labeled code snippet
export type HelpItem = { term: string; desc?: string };

export default function FieldHelp({
  title,
  items,
  example,
  children,
}: {
  title?: string;
  items?: HelpItem[];
  example?: string;
  children?: React.ReactNode;
}) {
  return (
    <span className="group/help relative ml-1.5 inline-flex align-middle">
      <span className="grid h-4 w-4 cursor-help select-none place-items-center rounded-full border border-edge text-[10px] leading-none text-textFaint transition hover:border-edgeBright hover:text-white">
        ?
      </span>
      <span className="pointer-events-none absolute bottom-full left-1/2 z-30 mb-1.5 w-72 -translate-x-1/2 rounded-md border border-edge bg-panel p-3 text-left font-sans text-xs font-normal leading-relaxed text-textMuted opacity-0 shadow-xl transition-opacity duration-100 group-hover/help:opacity-100">
        {title && <div className="mb-1.5 font-semibold text-text">{title}</div>}
        {items && items.length > 0 && (
          <ul className="space-y-1">
            {items.map((it) => (
              <li key={it.term}>
                <span className="font-mono text-[11px] text-brand">{it.term}</span>
                {it.desc && <span> — {it.desc}</span>}
              </li>
            ))}
          </ul>
        )}
        {example && (
          <div className="mt-2 border-t border-edge pt-2">
            <div className="mb-1 text-[10px] uppercase tracking-wide text-textFaint">
              Example
            </div>
            <code className="block whitespace-pre-wrap break-all font-mono text-[11px] text-pass">
              {example}
            </code>
          </div>
        )}
        {children && <div className="mt-1.5">{children}</div>}
      </span>
    </span>
  );
}
