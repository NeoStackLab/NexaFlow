"use client";

import { Languages } from "lucide-react";
import { useLanguage } from "@/lib/i18n";

export function LanguageSwitcher({ inverse = false }: { inverse?: boolean }) {
  const { language, setLanguage } = useLanguage();
  return (
    <label className={`language-switcher ${inverse ? "language-switcher-inverse" : ""}`}>
      <Languages className="size-3.5" aria-hidden="true" />
      <span className="sr-only">Language / 语言</span>
      <select aria-label="Language / 语言" value={language} onChange={(event) => setLanguage(event.target.value as "zh-CN" | "en")}>
        <option value="zh-CN">简体中文</option>
        <option value="en">English</option>
      </select>
    </label>
  );
}
