import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        base: "#0A0B0D",
        panel: "#121316",
        surface2: "#17191D",
        edge: "#262A31",
        edgeBright: "#33383F",
        brand: "#FF9E40",
        brandBright: "#FFAB5E",
        signalDim: "rgba(255,158,64,.14)",
        pass: "#5FA8C7",
        warn: "#D9A24A",
        fail: "#D85C46",
        text: "#FBFBFB",
        textMuted: "#B8BCC4",
        textFaint: "#69707A",
      },
      fontFamily: {
        // Self-hosted stacks (no build-time Google Fonts fetch). Brand faces are
        // listed first and used when installed locally; otherwise fall back to
        // system fonts so the build works fully offline.
        sans: [
          "Bricolage Grotesque",
          "-apple-system",
          "PingFang SC",
          "Segoe UI",
          "Roboto",
          "Helvetica",
          "Arial",
          "sans-serif",
        ],
        mono: [
          "Space Mono",
          "ui-monospace",
          "SFMono-Regular",
          "Menlo",
          "Monaco",
          "Cascadia Mono",
          "monospace",
        ],
      },
    },
  },
  plugins: [],
};

export default config;
