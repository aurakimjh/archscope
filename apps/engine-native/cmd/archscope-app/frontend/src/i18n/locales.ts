export const locales = ["en", "ko", "ja", "zh-CN", "zh-TW"] as const;
export type Locale = (typeof locales)[number];
export const localeLabels: Record<Locale, string> = {
  en: "English", ko: "한국어", ja: "日本語", "zh-CN": "简体中文", "zh-TW": "繁體中文（台灣）",
};
export const LOCALE_STORAGE_KEY = "archscope.profiler.locale";
// Bundled manuals currently have English and Korean editions.
export function documentationLocale(locale: Locale): "en" | "ko" { return locale === "ko" ? "ko" : "en"; }
export function isLocale(value: unknown): value is Locale {
  return typeof value === "string" && (locales as readonly string[]).includes(value);
}
export function resolveLocale(tag: string): Locale | null {
  const parts = tag.toLowerCase().replace(/_/g, "-").split("-");
  if (["en", "ko", "ja"].includes(parts[0])) return parts[0] as Locale;
  if (parts[0] !== "zh") return null;
  if (parts.includes("hant")) return "zh-TW";
  if (parts.includes("hans")) return "zh-CN";
  return parts.some(part => ["tw", "hk", "mo"].includes(part)) ? "zh-TW" : "zh-CN";
}
export function preferredLocale(saved: string | null, languages: readonly string[]): Locale {
  if (saved && resolveLocale(saved)) return resolveLocale(saved)!;
  for (const language of languages) {
    const locale = resolveLocale(language);
    if (locale) return locale;
  }
  return "en";
}
