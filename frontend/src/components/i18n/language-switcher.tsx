"use client";

import { Languages } from "lucide-react";
import { LANGUAGE_OPTIONS, useLanguage } from "@/lib/i18n";

export function LanguageSwitcher({ inverse = false }: { inverse?: boolean }) {
  const { language, setLanguage } = useLanguage();
  return (
    <label className={`language-switcher ${inverse ? "language-switcher-inverse" : ""}`}>
      <Languages className="size-3.5" aria-hidden="true" />
      <span className="sr-only">Language / 言語 / 语言</span>
      <select aria-label="Language / 言語 / 语言" value={language} onChange={(event) => setLanguage(event.target.value as typeof language)}>
        {LANGUAGE_OPTIONS.map((option) => <option value={option.value} key={option.value}>{option.label}</option>)}
      </select>
    </label>
  );
}
