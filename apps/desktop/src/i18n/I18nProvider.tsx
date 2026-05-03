import {
  createContext,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";

import {
  localeLabels,
  locales,
  messages,
  type Locale,
  type MessageKey,
} from "./messages";

type I18nContextValue = {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: (key: MessageKey) => string;
};

const I18nContext = createContext<I18nContextValue | null>(null);

type I18nProviderProps = {
  children: ReactNode;
};

export function I18nProvider({ children }: I18nProviderProps): JSX.Element {
  const [locale, setLocale] = useState<Locale>(detectInitialLocale);

  const value = useMemo<I18nContextValue>(
    () => ({
      locale,
      setLocale: (nextLocale) => {
        window.localStorage.setItem("archscope.locale", nextLocale);
        setLocale(nextLocale);
      },
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

export { localeLabels, locales };
export type { Locale, MessageKey };

function detectInitialLocale(): Locale {
  const savedLocale = window.localStorage.getItem("archscope.locale");
  if (isLocale(savedLocale)) {
    return savedLocale;
  }

  const browserLanguage = window.navigator.language.toLowerCase();
  return browserLanguage.startsWith("ko") ? "ko" : "en";
}

function isLocale(value: string | null): value is Locale {
  return value === "en" || value === "ko";
}
