import type { Metadata } from "next";
import { IBM_Plex_Mono, Noto_Sans_SC } from "next/font/google";

import "./globals.css";
import { QueryProvider } from "@/components/providers/query-provider";
import { LanguageProvider } from "@/lib/i18n";

const sans = Noto_Sans_SC({
  variable: "--font-nexaflow-sans",
  weight: "variable",
  subsets: ["latin"],
});

const mono = IBM_Plex_Mono({
  variable: "--font-nexaflow-mono",
  weight: ["500", "600"],
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "NexaFlow — Open Source AI Business OS",
  description:
    "开源 AI 企业业务操作系统 / Open Source AI Business Operating System",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="zh-CN"
      className={`${sans.variable} ${mono.variable} h-full antialiased`}
    >
      <body className="min-h-full">
        <LanguageProvider><QueryProvider>{children}</QueryProvider></LanguageProvider>
      </body>
    </html>
  );
}
