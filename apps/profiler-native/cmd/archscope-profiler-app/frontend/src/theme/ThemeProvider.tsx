// ─────────────────────────────────────────────────────────────────────
// [한글] theme/ThemeProvider.tsx — 라이트/다크/시스템 테마 Context.
//
// 책임/목적:
//   - theme(설정값) 와 resolvedTheme(실제 적용된 light|dark) 분리:
//     theme="system" 일 때 prefers-color-scheme 미디어쿼리를 listen 해
//     resolvedTheme 을 동적으로 바꿉니다.
//   - resolved 변경 시 documentElement 에 .dark 클래스와 data-theme
//     속성을 토글 → Tailwind dark variant 와 CSS variable 토큰 분기.
//
// 의존성 주의: localStorage 키는 'archscope.profiler.theme' 로
// 네임스페이스되어 web 셸과 충돌하지 않습니다 (두 앱이 같은 머신에서
// 실행되어도 독립 설정 유지).
// ─────────────────────────────────────────────────────────────────────
// Theme provider mirrored from apps/frontend/src/components/theme-provider.tsx.
// Storage key is namespaced to the native app so toggling here does not
// stomp the web app's preference if both run on the same machine.

import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

export type Theme = "light" | "dark" | "system";

type ThemeContextValue = {
  theme: Theme;
  resolvedTheme: "light" | "dark";
  setTheme: (theme: Theme) => void;
};

const ThemeContext = createContext<ThemeContextValue | null>(null);

const STORAGE_KEY = "archscope.profiler.theme";

function readInitialTheme(): Theme {
  if (typeof window === "undefined") return "system";
  try {
    const saved = window.localStorage.getItem(STORAGE_KEY);
    if (saved === "light" || saved === "dark" || saved === "system") return saved;
  } catch {
    // ignore storage failures
  }
  return "system";
}

function resolveTheme(theme: Theme): "light" | "dark" {
  if (theme !== "system") return theme;
  if (typeof window === "undefined") return "dark";
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(readInitialTheme);
  const [resolved, setResolved] = useState<"light" | "dark">(() => resolveTheme(theme));

  useEffect(() => {
    const root = document.documentElement;
    root.classList.toggle("dark", resolved === "dark");
    root.dataset.theme = resolved;
  }, [resolved]);

  useEffect(() => {
    setResolved(resolveTheme(theme));
    if (theme !== "system") return;
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const handler = () => setResolved(mq.matches ? "dark" : "light");
    mq.addEventListener("change", handler);
    return () => mq.removeEventListener("change", handler);
  }, [theme]);

  const value = useMemo<ThemeContextValue>(
    () => ({
      theme,
      resolvedTheme: resolved,
      setTheme: (next) => {
        try {
          window.localStorage.setItem(STORAGE_KEY, next);
        } catch {
          // ignore
        }
        setThemeState(next);
      },
    }),
    [theme, resolved],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error("useTheme must be used within ThemeProvider");
  return ctx;
}
