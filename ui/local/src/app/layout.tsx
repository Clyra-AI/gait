import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Gait Local UI",
  description: "Local first-run control room for runpack, regress, and policy checks.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
