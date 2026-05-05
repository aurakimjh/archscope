import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

import { messages, type Locale, type MessageKey } from "./messages";

const STORAGE_KEY = "archscope.profiler.locale";

type I18nContextValue = {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: MessageKey) => string;
};

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocale] = useState<Locale>(detectInitialLocale);

  useEffect(() => {
    try {
      window.localStorage.setItem(STORAGE_KEY, locale);
    } catch {
      // localStorage may be unavailable in some webviews — silently ignore.
    }
  }, [locale]);

  const value = useMemo<I18nContextValue>(
    () => ({
      locale,
      setLocale,
      t: (key) => messages[locale][key],
    }),
    [locale],
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nContextValue {
  const value = useContext(I18nContext);
  if (!value) {
    throw new Error("useI18n must be used within I18nProvider.");
  }
  return value;
}

function detectInitialLocale(): Locale {
  try {
    const saved = window.localStorage.getItem(STORAGE_KEY);
    if (saved === "ko" || saved === "en") return saved;
  } catch {
    // ignore
  }
  return navigator.language?.toLowerCase().startsWith("ko") ? "ko" : "en";
}

export type { Locale, MessageKey };
