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
      ? "text-brand"
      : tone === "bad"
      ? "text-red-400"
      : "text-gray-200";
  return (
    <div>
      <div className="text-xs text-gray-500">{label}</div>
      <div
        className={`mt-0.5 break-all text-sm ${mono ? "font-mono" : ""} ${toneClass}`}
      >
        {value}
      </div>
    </div>
  );
}
