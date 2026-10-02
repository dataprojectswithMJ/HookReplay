import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "HookReplay — test webhooks without the chaos",
  description:
    "Fire provider-accurate, correctly-signed webhooks at localhost and staging.",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
