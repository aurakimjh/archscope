// ─────────────────────────────────────────────────────────────────────
// [한글] i18n/I18nProvider.tsx — 영어/한국어/일본어/중국어 메시지 사전을
//   제공하는 Context Provider 와 useI18n() 훅.
//
// 책임/목적:
//   - locale state 를 localStorage("archscope.locale") 에 영속화.
//   - 초기값은 navigator.language 와 저장 값 모두를 보고 결정(detectInitialLocale).
//   - t(MessageKey) 는 단순 lookup 함수 — 모든 컴포넌트가 useI18n() 으로 사용.
//
// 데이터 흐름:
//   main.tsx 가 ThemeProvider 안쪽에 I18nProvider 를 마운트.
//   페이지 → useI18n().t("xxx") → messages[locale][key].
//
// 메시지 정의:
//   - messages.ts 에 모든 키와 4개 로케일 번역이 한 곳에 모여 있음.
//   - 새 키 추가 시 모든 로케일에 빠짐없이 채워야 TypeScript 가 강제.
// ─────────────────────────────────────────────────────────────────────
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
