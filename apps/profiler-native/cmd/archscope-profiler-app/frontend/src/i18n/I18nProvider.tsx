// ─────────────────────────────────────────────────────────────────────
// [한글] i18n/I18nProvider.tsx — React Context 기반 i18n 제공자.
//
// 책임/목적:
//   - 현재 로케일(en/ko) 상태를 보관, t(key) 함수로 messages 사전에서
//     번역 문자열을 조회.
//   - 초기 로케일은 localStorage 우선, 없으면 navigator.language 가
//     "ko" 로 시작하는지로 결정.
//   - locale 변경 시 localStorage 에 저장 (브라우저 webview 가 storage
//     를 지원하지 않을 수 있어 try/catch 로 감쌈).
//
// 의존성 주의: useI18n 훅은 Provider 외부에서 호출 시 throw — 모든
// 페이지/컴포넌트는 main.tsx 의 트리 안쪽에서만 사용 가능.
// ─────────────────────────────────────────────────────────────────────
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
