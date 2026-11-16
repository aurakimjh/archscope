import ja from "./ja.js";
import zhCN from "./zh-CN.js";
import zhTW from "./zh-TW.js";
import type { Locale } from "./locales";

// Source prose is the lookup key for legacy inline labels and help text, too.
// Imported evidence, engine diagnostics, identifiers and source values are never translated.
export const translationCatalogs: Record<Exclude<Locale, "en" | "ko">, Record<string, string>> = {
  ja, "zh-CN": zhCN, "zh-TW": zhTW,
};
export function translateText(locale: Locale, en: string, ko = en): string {
  if (locale === "en") return en;
  if (locale === "ko") return ko;
  return translationCatalogs[locale][en] ?? en;
}
export function translateMessages<T extends Record<string, string>>(locale: Locale, source: T): Record<keyof T, string> {
  return Object.fromEntries(Object.entries(source).map(([key, text]) => [key, translateText(locale, text)])) as Record<keyof T, string>;
}
