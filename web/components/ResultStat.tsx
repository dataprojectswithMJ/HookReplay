export default function ResultStat({
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
    tone === "good"
      ? "text-pass"
      : tone === "bad"
      ? "text-fail"
      : "text-textMuted";
  return (
    <div>
      <div className="text-xs text-textFaint">{label}</div>
      <div
        className={`mt-0.5 break-all text-sm ${mono ? "font-mono" : ""} ${toneClass}`}
      >
        {value}
      </div>
    </div>
  );
}
